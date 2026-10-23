package postgres

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/argon2"
	"velotrust/hub/internal/auth"
	"velotrust/hub/internal/model"
	"velotrust/shared/repository/database"
)

const (
	clientPasswordHashVersion int16 = 1
	clientPasswordMemory            = 64 * 1024
	clientPasswordIterations        = 3
	clientPasswordParallelism       = 2
	clientPasswordKeyLength         = 32
	clientLoginWindow               = 15 * time.Minute
	clientLoginBlock                = 15 * time.Minute
	clientLoginMaxFailures          = 5
)

var clientPasswordHashSlots = make(chan struct{}, 4)

// ClientDirectory authorizes only an active node with an active linked device,
// active owner account, and current device key credential.
type ClientDirectory struct{ db *database.Database }

func NewClientDirectory(db *database.Database) (*ClientDirectory, error) {
	if db == nil || db.Engine() == nil || db.Driver() != "postgres" {
		return nil, errors.New("Client directory requires a PostgreSQL database")
	}
	return &ClientDirectory{db: db}, nil
}

func (r *ClientDirectory) LookupClientByPublicKey(ctx context.Context, claimed []byte) (auth.ClientIdentity, error) {
	if ctx == nil || len(claimed) != 32 {
		return auth.ClientIdentity{}, errors.New("Client identity context and 32-byte public key are required")
	}
	var row struct {
		NodeID      string `xorm:"'node_code'"`
		DeviceID    int64  `xorm:"'device_id'"`
		NodeStatus  string `xorm:"'node_status'"`
		PublicKey   string `xorm:"'public_key'"`
		DeviceState string `xorm:"'device_status'"`
		OwnerID     int64  `xorm:"'owner_user_id'"`
		OwnerState  string `xorm:"'user_status'"`
		Credential  bool   `xorm:"'credential_active'"`
	}
	s := r.db.Engine().NewSession().Context(ctx)
	defer s.Close()
	found, err := s.SQL(`
		SELECT node.node_code, node.node_status, node.public_key, device.id AS device_id, device.owner_user_id,
		       device.status AS device_status, owner.user_status,
		       EXISTS (
		           SELECT 1 FROM public.t_ztna_device_credential credential
		           WHERE credential.device_id=device.id
		             AND credential.credential_type='x25519'
		             AND credential.public_key=node.public_key
		             AND credential.status='active'
		             AND credential.not_before<=CURRENT_TIMESTAMP
		             AND (credential.expires_at IS NULL OR credential.expires_at>CURRENT_TIMESTAMP)
		       ) AS credential_active
		FROM public.t_ztna_node node
		JOIN public.t_ztna_device device ON device.node_id=node.id
		JOIN public.t_iam_user owner ON owner.id=device.owner_user_id
		WHERE node.node_type='client_agent' AND node.public_key=?
	`, base64.StdEncoding.EncodeToString(claimed)).Get(&row)
	if err != nil {
		return auth.ClientIdentity{}, fmt.Errorf("query Client identity directory: %w", err)
	}
	if !found {
		return auth.ClientIdentity{}, auth.ErrClientNotFound
	}
	if row.NodeStatus == "pending" {
		if row.PublicKey == "" {
			return auth.ClientIdentity{}, auth.ErrClientNotRegistered
		}
		return auth.ClientIdentity{NodeID: row.NodeID, NodeStatus: "pending"}, nil
	}
	if row.NodeStatus != "active" || row.DeviceState != "active" || row.OwnerState != "active" || !row.Credential {
		return auth.ClientIdentity{}, auth.ErrClientNotAllowed
	}
	key, err := base64.StdEncoding.DecodeString(row.PublicKey)
	if err != nil || len(key) != 32 || subtle.ConstantTimeCompare(key, claimed) != 1 {
		return auth.ClientIdentity{}, auth.ErrClientNotAllowed
	}
	return auth.ClientIdentity{NodeID: row.NodeID, NodeStatus: row.NodeStatus, PublicKey: key, OwnerID: row.OwnerID, DeviceID: row.DeviceID}, nil
}

