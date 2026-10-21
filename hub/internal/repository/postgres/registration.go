package postgres

import (
	"context"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"strings"
	"time"

	"velotrust/hub/internal/auth"
	"velotrust/hub/internal/model"
	"velotrust/shared/repository/database"
)

// GatewayRegistrationRepository checks and atomically consumes first-time
// Gateway credentials while binding the proved X25519 public key.
type GatewayRegistrationRepository struct {
	db    *database.Database
	actor string
}

func NewGatewayRegistrationRepository(db *database.Database, instanceID string) (*GatewayRegistrationRepository, error) {
	if db == nil || db.Engine() == nil || db.Driver() != "postgres" {
		return nil, errors.New("Gateway registration repository requires PostgreSQL")
	}
	if strings.TrimSpace(instanceID) == "" {
		return nil, errors.New("HUB instance ID is required")
	}
	return &GatewayRegistrationRepository{db: db, actor: auditActor(instanceID)}, nil
}

// CheckCredential resolves the target Gateway from its unique one-time token.
// RegisterGateway repeats every check while holding row locks.
func (r *GatewayRegistrationRepository) CheckCredential(ctx context.Context, tokenHash string) (auth.GatewayIdentity, error) {
	if ctx == nil || !validTokenHash(tokenHash) {
		return auth.GatewayIdentity{}, auth.ErrRegistrationNotAllowed
	}
	var row struct {
		NodeID     string `xorm:"'node_code'"`
		NodeStatus string `xorm:"'node_status'"`
	}
	s := r.db.Engine().NewSession().Context(ctx)
	defer s.Close()
	found, err := s.SQL(`
		SELECT node.node_code, node.node_status
		FROM public.t_ztna_node AS node
		JOIN public.t_gateway_registration_token AS credential ON credential.node_id = node.id
		WHERE node.node_type = 'gateway'
		  AND node.node_status = 'pending' AND node.public_key IS NULL AND node.registered_at IS NULL
		  AND credential.token_hash = ? AND credential.status = 'pending'
		  AND credential.max_uses = 1 AND credential.used_count = 0
		  AND credential.expires_at > CURRENT_TIMESTAMP
	`, tokenHash).Get(&row)
	if err != nil {
		return auth.GatewayIdentity{}, fmt.Errorf("check Gateway registration credential: %w", err)
	}
	if !found || row.NodeID == "" {
		return auth.GatewayIdentity{}, auth.ErrRegistrationNotAllowed
	}
	return auth.GatewayIdentity{NodeID: row.NodeID, NodeStatus: row.NodeStatus}, nil
}