// ClientRegistrationRepository authenticates employees and binds newly
// registered Client device keys to the owning employee account.
type ClientRegistrationRepository struct {
	db            *database.Database
	loginMu       sync.Mutex
	loginAttempts map[string]clientLoginAttempt
}

type clientLoginAttempt struct {
	windowStarted time.Time
	failureCount  int
	blockedUntil  time.Time
}

func NewClientRegistrationRepository(db *database.Database, instanceID string) (*ClientRegistrationRepository, error) {
	if db == nil || db.Engine() == nil || db.Driver() != "postgres" || strings.TrimSpace(instanceID) == "" {
		return nil, errors.New("Client registration requires PostgreSQL and a HUB instance ID")
	}
	return &ClientRegistrationRepository{db: db, loginAttempts: make(map[string]clientLoginAttempt)}, nil
}

// LoginClient verifies an active employee account for each Client control
// connection. If the X25519 key is new, the same transaction creates the node,
// device, and key credential so later connections can prove device possession.
func (r *ClientRegistrationRepository) LoginClient(ctx context.Context, username string, password, publicKey []byte, deviceName, platform, sourceIP, agentVersion string, loggedInAt time.Time) (auth.ClientIdentity, bool, error) {
	username = strings.ToLower(strings.TrimSpace(username))
	deviceName = strings.TrimSpace(deviceName)
	platform = strings.TrimSpace(platform)
	agentVersion = strings.TrimSpace(agentVersion)
	if ctx == nil || username == "" || len(username) > 64 || len(password) < 8 || len(password) > 1024 || len(publicKey) != 32 ||
		deviceName == "" || len(deviceName) > 128 || strings.ContainsAny(deviceName, "\r\n\x00") ||
		platform == "" || len(platform) > 64 || strings.ContainsAny(platform, "\r\n\x00") ||
		len(agentVersion) > 64 || strings.ContainsAny(agentVersion, "\r\n\x00") || loggedInAt.IsZero() {
		return auth.ClientIdentity{}, false, auth.ErrClientLoginFailed
	}
	if r.isClientLoginBlocked(username, sourceIP) {
		clear(password)
		return auth.ClientIdentity{}, false, auth.ErrClientLoginFailed
	}
	loggedInAt = loggedInAt.UTC()

	var account struct {
		Username            string `xorm:"'username'"`
		ID                  int64  `xorm:"'id'"`
		AccountType         string `xorm:"'account_type'"`
		UserStatus          string `xorm:"'user_status'"`
		PasswordSalt        string `xorm:"'password_salt'"`
		PasswordHash        string `xorm:"'password_hash'"`
		PasswordHashVersion int16  `xorm:"'password_hash_version'"`
	}
	s := r.db.Engine().NewSession().Context(ctx)
	found, err := s.SQL(`SELECT username, id, account_type, user_status, password_salt, password_hash, password_hash_version
		FROM public.t_iam_user WHERE username=?`, username).Get(&account)
	closeErr := s.Close()
	if err != nil {
		return auth.ClientIdentity{}, false, fmt.Errorf("load HUB employee login account: %w", err)
	}
	if closeErr != nil {
		return auth.ClientIdentity{}, false, fmt.Errorf("close HUB employee login query: %w", closeErr)
	}
	validPassword := verifyEmployeePassword(ctx, account.PasswordSalt, account.PasswordHash, account.PasswordHashVersion, password)
	clear(password)
	if !found || account.ID <= 0 || account.AccountType != "employee" || account.UserStatus != "active" || !validPassword {
		r.recordClientLoginFailure(username, sourceIP)
		return auth.ClientIdentity{}, false, auth.ErrClientLoginFailed
	}
	r.clearClientLoginFailures(username, sourceIP)

	var source *string
	if sourceIP != "" {
		ip := net.ParseIP(sourceIP)
		if ip == nil {
			return auth.ClientIdentity{}, false, auth.ErrClientLoginFailed
		}
		canonical := ip.String()
		source = &canonical
	}
	encodedKey := base64.StdEncoding.EncodeToString(publicKey)
	var identity auth.ClientIdentity
	registered := false
	err = r.db.WithSaveTransaction(ctx, func(tx *database.DBTransaction) error {
		session := tx.Session
		var current struct {
			Username            string `xorm:"'username'"`
			ID                  int64  `xorm:"'id'"`
			AccountType         string `xorm:"'account_type'"`
			UserStatus          string `xorm:"'user_status'"`
			PasswordSalt        string `xorm:"'password_salt'"`
			PasswordHash        string `xorm:"'password_hash'"`
			PasswordHashVersion int16  `xorm:"'password_hash_version'"`
		}
		found, err := session.SQL(`SELECT username, id, account_type, user_status, password_salt, password_hash, password_hash_version
			FROM public.t_iam_user WHERE id=? FOR SHARE`, account.ID).Get(&current)
		if err != nil {
			return fmt.Errorf("lock HUB employee login account: %w", err)
		}
		if !found || current.ID != account.ID || current.AccountType != "employee" || current.UserStatus != "active" ||
			current.PasswordSalt != account.PasswordSalt || current.PasswordHash != account.PasswordHash || current.PasswordHashVersion != account.PasswordHashVersion {
			return auth.ErrClientLoginFailed
		}
		auditUsername := strings.TrimSpace(current.Username)
		if auditUsername == "" || len(auditUsername) > 64 {
			return errors.New("employee username is invalid for Client registration audit")
		}

		var existing struct {
			NodeID      string `xorm:"'node_code'"`
			NodeRowID   int64  `xorm:"'node_id'"`
			NodeName    string `xorm:"'node_name'"`
			NodeStatus  string `xorm:"'node_status'"`
			PublicKey   string `xorm:"'public_key'"`
			DeviceID    int64  `xorm:"'device_id'"`
			DeviceUser  int64  `xorm:"'owner_user_id'"`
			DeviceState string `xorm:"'device_status'"`
		}
		found, err = session.SQL(`SELECT node.id AS node_id, node.node_name, node.node_code, node.node_status, node.public_key,
			device.id AS device_id, device.owner_user_id, device.status AS device_status
			FROM public.t_ztna_node node
			JOIN public.t_ztna_device device ON device.node_id=node.id
			WHERE node.node_type='client_agent' AND node.public_key=? FOR UPDATE OF node, device`, encodedKey).Get(&existing)
		if err != nil {
			return fmt.Errorf("check HUB employee device identity: %w", err)
		}
		if found {
			if existing.DeviceUser != account.ID {
				return auth.ErrClientLoginFailed
			}
			if existing.NodeStatus == "pending" || existing.DeviceState == "pending" {
				return auth.ErrClientApprovalPending
			}
			if existing.NodeStatus != "active" || existing.DeviceState != "active" || existing.PublicKey != encodedKey {
				return auth.ErrClientNotAllowed
			}
			if err := repairHUBClientCredentialClockSkew(tx, existing.DeviceID, encodedKey, auditUsername); err != nil {
				return fmt.Errorf("repair HUB Client credential activation time: %w", err)
			}
			if err := ensureClientAddressLease(tx, account.ID, existing.NodeRowID, existing.DeviceID,
				existing.NodeID, existing.NodeName, encodedKey, auditUsername); err != nil {
				return fmt.Errorf("reconcile Client Overlay address: %w", err)
			}
			identity = auth.ClientIdentity{NodeID: existing.NodeID, NodeStatus: existing.NodeStatus, PublicKey: append([]byte(nil), publicKey...), OwnerID: account.ID, DeviceID: existing.DeviceID}
			return nil
		}

		var duplicate struct {
			ID int64 `xorm:"'id'"`
		}
		duplicateFound, err := session.SQL(`SELECT id FROM public.t_ztna_node WHERE public_key=? LIMIT 1`, encodedKey).Get(&duplicate)
		if err != nil {
			return fmt.Errorf("check HUB device public-key uniqueness: %w", err)
		}
		if duplicateFound {
			return auth.ErrClientNotAllowed
		}

		nodeCodeSuffix, err := randomHex(12)
		if err != nil {
			return fmt.Errorf("generate Client node code: %w", err)
		}
		deviceUUID, err := randomHex(16)
		if err != nil {
			return fmt.Errorf("generate Client device ID: %w", err)
		}
		node := model.Node{
			NodeName: deviceName, NodeCode: "cl_" + nodeCodeSuffix, NodeType: "client_agent", NodeStatus: "active",
			HealthStatus: "unknown", CapabilitiesJSON: []byte(`{}`), PublicKey: &encodedKey,
			RegisteredAt: &loggedInAt,
		}
		if agentVersion != "" {
			node.AgentVersion = &agentVersion
		}
		node.CreateNew(auditUsername)
		if _, err := tx.Save(&node); err != nil {
			return fmt.Errorf("save HUB Client node registration: %w", err)
		}
		ownerID := account.ID
		device := model.Device{
			DeviceName: deviceName, DeviceUUID: deviceUUID, DeviceType: "computer", Platform: platform,
			NodeID: &node.ID, OwnerID: &ownerID, Status: "active", TrustStatus: "unknown",
			RegisteredAt: &loggedInAt, LastSeenAt: &loggedInAt, LastSeenIP: source,
		}
		if agentVersion != "" {
			device.AgentVersion = &agentVersion
		}
		device.CreateNew(auditUsername)
		if _, err := tx.Save(&device); err != nil {
			return fmt.Errorf("save HUB Client device registration: %w", err)
		}
		var databaseClock struct {
			CurrentTime time.Time `xorm:"'current_time'"`
		}
		found, err = session.SQL(`SELECT CURRENT_TIMESTAMP AS current_time`).Get(&databaseClock)
		if err != nil {
			return fmt.Errorf("load database time for HUB Client credential: %w", err)
		}
		if !found || databaseClock.CurrentTime.IsZero() {
			return errors.New("database time for HUB Client credential is unavailable")
		}
		fingerprint := sha256.Sum256(publicKey)
		credential := model.DeviceCredential{
			DeviceID: device.ID, CredentialType: "x25519", PublicKey: encodedKey,
			Fingerprint: hex.EncodeToString(fingerprint[:]), KeyVersion: 1, Status: "active", NotBefore: databaseClock.CurrentTime.UTC(),
		}
		credential.CreateNew(auditUsername)
		if _, err := tx.Save(&credential); err != nil {
			return fmt.Errorf("save HUB Client device credential: %w", err)
		}
		if err := ensureClientAddressLease(tx, account.ID, node.ID, device.ID,
			node.NodeCode, node.NodeName, encodedKey, auditUsername); err != nil {
			return fmt.Errorf("assign Client Overlay address: %w", err)
		}
		registered = true
		identity = auth.ClientIdentity{NodeID: node.NodeCode, NodeStatus: node.NodeStatus, PublicKey: append([]byte(nil), publicKey...), OwnerID: account.ID, DeviceID: device.ID}
		return nil
	})
	if err != nil {
		return auth.ClientIdentity{}, false, err
	}
	return identity, registered, nil
}

// repairHUBClientCredentialClockSkew corrects a future activation time on a
// HUB-created credential after the employee proves account ownership again.
// Management-created credentials keep their explicitly managed validity window.
func repairHUBClientCredentialClockSkew(tx *database.DBTransaction, deviceID int64, publicKey, actor string) error {
	var credential model.DeviceCredential
	found, err := tx.SQL(`
		SELECT id, device_id, credential_type, public_key, fingerprint, key_version, status, not_before,
		       created_at, created_by, updated_at, updated_by, version_no
		FROM public.t_ztna_device_credential
		WHERE device_id=? AND credential_type='x25519' AND public_key=? AND status='active'
		  AND created_by LIKE 'hub:%' AND not_before>CURRENT_TIMESTAMP
		ORDER BY key_version DESC LIMIT 1 FOR UPDATE
	`, deviceID, publicKey).Get(&credential)
	if err != nil {
		return err
	}
	if !found {
		return nil
	}
	var databaseClock struct {
		CurrentTime time.Time `xorm:"'current_time'"`
	}
	found, err = tx.SQL(`SELECT CURRENT_TIMESTAMP AS current_time`).Get(&databaseClock)
	if err != nil {
		return err
	}
	if !found || databaseClock.CurrentTime.IsZero() {
		return errors.New("database time for HUB Client credential repair is unavailable")
	}
	credential.NotBefore = databaseClock.CurrentTime.UTC()
	credential.Update(actor)
	credential.UpdatedAt = databaseClock.CurrentTime.UTC()
	if _, err := tx.Save(&credential); err != nil {
		return err
	}
	return nil
}