// RegisterGateway consumes the credential and binds the Gateway public key in
// one transaction. A second concurrent use cannot bind another key.
func (r *GatewayRegistrationRepository) RegisterGateway(ctx context.Context, nodeID, tokenHash string, publicKey []byte, sourceIP string, registeredAt time.Time) (auth.GatewayIdentity, bool, error) {
	if ctx == nil || strings.TrimSpace(nodeID) == "" || !validTokenHash(tokenHash) || len(publicKey) != 32 || registeredAt.IsZero() {
		return auth.GatewayIdentity{}, false, auth.ErrRegistrationNotAllowed
	}
	var usedIP *string
	if sourceIP != "" {
		parsedIP := net.ParseIP(sourceIP)
		if parsedIP == nil {
			return auth.GatewayIdentity{}, false, auth.ErrRegistrationNotAllowed
		}
		canonicalIP := parsedIP.String()
		usedIP = &canonicalIP
	}

	var identity auth.GatewayIdentity
	var requireApproval bool
	err := r.db.WithSaveTransaction(ctx, func(tx *database.DBTransaction) error {
		var node model.Node
		found, err := tx.SQL(`
			SELECT * FROM public.t_ztna_node
			WHERE node_code = ? AND node_type = 'gateway'
			FOR UPDATE
		`, nodeID).Get(&node)
		if err != nil {
			return fmt.Errorf("lock Gateway registration target: %w", err)
		}
		if !found || node.NodeStatus != "pending" || node.PublicKey != nil || node.RegisteredAt != nil {
			return auth.ErrRegistrationNotAllowed
		}

		var credential model.GatewayRegistrationToken
		found, err = tx.SQL(`
			SELECT * FROM public.t_gateway_registration_token
			WHERE node_id = ? AND token_hash = ? AND status = 'pending'
			  AND max_uses = 1 AND used_count = 0 AND expires_at > CURRENT_TIMESTAMP
			FOR UPDATE
		`, node.ID, tokenHash).Get(&credential)
		if err != nil {
			return fmt.Errorf("lock Gateway registration credential: %w", err)
		}
		if !found || credential.Status != "pending" || credential.MaxUses != 1 || credential.UsedCount != 0 {
			return auth.ErrRegistrationNotAllowed
		}

		encodedKey := base64.StdEncoding.EncodeToString(publicKey)
		var existing struct {
			ID int64 `xorm:"'id'"`
		}
		keyFound, err := tx.SQL(`
			SELECT id FROM public.t_ztna_node WHERE public_key = ? AND id <> ? LIMIT 1
		`, encodedKey, node.ID).Get(&existing)
		if err != nil {
			return fmt.Errorf("check Gateway public key uniqueness: %w", err)
		}
		if keyFound {
			return auth.ErrPublicKeyAlreadyBound
		}

		registeredAt = registeredAt.UTC()
		node.PublicKey = &encodedKey
		node.RegisteredAt = &registeredAt
		requireApproval = node.RequireApproval
		if !requireApproval {
			node.NodeStatus = "active"
		}
		node.Update(r.actor)
		node.Cols("public_key", "registered_at", "node_status")
		node.AddSaveCondition("node_type='gateway'")
		node.AddSaveCondition("node_status='pending'")
		node.AddSaveCondition("public_key IS NULL")
		node.AddSaveCondition("registered_at IS NULL")
		affected, err := tx.Save(&node)
		if err != nil {
			return fmt.Errorf("bind Gateway public key: %w", err)
		}
		if affected != 1 {
			return auth.ErrRegistrationNotAllowed
		}

		credential.Status = "used"
		credential.UsedCount = 1
		credential.UsedAt = &registeredAt
		credential.UsedIP = usedIP
		credential.Update(r.actor)
		credential.Cols("status", "used_count", "used_at", "used_ip")
		credential.AddSaveCondition("status='pending'")
		credential.AddSaveCondition("used_count=0")
		credential.AddSaveCondition("expires_at > CURRENT_TIMESTAMP")
		affected, err = tx.Save(&credential)
		if err != nil {
			return fmt.Errorf("consume Gateway registration credential: %w", err)
		}
		if affected != 1 {
			return auth.ErrRegistrationNotAllowed
		}
		status := node.NodeStatus
		identity = auth.GatewayIdentity{NodeID: node.NodeCode, NodeStatus: status, PublicKey: append([]byte(nil), publicKey...)}
		return nil
	})
	if err != nil {
		return auth.GatewayIdentity{}, false, err
	}
	return identity, requireApproval, nil
}

// RegistrationState returns the Management-owned approval state and the
// Gateway key bound by the one-time enrollment transaction.
func (r *GatewayRegistrationRepository) RegistrationState(ctx context.Context, nodeID string) (auth.GatewayIdentity, error) {
	if ctx == nil || strings.TrimSpace(nodeID) == "" {
		return auth.GatewayIdentity{}, auth.ErrGatewayNotFound
	}
	var row struct {
		NodeID       string     `xorm:"'node_code'"`
		NodeStatus   string     `xorm:"'node_status'"`
		PublicKey    string     `xorm:"'public_key'"`
		RegisteredAt *time.Time `xorm:"'registered_at'"`
	}
	s := r.db.Engine().NewSession().Context(ctx)
	defer s.Close()
	found, err := s.SQL(`
		SELECT node_code, node_status, public_key, registered_at
		FROM public.t_ztna_node
		WHERE node_code = ? AND node_type = 'gateway'
	`, nodeID).Get(&row)
	if err != nil {
		return auth.GatewayIdentity{}, fmt.Errorf("load Gateway registration state: %w", err)
	}
	if !found {
		return auth.GatewayIdentity{}, auth.ErrGatewayNotFound
	}
	if row.PublicKey == "" || row.RegisteredAt == nil {
		return auth.GatewayIdentity{}, auth.ErrRegistrationNotAllowed
	}
	publicKey, err := base64.StdEncoding.DecodeString(row.PublicKey)
	if err != nil || len(publicKey) != 32 {
		return auth.GatewayIdentity{}, errors.New("registered Gateway X25519 public key is invalid")
	}
	return auth.GatewayIdentity{NodeID: row.NodeID, NodeStatus: row.NodeStatus, PublicKey: publicKey}, nil
}

func validTokenHash(value string) bool {
	if len(value) != 64 {
		return false
	}
	decoded, err := hex.DecodeString(value)
	return err == nil && len(decoded) == 32 && subtle.ConstantTimeCompare([]byte(hex.EncodeToString(decoded)), []byte(value)) == 1
}

var _ interface {
	CheckCredential(context.Context, string) (auth.GatewayIdentity, error)
	RegisterGateway(context.Context, string, string, []byte, string, time.Time) (auth.GatewayIdentity, bool, error)
	RegistrationState(context.Context, string) (auth.GatewayIdentity, error)
} = (*GatewayRegistrationRepository)(nil)