func verifyEmployeePassword(ctx context.Context, saltText, hashText string, version int16, password []byte) bool {
	versionSupported := version == clientPasswordHashVersion
	if !versionSupported {
		saltText, hashText = "AAAAAAAAAAAAAAAAAAAAAA", "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
	}
	salt, err := base64.RawStdEncoding.DecodeString(saltText)
	if err != nil || len(salt) < 16 {
		salt, _ = base64.RawStdEncoding.DecodeString("AAAAAAAAAAAAAAAAAAAAAA")
	}
	expected, err := base64.RawStdEncoding.DecodeString(hashText)
	if err != nil || len(expected) != clientPasswordKeyLength {
		expected, _ = base64.RawStdEncoding.DecodeString("AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA")
	}
	select {
	case clientPasswordHashSlots <- struct{}{}:
	case <-ctx.Done():
		return false
	}
	actual := argon2.IDKey(password, salt, clientPasswordIterations, clientPasswordMemory, clientPasswordParallelism, clientPasswordKeyLength)
	<-clientPasswordHashSlots
	valid := versionSupported && len(expected) == clientPasswordKeyLength && hmac.Equal(actual, expected)
	clear(actual)
	return valid
}

func (r *ClientRegistrationRepository) isClientLoginBlocked(username, sourceIP string) bool {
	key := clientLoginBucket(username, sourceIP)
	now := time.Now()
	r.loginMu.Lock()
	defer r.loginMu.Unlock()
	attempt, ok := r.loginAttempts[key]
	if !ok {
		return false
	}
	if now.Before(attempt.blockedUntil) {
		return true
	}
	if now.Sub(attempt.windowStarted) >= clientLoginWindow || !attempt.blockedUntil.IsZero() {
		delete(r.loginAttempts, key)
	}
	return false
}

func (r *ClientRegistrationRepository) recordClientLoginFailure(username, sourceIP string) {
	key := clientLoginBucket(username, sourceIP)
	now := time.Now()
	r.loginMu.Lock()
	defer r.loginMu.Unlock()
	if len(r.loginAttempts) > 8192 {
		for bucket, attempt := range r.loginAttempts {
			if now.Sub(attempt.windowStarted) >= clientLoginWindow && !now.Before(attempt.blockedUntil) {
				delete(r.loginAttempts, bucket)
			}
		}
	}
	attempt := r.loginAttempts[key]
	if attempt.windowStarted.IsZero() || now.Sub(attempt.windowStarted) >= clientLoginWindow {
		attempt = clientLoginAttempt{windowStarted: now}
	}
	attempt.failureCount++
	if attempt.failureCount >= clientLoginMaxFailures {
		attempt.blockedUntil = now.Add(clientLoginBlock)
	}
	r.loginAttempts[key] = attempt
}

func (r *ClientRegistrationRepository) clearClientLoginFailures(username, sourceIP string) {
	r.loginMu.Lock()
	delete(r.loginAttempts, clientLoginBucket(username, sourceIP))
	r.loginMu.Unlock()
}

func clientLoginBucket(username, sourceIP string) string {
	digest := sha256.Sum256([]byte(strings.ToLower(strings.TrimSpace(username)) + "\x00" + sourceIP))
	return hex.EncodeToString(digest[:])
}

func randomHex(bytesCount int) (string, error) {
	value := make([]byte, bytesCount)
	if _, err := rand.Read(value); err != nil {
		return "", err
	}
	return hex.EncodeToString(value), nil
}

var _ auth.ClientDirectory = (*ClientDirectory)(nil)
