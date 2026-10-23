package iam

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/go-webauthn/webauthn/protocol"
	webauthnlib "github.com/go-webauthn/webauthn/webauthn"
	"golang.org/x/crypto/argon2"
	servicecontext "velotrust/management/internal/context"
	"velotrust/management/internal/lib/cache"
	model "velotrust/management/internal/model/iam"
	repository "velotrust/management/internal/repository/iam"
	appErrors "velotrust/pkg/errors"
)

const (
	PasswordHashVersion        int16 = 1
	minPasswordLength                = 8
	maxPasswordBytes                 = 1024
	argonMemory                      = 64 * 1024
	argonIterations                  = 3
	argonParallelism                 = 2
	argonKeyLength                   = 32
	passwordSaltLength               = 16
	loginThrottleFailures            = 5
	loginThrottleWindowMinutes       = 15
	loginThrottleBlockMinutes        = 15
	argonConcurrentHashes            = 4
)

var passwordKDFSlots = make(chan struct{}, argonConcurrentHashes)

type Options struct {
	SessionIdleTTL     time.Duration
	SessionAbsoluteTTL time.Duration
	JWTSigningKey      []byte
	ThrottleHMACKey    []byte
	TOTPEncryptionKey  []byte
	TOTPKeyVersion     string
	WebAuthnRPID       string
	WebAuthnName       string
	WebAuthnOrigins    []string
	CeremonyStore      cache.OneTimeStore
}

type Service struct {
	store      repository.Store
	opts       Options
	webAuthn   *webauthnlib.WebAuthn
	ceremonies cache.OneTimeStore
}

type SessionGrant struct {
	AccessToken string    `json:"access_token"`
	TokenType   string    `json:"token_type"`
	ExpiresAt   time.Time `json:"expires_at"`
	User        UserView  `json:"user"`
}

type UserView struct {
	ID                 int64         `json:"id"`
	AccountType        string        `json:"account_type"`
	Username           string        `json:"username"`
	DisplayName        string        `json:"display_name"`
	Email              *string       `json:"email,omitempty"`
	UserStatus         string        `json:"user_status"`
	MustChangePassword bool          `json:"must_change_password"`
	Permissions        []string      `json:"permissions,omitempty"`
	LastLoginAt        *time.Time    `json:"last_login_at,omitempty"`
	CreatedAt          time.Time     `json:"created_at"`
	TOTPEnabled        bool          `json:"totp_enabled,omitempty"`
	Passkeys           []PasskeyView `json:"passkeys,omitempty"`
}

type PasskeyView struct {
	ID         int64      `json:"id"`
	Name       string     `json:"name"`
	CreatedAt  time.Time  `json:"created_at"`
	LastUsedAt *time.Time `json:"last_used_at,omitempty"`
}

type CreateUserInput struct {
	AccountType string
	Username    string
	DisplayName string
	Email       string
	Password    string
}

type CreateRoleInput struct {
	RoleName    string
	RoleCode    string
	Description string
}

type GroupInput struct {
	ID          int64
	VersionNo   int64
	GroupName   string
	GroupCode   string
	Description string
	Status      string
	SortOrder   int
}

type MenuInput struct {
	ID        int64
	ParentID  *int64
	MenuName  string
	MenuCode  string
	Path      string
	Component string
	Icon      string
	SortOrder int
	MenuType  string
	Visible   bool
	Status    string
	VersionNo int64
}

func New(store repository.Store, opts Options) (*Service, error) {
	if store == nil {
		return nil, errors.New("IAM store is required")
	}
	if opts.SessionIdleTTL <= 0 {
		opts.SessionIdleTTL = 30 * time.Minute
	}
	if opts.SessionAbsoluteTTL <= 0 {
		opts.SessionAbsoluteTTL = 12 * time.Hour
	}
	if opts.SessionIdleTTL > opts.SessionAbsoluteTTL {
		return nil, errors.New("IAM idle session TTL cannot exceed absolute TTL")
	}
	if len(opts.JWTSigningKey) < 32 || strings.Contains(string(opts.JWTSigningKey), "${") {
		return nil, errors.New("IAM JWT signing key must be configured with at least 32 bytes")
	}
	if len(opts.ThrottleHMACKey) < 32 {
		return nil, errors.New("IAM throttle HMAC key must contain at least 32 bytes")
	}
	opts.JWTSigningKey = append([]byte(nil), opts.JWTSigningKey...)
	opts.ThrottleHMACKey = append([]byte(nil), opts.ThrottleHMACKey...)
	opts.TOTPEncryptionKey = append([]byte(nil), opts.TOTPEncryptionKey...)
	svc := &Service{store: store, opts: opts, ceremonies: opts.CeremonyStore}
	if opts.WebAuthnRPID != "" || len(opts.WebAuthnOrigins) > 0 {
		if strings.TrimSpace(opts.WebAuthnRPID) == "" || len(opts.WebAuthnOrigins) == 0 || opts.CeremonyStore == nil {
			return nil, errors.New("WebAuthn RP ID, origin allowlist, and ceremony store must be configured together")
		}
		name := strings.TrimSpace(opts.WebAuthnName)
		if name == "" {
			name = "VeloTrust"
		}
		instance, err := webauthnlib.New(&webauthnlib.Config{
			RPID: opts.WebAuthnRPID, RPDisplayName: name, RPOrigins: append([]string(nil), opts.WebAuthnOrigins...),
			AuthenticatorSelection: protocol.AuthenticatorSelection{UserVerification: protocol.VerificationRequired},
		})
		if err != nil {
			return nil, fmt.Errorf("configure WebAuthn: %w", err)
		}
		svc.webAuthn = instance
	}
	return svc, nil
}

func (s *Service) Close() error {
	if closer, ok := s.ceremonies.(io.Closer); ok {
		return closer.Close()
	}
	return nil
}

func (s *Service) Login(ctx context.Context, username, password, sourceIP, userAgent, requestID string) (*SessionGrant, error) {
	return s.LoginWithTOTP(ctx, username, password, "", sourceIP, userAgent, requestID)
}

func (s *Service) LoginWithTOTP(ctx context.Context, username, password, totpCode, sourceIP, userAgent, requestID string) (*SessionGrant, error) {
	serviceCtx := servicecontext.FromContext(ctx)
	username = strings.TrimSpace(username)
	if username == "" || password == "" || len(username) > 64 || len(password) > 1024 {
		return nil, invalidLogin()
	}
	usernameHash := s.throttleHash("username", strings.ToLower(username))
	ip := strings.TrimSpace(sourceIP)
	if ip == "" {
		ip = "unknown"
	}
	ipHash := s.throttleHash("source_ip", ip)
	blocked, err := s.store.LoginBlocked(serviceCtx, usernameHash, ipHash)
	if err != nil {
		return nil, fmt.Errorf("check login throttle: %w", err)
	}
	if blocked {
		return nil, invalidLogin()
	}

	user, err := s.store.FindUserByUsername(serviceCtx, username)
	if err != nil {
		return nil, fmt.Errorf("load login identity: %w", err)
	}
	validPassword := user != nil && user.AccountType == model.AccountTypePlatform && verifyPassword(user, password)
	mfaUsed := false
	if user != nil && user.AccountType == model.AccountTypePlatform && user.UserStatus == "active" && validPassword {
		authenticator, authErr := s.store.FindTOTPAuthenticator(serviceCtx, user.ID)
		if authErr != nil {
			return nil, fmt.Errorf("load IAM MFA authenticator: %w", authErr)
		}
		if authenticator != nil && authenticator.Status == "active" {
			secret, decryptErr := decryptTOTPSecret(authenticator.SecretCiphertext, s.opts.TOTPEncryptionKey, user.ID, authenticator.EncryptionKeyVersion)
			if decryptErr != nil {
				return nil, fmt.Errorf("load IAM MFA secret: %w", decryptErr)
			}
			step, ok := matchingTOTPStep(secret, totpCode, time.Now().UTC())
			if !ok || s.store.ConsumeTOTP(serviceCtx, user.ID, step) != nil {
				validPassword = false
			} else {
				mfaUsed = true
			}
		}
	}
	if user == nil || user.AccountType != model.AccountTypePlatform || user.UserStatus != "active" || !validPassword {
		log := s.loginLog(user, "local", "failure", "invalid_credentials", sourceIP, userAgent, requestID, "")
		if err := s.store.RecordLoginFailure(serviceCtx, log, usernameHash, ipHash, loginThrottleFailures, loginThrottleWindowMinutes, loginThrottleBlockMinutes); err != nil {
			return nil, fmt.Errorf("record login failure: %w", err)
		}
		return nil, invalidLogin()
	}

	sessionID, err := randomHex(32)
	if err != nil {
		return nil, fmt.Errorf("create IAM session ID: %w", err)
	}
	now := time.Now().UTC()
	expiresAt := now.Add(s.opts.SessionAbsoluteTTL).Truncate(time.Second)
	rawToken, err := issueAccessToken(s.opts.JWTSigningKey, user.ID, sessionID, now, expiresAt)
	if err != nil {
		return nil, fmt.Errorf("create IAM JWT access token: %w", err)
	}
	tokenHash := hashToken(rawToken)
	userSession := &model.UserSession{
		SessionID: sessionID, UserID: user.ID, TokenHash: tokenHash, AuthLevel: "password",
		IssuedAt: now, ExpiresAt: expiresAt, LastSeenAt: &now, SourceIP: optional(sourceIP),
		UserAgent: optional(userAgent), Status: "active",
	}
	if mfaUsed {
		userSession.AuthLevel = "password_mfa"
		userSession.MFAAuthenticatedAt = &now
	}
	loginLog := s.loginLog(user, "local", "success", "", sourceIP, userAgent, requestID, sessionID)
	loginLog.MFAUsed = mfaUsed
	if err := s.store.CreateLoginSession(serviceCtx, userSession, loginLog, sourceIP, usernameHash, ipHash); err != nil {
		return nil, fmt.Errorf("persist IAM login: %w", err)
	}
	return &SessionGrant{AccessToken: rawToken, TokenType: "Bearer", ExpiresAt: expiresAt, User: view(user)}, nil
}

func (s *Service) ResolveBearer(ctx context.Context, token string) (*model.UserSession, *model.User, error) {
	serviceCtx := servicecontext.FromContext(ctx)
	claims, err := parseAccessToken(s.opts.JWTSigningKey, token)
	if err != nil {
		return nil, nil, fmt.Errorf("validate IAM JWT: %w", err)
	}
	userID, err := parseAccessTokenSubject(claims.Subject)
	if err != nil || claims.ID == "" {
		if err != nil {
			return nil, nil, fmt.Errorf("validate IAM JWT subject: %w", err)
		}
		return nil, nil, errors.New("validate IAM JWT: session ID claim is missing")
	}
	tokenHash := hashToken(token)
	session, user, err := s.store.ResolveSession(serviceCtx, tokenHash)
	if err != nil {
		return nil, nil, fmt.Errorf("resolve IAM bearer session: %w", err)
	}
	now := time.Now().UTC()
	if session == nil {
		return nil, nil, errors.New("IAM JWT has no active backing session")
	}
	if user == nil {
		return nil, nil, errors.New("IAM session references a missing user")
	}
	if user.ID != userID || session.UserID != userID || session.SessionID != claims.ID {
		return nil, nil, errors.New("IAM JWT claims do not match the stored session")
	}
	if user.AccountType != model.AccountTypePlatform || user.UserStatus != "active" {
		return nil, nil, errors.New("IAM session user is not active")
	}
	if session.IssuedAt.After(now) {
		return nil, nil, errors.New("IAM session issue time is in the future")
	}
	if !session.ExpiresAt.After(now) {
		return nil, nil, errors.New("IAM backing session has expired")
	}
	actorContext := servicecontext.New(serviceCtx, servicecontext.Principal{ID: fmt.Sprintf("%d", user.ID), Name: user.Username, EntityID: session.SessionID, Type: "user"}, servicecontext.RequestMeta{})
	lastSeen := session.IssuedAt
	if session.LastSeenAt != nil {
		lastSeen = *session.LastSeenAt
	}
	if now.Sub(lastSeen) > s.opts.SessionIdleTTL {
		_ = s.store.RevokeSession(actorContext.WithoutCancel(), session.SessionID, "idle_timeout")
		return nil, nil, fmt.Errorf("IAM session exceeded idle TTL: %w", appErrors.NewAuthError("IAM_UNAUTHENTICATED", appErrors.AuthTokenExpired))
	}
	if err := s.store.TouchSession(actorContext, session.SessionID, tokenHash, ""); err != nil {
		return nil, nil, fmt.Errorf("update IAM session activity: %w", err)
	}
	return session, user, nil
}

func (s *Service) Logout(ctx context.Context, sessionID string) error {
	if strings.TrimSpace(sessionID) == "" {
		return appErrors.NewAuthError("IAM_UNAUTHENTICATED", appErrors.AuthUnauthenticated)
	}
	serviceCtx := servicecontext.FromContext(ctx)
	return s.store.RevokeSession(serviceCtx, sessionID, "logout")
}

func (s *Service) CurrentUser(ctx *servicecontext.Context) (*UserView, error) {
	if ctx == nil {
		return nil, appErrors.NewAuthError("IAM_UNAUTHENTICATED", appErrors.AuthUnauthenticated)
	}
	userID, err := parseUserID(ctx.UserID())
	if err != nil {
		return nil, appErrors.NewAuthError("IAM_UNAUTHENTICATED", appErrors.AuthUnauthenticated)
	}
	user, err := s.store.FindUserByID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("load current IAM user: %w", err)
	}
	if user == nil || user.AccountType != model.AccountTypePlatform || user.UserStatus != "active" {
		return nil, appErrors.NewAuthError("IAM_UNAUTHENTICATED", appErrors.AuthUnauthenticated)
	}
	permissions, err := s.store.ListPermissions(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("load current IAM permissions: %w", err)
	}
	result := view(user)
	result.Permissions = permissions
	authenticator, err := s.store.FindTOTPAuthenticator(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("load current IAM TOTP status: %w", err)
	}
	result.TOTPEnabled = authenticator != nil && authenticator.Status == "active"
	credentials, err := s.store.ListWebAuthnCredentials(ctx, userID, s.opts.WebAuthnRPID)
	if err != nil {
		return nil, fmt.Errorf("load current IAM Passkeys: %w", err)
	}
	result.Passkeys = make([]PasskeyView, 0, len(credentials))
	for _, credential := range credentials {
		result.Passkeys = append(result.Passkeys, PasskeyView{ID: credential.ID, Name: credential.CredentialName, CreatedAt: credential.CreatedAt, LastUsedAt: credential.LastUsedAt})
	}
	return &result, nil
}

func (s *Service) HasPermission(ctx context.Context, userID int64, permission string) (bool, error) {
	if userID <= 0 || strings.TrimSpace(permission) == "" {
		return false, nil
	}
	serviceCtx := servicecontext.FromContext(ctx)
	return s.store.HasPermission(serviceCtx, userID, permission)
}

func (s *Service) ListUsers(ctx *servicecontext.Context, page, pageSize int, accountType string) ([]UserView, int64, error) {
	if err := s.requirePermission(ctx, "iam.user.read"); err != nil {
		return nil, 0, err
	}
	if accountType != "" && accountType != model.AccountTypePlatform && accountType != model.AccountTypeEmployee {
		return nil, 0, appErrors.NewClientError("IAM_INVALID_ARGUMENT", map[string]string{"account_type": "must_be_platform_or_employee"})
	}
	users, total, err := s.store.ListUsers(ctx, page, pageSize, accountType)
	if err != nil {
		return nil, 0, err
	}
	result := make([]UserView, 0, len(users))
	for _, user := range users {
		result = append(result, view(&user))
	}
	return result, total, nil
}

func (s *Service) ListGroups(ctx *servicecontext.Context) ([]model.Group, error) {
	if err := s.requirePermission(ctx, "iam.group.read"); err != nil {
		return nil, err
	}
	return s.store.ListGroups(ctx)
}

func (s *Service) SaveGroup(ctx *servicecontext.Context, input GroupInput) (*model.Group, error) {
	if err := s.requirePermission(ctx, "iam.group.write"); err != nil {
		return nil, err
	}
	name := strings.TrimSpace(input.GroupName)
	code := strings.ToLower(strings.TrimSpace(input.GroupCode))
	description := strings.TrimSpace(input.Description)
	if name == "" || len(name) > 128 || len(code) == 0 || len(code) > 64 || len(description) > 512 || input.SortOrder < 0 || (input.Status != "active" && input.Status != "disabled") {
		return nil, appErrors.NewClientError("IAM_INVALID_ARGUMENT", map[string]string{"group": "invalid"})
	}
	for index, char := range code {
		valid := char >= 'a' && char <= 'z' || char >= '0' && char <= '9' || char == '_' || char == '-' || char == '.'
		if !valid || index == 0 && !(char >= 'a' && char <= 'z' || char >= '0' && char <= '9') {
			return nil, appErrors.NewClientError("IAM_INVALID_ARGUMENT", map[string]string{"group_code": "must_start_with_letter_or_number_and_use_only_letters_numbers_dot_dash_or_underscore"})
		}
	}
	if input.ID == 0 && input.VersionNo != 0 || input.ID > 0 && input.VersionNo <= 0 {
		return nil, appErrors.NewClientError("IAM_INVALID_ARGUMENT", map[string]string{"version_no": "new_group_omits_version_existing_group_requires_version"})
	}
	group := &model.Group{ID: input.ID, GroupName: name, GroupCode: code, Status: input.Status, SortOrder: input.SortOrder}
	if description != "" {
		group.Description = &description
	}
	actor, actorID := actorFromContext(ctx)
	action := "iam.group.create"
	if group.ID > 0 {
		action = "iam.group.update"
	}
	changes, _ := json.Marshal(map[string]any{"group_name": name, "group_code": code, "status": input.Status})
	audit := s.newAudit(ctx, actorID, actor, action, "group", code, "success", nil)
	audit.ChangesJSON = changes
	if err := s.store.SaveGroup(ctx, group, input.VersionNo, audit); err != nil {
		switch {
		case errors.Is(err, repository.ErrGroupCodeExists):
			return nil, appErrors.NewClientError("IAM_GROUP_CODE_EXISTS", nil)
		case errors.Is(err, repository.ErrGroupNotFound):
			return nil, appErrors.NewClientError("IAM_GROUP_NOT_FOUND", nil)
		case errors.Is(err, repository.ErrGroupVersionConflict):
			return nil, appErrors.NewClientError("IAM_GROUP_CONFLICT", nil)
		default:
			return nil, fmt.Errorf("save IAM group: %w", err)
		}
	}
	return group, nil
}

func (s *Service) ListGroupMembers(ctx *servicecontext.Context, groupID int64) ([]model.GroupMember, error) {
	if err := s.requirePermission(ctx, "iam.group.read"); err != nil {
		return nil, err
	}
	if groupID <= 0 {
		return nil, appErrors.NewClientError("IAM_INVALID_ARGUMENT", map[string]string{"group_id": "required_positive_integer"})
	}
	members, err := s.store.ListGroupMembers(ctx, groupID)
	if errors.Is(err, repository.ErrGroupNotFound) {
		return nil, appErrors.NewClientError("IAM_GROUP_NOT_FOUND", nil)
	}
	return members, err
}

func (s *Service) SetGroupMembers(ctx *servicecontext.Context, groupID int64, userIDs []int64) error {
	if err := s.requirePermission(ctx, "iam.group.write"); err != nil {
		return err
	}
	if groupID <= 0 || len(userIDs) > 5000 {
		return appErrors.NewClientError("IAM_INVALID_ARGUMENT", map[string]string{"group_members": "invalid"})
	}
	unique := make(map[int64]struct{}, len(userIDs))
	for _, userID := range userIDs {
		if userID <= 0 {
			return appErrors.NewClientError("IAM_INVALID_ARGUMENT", map[string]string{"user_ids": "must_be_positive"})
		}
		unique[userID] = struct{}{}
	}
	userIDs = make([]int64, 0, len(unique))
	for userID := range unique {
		userIDs = append(userIDs, userID)
	}
	sort.Slice(userIDs, func(i, j int) bool { return userIDs[i] < userIDs[j] })
	actor, actorID := actorFromContext(ctx)
	audit := s.newAudit(ctx, actorID, actor, "iam.group.members.update", "group_members", fmt.Sprintf("%d", groupID), "success", nil)
	if err := s.store.SetGroupMembers(ctx, groupID, userIDs, audit); err != nil {
		switch {
		case errors.Is(err, repository.ErrGroupNotFound):
			return appErrors.NewClientError("IAM_GROUP_NOT_FOUND", nil)
		case errors.Is(err, repository.ErrGroupMembersInvalid):
			return appErrors.NewClientError("IAM_INVALID_ARGUMENT", map[string]string{"user_ids": "must_reference_employee_users"})
		default:
			return fmt.Errorf("update IAM group membership: %w", err)
		}
	}
	return nil
}

func (s *Service) CreateUser(ctx *servicecontext.Context, input CreateUserInput) (*UserView, error) {
	if err := s.requirePermission(ctx, "iam.user.write"); err != nil {
		return nil, err
	}
	username := strings.ToLower(strings.TrimSpace(input.Username))
	displayName := strings.TrimSpace(input.DisplayName)
	accountType := strings.TrimSpace(input.AccountType)
	if accountType != model.AccountTypePlatform && accountType != model.AccountTypeEmployee {
		return nil, appErrors.NewClientError("IAM_INVALID_ARGUMENT", map[string]string{"account_type": "must_be_platform_or_employee"})
	}
	if username == "" || len(username) > 64 || len(displayName) > 128 || len(strings.TrimSpace(input.Email)) > 128 {
		return nil, appErrors.NewClientError("IAM_INVALID_ARGUMENT", map[string]string{"user": "invalid"})
	}
	if !validPasswordLength(input.Password) {
		return nil, appErrors.NewClientError("IAM_INVALID_ARGUMENT", map[string]string{"password": "must_be_8_chars_max_1024_bytes"})
	}
	if displayName == "" {
		displayName = username
	}
	var salt, hash *string
	var version *int16
	var handle *[]byte
	mustChangePassword := false
	saltValue, hashValue, err := hashPassword(input.Password)
	if err != nil {
		return nil, fmt.Errorf("hash IAM password: %w", err)
	}
	salt, hash, version = &saltValue, &hashValue, int16Ptr(PasswordHashVersion)
	if accountType == model.AccountTypePlatform {
		handleValue := make([]byte, 32)
		if _, err := rand.Read(handleValue); err != nil {
			return nil, fmt.Errorf("generate WebAuthn user handle: %w", err)
		}
		handle = &handleValue
		mustChangePassword = true
	}
	actor, actorID := actorFromContext(ctx)
	user := &model.User{AccountType: accountType, Username: username, DisplayName: displayName, WebAuthnUserHandle: handle, PasswordSalt: salt, PasswordHash: hash, PasswordHashVersion: version, AuthSource: "local", UserStatus: "active", MustChangePassword: mustChangePassword}
	if email := strings.TrimSpace(input.Email); email != "" {
		user.Email = &email
	}
	audit := s.newAudit(ctx, actorID, actor, "iam.user.create", "user", username, "success", nil)
	audit.ChangesJSON = []byte(fmt.Sprintf(`{"account_type":%q}`, accountType))
	if err := s.store.CreateUser(ctx, user, audit); err != nil {
		return nil, fmt.Errorf("create IAM user: %w", err)
	}
	result := view(user)
	return &result, nil
}

func (s *Service) CreateRole(ctx *servicecontext.Context, input CreateRoleInput) (*model.Role, error) {
	if err := s.requirePermission(ctx, "iam.role.write"); err != nil {
		return nil, err
	}
	name, code := strings.TrimSpace(input.RoleName), strings.TrimSpace(input.RoleCode)
	if name == "" || code == "" || len(name) > 64 || len(code) > 64 || len(input.Description) > 512 {
		return nil, appErrors.NewClientError("IAM_INVALID_ARGUMENT", map[string]string{"role": "invalid"})
	}
	actor, actorID := actorFromContext(ctx)
	role := &model.Role{RoleName: name, RoleCode: code, Status: "active"}
	if input.Description != "" {
		role.Description = &input.Description
	}
	audit := s.newAudit(ctx, actorID, actor, "iam.role.create", "role", code, "success", nil)
	if err := s.store.CreateRole(ctx, role, audit); err != nil {
		return nil, fmt.Errorf("create IAM role: %w", err)
	}
	return role, nil
}

func (s *Service) ListRoles(ctx *servicecontext.Context) ([]model.Role, error) {
	if err := s.requirePermission(ctx, "iam.role.read"); err != nil {
		return nil, err
	}
	return s.store.ListRoles(ctx)
}

func (s *Service) ListRoleMenus(ctx *servicecontext.Context, roleID int64) ([]model.Menu, error) {
	if err := s.requirePermission(ctx, "iam.role.read"); err != nil {
		return nil, err
	}
	if roleID <= 0 {
		return nil, appErrors.NewClientError("IAM_INVALID_ARGUMENT", map[string]string{"role_id": "required_positive_integer"})
	}
	menus, err := s.store.ListRoleMenus(ctx, roleID)
	if errors.Is(err, repository.ErrRoleNotFound) {
		return nil, appErrors.NewClientError("IAM_ROLE_NOT_FOUND", nil)
	}
	return menus, err
}

func (s *Service) ListCurrentUserMenus(ctx *servicecontext.Context) ([]model.Menu, error) {
	_, actorID := actorFromContext(ctx)
	if actorID == nil || *actorID <= 0 {
		return nil, appErrors.NewAuthError("IAM_UNAUTHENTICATED", appErrors.AuthUnauthenticated)
	}
	return s.store.ListUserMenus(ctx, *actorID)
}

func (s *Service) SetRoleMenus(ctx *servicecontext.Context, roleID int64, menuIDs []int64) error {
	if roleID <= 0 || len(menuIDs) > 1000 {
		return appErrors.NewClientError("IAM_INVALID_ARGUMENT", map[string]string{"role_menus": "invalid"})
	}
	if err := s.requirePermission(ctx, "iam.role.write"); err != nil {
		return err
	}
	actor, actorID := actorFromContext(ctx)
	if actorID == nil || *actorID <= 0 {
		return appErrors.NewAuthError("IAM_UNAUTHENTICATED", appErrors.AuthUnauthenticated)
	}
	unique := make(map[int64]struct{}, len(menuIDs))
	for _, menuID := range menuIDs {
		if menuID <= 0 {
			return appErrors.NewClientError("IAM_INVALID_ARGUMENT", map[string]string{"menu_ids": "must_be_positive"})
		}
		unique[menuID] = struct{}{}
	}
	menuIDs = make([]int64, 0, len(unique))
	for menuID := range unique {
		menuIDs = append(menuIDs, menuID)
	}
	sort.Slice(menuIDs, func(i, j int) bool { return menuIDs[i] < menuIDs[j] })
	audit := s.newAudit(ctx, actorID, actor, "iam.role.menus.update", "role_menu", fmt.Sprintf("%d", roleID), "success", nil)
	err := s.store.SetRoleMenus(ctx, *actorID, roleID, menuIDs, audit)
	switch {
	case errors.Is(err, repository.ErrRoleNotFound):
		return appErrors.NewClientError("IAM_ROLE_NOT_FOUND", nil)
	case errors.Is(err, repository.ErrRoleMenuSystemRole):
		return appErrors.NewClientError("IAM_SYSTEM_ROLE_PROTECTED", map[string]string{"role_menus": "system_role_is_read_only"})
	case errors.Is(err, repository.ErrRoleMenuInvalid):
		return appErrors.NewClientError("IAM_INVALID_ARGUMENT", map[string]string{"menu_ids": "must_reference_existing_menus"})
	default:
		return err
	}
}

func (s *Service) ListMenus(ctx *servicecontext.Context) ([]model.Menu, error) {
	if err := s.requirePermission(ctx, "iam.role.read"); err != nil {
		return nil, err
	}
	return s.store.ListMenus(ctx)
}

func (s *Service) CreateMenu(ctx *servicecontext.Context, input MenuInput) (*model.Menu, error) {
	if err := s.requirePermission(ctx, "iam.role.write"); err != nil {
		return nil, err
	}
	menu, err := normalizeMenuInput(input)
	if err != nil {
		return nil, err
	}
	actor, actorID := actorFromContext(ctx)
	audit := s.newAudit(ctx, actorID, actor, "iam.menu.create", "menu", menu.MenuCode, "success", nil)
	if err := s.store.CreateMenu(ctx, menu, audit); err != nil {
		if errors.Is(err, repository.ErrMenuCodeExists) {
			return nil, appErrors.NewClientError("IAM_MENU_CODE_EXISTS", nil)
		}
		if errors.Is(err, repository.ErrMenuParentInvalid) {
			return nil, appErrors.NewClientError("IAM_INVALID_ARGUMENT", map[string]string{"parent_id": "must_reference_active_parent_of_expected_type"})
		}
		return nil, fmt.Errorf("create IAM menu: %w", err)
	}
	return menu, nil
}

func (s *Service) UpdateMenu(ctx *servicecontext.Context, input MenuInput) (*model.Menu, error) {
	if err := s.requirePermission(ctx, "iam.role.write"); err != nil {
		return nil, err
	}
	if input.ID <= 0 || input.VersionNo <= 0 {
		return nil, appErrors.NewClientError("IAM_INVALID_ARGUMENT", map[string]string{"menu": "id_and_version_required"})
	}
	menu, err := normalizeMenuInput(input)
	if err != nil {
		return nil, err
	}
	menu.ID = input.ID
	actor, actorID := actorFromContext(ctx)
	audit := s.newAudit(ctx, actorID, actor, "iam.menu.update", "menu", fmt.Sprintf("%d", menu.ID), "success", nil)
	if err := s.store.UpdateMenu(ctx, menu, input.VersionNo, audit); err != nil {
		if errors.Is(err, repository.ErrMenuCodeExists) {
			return nil, appErrors.NewClientError("IAM_MENU_CODE_EXISTS", nil)
		}
		if errors.Is(err, repository.ErrMenuParentInvalid) {
			return nil, appErrors.NewClientError("IAM_INVALID_ARGUMENT", map[string]string{"parent_id": "must_reference_active_parent_of_expected_type"})
		}
		if errors.Is(err, repository.ErrMenuHasChildren) {
			return nil, appErrors.NewClientError("IAM_MENU_HAS_CHILDREN", nil)
		}
		if errors.Is(err, repository.ErrMenuNotFound) {
			return nil, appErrors.NewClientError("IAM_MENU_NOT_FOUND", nil)
		}
		if errors.Is(err, repository.ErrMenuVersionConflict) {
			return nil, appErrors.NewClientError("IAM_MENU_CONFLICT", nil)
		}
		return nil, fmt.Errorf("update IAM menu: %w", err)
	}
	return menu, nil
}

func (s *Service) DeleteMenu(ctx *servicecontext.Context, menuID int64) error {
	if err := s.requirePermission(ctx, "iam.role.write"); err != nil {
		return err
	}
	if menuID <= 0 {
		return appErrors.NewClientError("IAM_INVALID_ARGUMENT", map[string]string{"menu_id": "required_positive_integer"})
	}
	actor, actorID := actorFromContext(ctx)
	audit := s.newAudit(ctx, actorID, actor, "iam.menu.delete", "menu", fmt.Sprintf("%d", menuID), "success", nil)
	if err := s.store.DeleteMenu(ctx, menuID, audit); err != nil {
		switch {
		case errors.Is(err, repository.ErrMenuNotFound):
			return appErrors.NewClientError("IAM_MENU_NOT_FOUND", nil)
		case errors.Is(err, repository.ErrMenuHasChildren):
			return appErrors.NewClientError("IAM_MENU_HAS_CHILDREN", nil)
		case errors.Is(err, repository.ErrMenuAssignedToRole):
			return appErrors.NewClientError("IAM_MENU_ASSIGNED_TO_ROLE", nil)
		default:
			return fmt.Errorf("delete IAM menu: %w", err)
		}
	}
	return nil
}

func normalizeMenuInput(input MenuInput) (*model.Menu, error) {
	name, code := strings.TrimSpace(input.MenuName), strings.TrimSpace(input.MenuCode)
	path, component, icon := strings.TrimSpace(input.Path), strings.TrimSpace(input.Component), strings.TrimSpace(input.Icon)
	if name == "" || len(name) > 128 || code == "" || len(code) > 64 || input.SortOrder < 0 || input.SortOrder > 1000000 {
		return nil, appErrors.NewClientError("IAM_INVALID_ARGUMENT", map[string]string{"menu": "invalid_name_code_or_sort_order"})
	}
	for _, char := range code {
		if !(char >= 'a' && char <= 'z') && !(char >= 'A' && char <= 'Z') && !(char >= '0' && char <= '9') && char != '.' && char != '_' && char != ':' && char != '-' {
			return nil, appErrors.NewClientError("IAM_INVALID_ARGUMENT", map[string]string{"menu_code": "invalid_characters"})
		}
	}
	if input.MenuType != "directory" && input.MenuType != "page" && input.MenuType != "external" && input.MenuType != "button" {
		return nil, appErrors.NewClientError("IAM_INVALID_ARGUMENT", map[string]string{"menu_type": "unsupported"})
	}
	if input.Status != "active" && input.Status != "disabled" {
		return nil, appErrors.NewClientError("IAM_INVALID_ARGUMENT", map[string]string{"status": "unsupported"})
	}
	if len(path) > 255 || len(component) > 255 || len(icon) > 128 {
		return nil, appErrors.NewClientError("IAM_INVALID_ARGUMENT", map[string]string{"menu": "field_too_long"})
	}
	if input.MenuType == "page" && path == "" {
		return nil, appErrors.NewClientError("IAM_INVALID_ARGUMENT", map[string]string{"path": "required_for_page"})
	}
	if input.MenuType == "external" {
		parsed, err := url.ParseRequestURI(path)
		if err != nil || (parsed.Scheme != "https" && parsed.Scheme != "http") || parsed.Host == "" {
			return nil, appErrors.NewClientError("IAM_INVALID_ARGUMENT", map[string]string{"path": "must_be_absolute_http_url"})
		}
	}
	if input.MenuType == "button" && (path != "" || component != "") {
		return nil, appErrors.NewClientError("IAM_INVALID_ARGUMENT", map[string]string{"menu": "button_entries_cannot_define_route_or_component"})
	}
	if input.MenuType == "directory" && input.ParentID != nil {
		return nil, appErrors.NewClientError("IAM_INVALID_ARGUMENT", map[string]string{"parent_id": "directory_must_be_root"})
	}
	if input.MenuType != "directory" && (input.ParentID == nil || *input.ParentID <= 0) {
		return nil, appErrors.NewClientError("IAM_INVALID_ARGUMENT", map[string]string{"parent_id": "required_for_page"})
	}
	menu := &model.Menu{
		ParentID: input.ParentID, MenuName: name, MenuCode: code,
		Path: optional(path), Component: optional(component), Icon: optional(icon),
		SortOrder: input.SortOrder, MenuType: input.MenuType, Visible: input.Visible,
		Status: input.Status,
	}
	menu.VersionNo = input.VersionNo
	return menu, nil
}

func (s *Service) ListUserRoles(ctx *servicecontext.Context, userID int64) ([]model.Role, error) {
	if err := s.requirePermission(ctx, "iam.user.read"); err != nil {
		return nil, err
	}
	if userID <= 0 {
		return nil, appErrors.NewClientError("IAM_INVALID_ARGUMENT", map[string]string{"user_id": "required_positive_integer"})
	}
	user, err := s.store.FindUserByID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("load IAM role target: %w", err)
	}
	if user == nil || user.AccountType != model.AccountTypePlatform {
		return []model.Role{}, nil
	}
	return s.store.ListUserRoles(ctx, userID)
}

func (s *Service) ListRolePermissions(ctx *servicecontext.Context, roleID int64) ([]model.Permission, error) {
	if err := s.requirePermission(ctx, "iam.role.read"); err != nil {
		return nil, err
	}
	if roleID <= 0 {
		return nil, appErrors.NewClientError("IAM_INVALID_ARGUMENT", map[string]string{"role_id": "required_positive_integer"})
	}
	return s.store.ListRolePermissions(ctx, roleID)
}

func (s *Service) GrantRole(ctx *servicecontext.Context, userID, roleID int64) error {
	if userID <= 0 || roleID <= 0 {
		return appErrors.NewClientError("IAM_INVALID_ARGUMENT", map[string]string{"role_grant": "invalid"})
	}
	actor, actorID := actorFromContext(ctx)
	if actorID == nil || *actorID <= 0 || *actorID == userID {
		return appErrors.NewClientError("IAM_ROLE_GRANT_DENIED", nil)
	}
	if err := s.requirePermission(ctx, "iam.role.grant"); err != nil {
		return err
	}
	target, err := s.store.FindUserByID(ctx, userID)
	if err != nil {
		return fmt.Errorf("load IAM role target: %w", err)
	}
	if target == nil || target.AccountType != model.AccountTypePlatform {
		return appErrors.NewClientError("IAM_ROLE_GRANT_DENIED", map[string]string{"account_type": "platform_required"})
	}
	return s.store.GrantRole(ctx, *actorID, userID, roleID, s.newAudit(ctx, actorID, actor, "iam.role.grant", "user_role", fmt.Sprintf("%d:%d", userID, roleID), "success", nil))
}

func (s *Service) RevokeRole(ctx *servicecontext.Context, userID, roleID int64) error {
	if userID <= 0 || roleID <= 0 {
		return appErrors.NewClientError("IAM_INVALID_ARGUMENT", map[string]string{"role_revoke": "invalid"})
	}
	actor, actorID := actorFromContext(ctx)
	if actorID == nil || *actorID <= 0 || *actorID == userID {
		return appErrors.NewClientError("IAM_ROLE_REVOKE_DENIED", nil)
	}
	if err := s.requirePermission(ctx, "iam.role.grant"); err != nil {
		return err
	}
	target, err := s.store.FindUserByID(ctx, userID)
	if err != nil {
		return fmt.Errorf("load IAM role target: %w", err)
	}
	if target == nil || target.AccountType != model.AccountTypePlatform {
		return appErrors.NewClientError("IAM_ROLE_REVOKE_DENIED", map[string]string{"account_type": "platform_required"})
	}
	return s.store.RevokeRole(ctx, *actorID, userID, roleID, s.newAudit(ctx, actorID, actor, "iam.role.revoke", "user_role", fmt.Sprintf("%d:%d", userID, roleID), "success", nil))
}

func (s *Service) ListPermissionCatalog(ctx *servicecontext.Context) ([]model.Permission, error) {
	if err := s.requirePermission(ctx, "iam.role.read"); err != nil {
		return nil, err
	}
	return s.store.ListPermissionCatalog(ctx)
}

func (s *Service) ListResourceCatalog(ctx *servicecontext.Context) ([]model.Resource, error) {
	if err := s.requirePermission(ctx, "iam.role.read"); err != nil {
		return nil, err
	}
	return s.store.ListResourceCatalog(ctx)
}

func (s *Service) GrantPermission(ctx *servicecontext.Context, roleID, permissionID int64) error {
	if roleID <= 0 || permissionID <= 0 {
		return appErrors.NewClientError("IAM_INVALID_ARGUMENT", map[string]string{"permission_grant": "invalid"})
	}
	actor, actorID := actorFromContext(ctx)
	if actorID == nil || *actorID <= 0 {
		return appErrors.NewClientError("IAM_PERMISSION_GRANT_DENIED", nil)
	}
	if err := s.requirePermission(ctx, "iam.role.write"); err != nil {
		return err
	}
	return s.store.GrantPermission(ctx, *actorID, roleID, permissionID,
		s.newAudit(ctx, actorID, actor, "iam.permission.grant", "role_permission", fmt.Sprintf("%d:%d", roleID, permissionID), "success", nil))
}

func (s *Service) RevokePermission(ctx *servicecontext.Context, roleID, permissionID int64) error {
	if roleID <= 0 || permissionID <= 0 {
		return appErrors.NewClientError("IAM_INVALID_ARGUMENT", map[string]string{"permission_revoke": "invalid"})
	}
	actor, actorID := actorFromContext(ctx)
	if actorID == nil || *actorID <= 0 {
		return appErrors.NewClientError("IAM_PERMISSION_REVOKE_DENIED", nil)
	}
	if err := s.requirePermission(ctx, "iam.role.write"); err != nil {
		return err
	}
	return s.store.RevokePermission(ctx, *actorID, roleID, permissionID, s.newAudit(ctx, actorID, actor, "iam.permission.revoke", "role_permission", fmt.Sprintf("%d:%d", roleID, permissionID), "success", nil))
}

func (s *Service) SetUserStatus(ctx *servicecontext.Context, userID int64, status string) error {
	if userID <= 0 || (status != "active" && status != "disabled") {
		return appErrors.NewClientError("IAM_INVALID_ARGUMENT", map[string]string{"user_status": "invalid"})
	}
	actor, actorID := actorFromContext(ctx)
	if actorID == nil || *actorID <= 0 || *actorID == userID {
		return appErrors.NewClientError("IAM_USER_STATUS_DENIED", nil)
	}
	if err := s.requirePermission(ctx, "iam.user.write"); err != nil {
		return err
	}
	return s.store.SetUserStatus(ctx, *actorID, userID, status, s.newAudit(ctx, actorID, actor, "iam.user.status", "user", fmt.Sprintf("%d", userID), "success", nil))
}

func (s *Service) ResetUserPassword(ctx *servicecontext.Context, userID int64, newPassword string) error {
	if userID <= 0 || !validPasswordLength(newPassword) {
		return appErrors.NewClientError("IAM_INVALID_ARGUMENT", map[string]string{"password": "must_be_8_chars_max_1024_bytes"})
	}
	actor, actorID := actorFromContext(ctx)
	if actorID == nil || *actorID <= 0 || *actorID == userID {
		return appErrors.NewClientError("IAM_PASSWORD_RESET_DENIED", nil)
	}
	if err := s.requirePermission(ctx, "iam.user.write"); err != nil {
		return err
	}
	target, err := s.store.FindUserByID(ctx, userID)
	if err != nil {
		return fmt.Errorf("load IAM password reset target: %w", err)
	}
	if target == nil || (target.AccountType != model.AccountTypePlatform && target.AccountType != model.AccountTypeEmployee) {
		return appErrors.NewClientError("IAM_PASSWORD_RESET_DENIED", map[string]string{"account_type": "unsupported"})
	}
	salt, hash, err := hashPassword(newPassword)
	if err != nil {
		return fmt.Errorf("hash reset IAM password: %w", err)
	}
	audit := s.newAudit(ctx, actorID, actor, "iam.user.password_reset", "user", fmt.Sprintf("%d", userID), "success", nil)
	return s.store.ResetUserPassword(ctx, *actorID, userID, salt, hash, PasswordHashVersion, audit)
}

func (s *Service) ChangeOwnPassword(ctx *servicecontext.Context, currentPassword, newPassword string) error {
	userID, err := parseUserID(ctxUserID(ctx))
	if err != nil {
		return appErrors.NewAuthError("IAM_UNAUTHENTICATED", appErrors.AuthUnauthenticated)
	}
	if !validPasswordLength(newPassword) || currentPassword == newPassword {
		return appErrors.NewClientError("IAM_INVALID_ARGUMENT", map[string]string{"password": "must_be_new_and_8_chars_max_1024_bytes"})
	}
	user, err := s.store.FindUserByID(ctx, userID)
	if err != nil {
		return fmt.Errorf("load IAM password change user: %w", err)
	}
	if user == nil || user.AccountType != model.AccountTypePlatform || user.UserStatus != "active" || !verifyPassword(user, currentPassword) {
		return appErrors.NewAuthError("IAM_REAUTHENTICATION_REQUIRED", appErrors.AuthUnauthenticated)
	}
	if user.PasswordHash == nil {
		return appErrors.NewAuthError("IAM_REAUTHENTICATION_REQUIRED", appErrors.AuthUnauthenticated)
	}
	currentSessionID := strings.TrimSpace(ctx.EntityID())
	if currentSessionID == "" {
		return appErrors.NewAuthError("IAM_UNAUTHENTICATED", appErrors.AuthUnauthenticated)
	}
	salt, hash, err := hashPassword(newPassword)
	if err != nil {
		return fmt.Errorf("hash new IAM password: %w", err)
	}
	actor, actorID := actorFromContext(ctx)
	audit := s.newAudit(ctx, actorID, actor, "iam.user.password_change", "user", fmt.Sprintf("%d", userID), "success", nil)
	return s.store.ChangeOwnPassword(ctx, userID, *user.PasswordHash, salt, hash, PasswordHashVersion, currentSessionID, audit)
}

func (s *Service) requirePermission(ctx *servicecontext.Context, permission string) error {
	if ctx == nil {
		return appErrors.NewAuthError("IAM_UNAUTHENTICATED", appErrors.AuthUnauthenticated)
	}
	userID, err := parseUserID(ctx.UserID())
	if err != nil {
		return appErrors.NewAuthError("IAM_UNAUTHENTICATED", appErrors.AuthUnauthenticated)
	}
	allowed, err := s.store.HasPermission(ctx, userID, permission)
	if err != nil {
		return fmt.Errorf("check IAM permission %s: %w", permission, err)
	}
	if !allowed {
		return appErrors.NewAuthError("IAM_PERMISSION_DENIED", appErrors.AuthPermissionDenied)
	}
	return nil
}

func (s *Service) BootstrapAdmin(ctx context.Context, username, displayName, password string) error {
	serviceCtx := servicecontext.New(ctx, servicecontext.Principal{ID: "bootstrap", Name: "bootstrap"}, servicecontext.RequestMeta{})
	username = strings.ToLower(strings.TrimSpace(username))
	displayName = strings.TrimSpace(displayName)
	if username == "" || len(username) > 64 || len(displayName) > 128 {
		return appErrors.NewClientError("IAM_INVALID_ARGUMENT", map[string]string{"credentials": "invalid"})
	}
	if !validPasswordLength(password) {
		return appErrors.NewClientError("IAM_INVALID_ARGUMENT", map[string]string{"password": "must_be_8_chars_max_1024_bytes"})
	}
	salt, hash, err := hashPassword(password)
	if err != nil {
		return err
	}
	if displayName == "" {
		displayName = username
	}
	handle := make([]byte, 32)
	if _, err := rand.Read(handle); err != nil {
		return err
	}
	userHandle := &handle
	user := &model.User{AccountType: model.AccountTypePlatform, Username: username, DisplayName: displayName, WebAuthnUserHandle: userHandle, PasswordSalt: &salt, PasswordHash: &hash, PasswordHashVersion: int16Ptr(PasswordHashVersion), AuthSource: "local", UserStatus: "active", MustChangePassword: true}
	audit := &model.AuditLog{EventID: eventID(), UserName: "bootstrap", Action: "iam.bootstrap", TargetType: "user", TargetID: stringPtr(username), Result: "success", OccurredAt: time.Now().UTC()}
	return s.store.BootstrapAdmin(serviceCtx, user, audit)
}

func validPasswordLength(password string) bool {
	return utf8.RuneCountInString(password) >= minPasswordLength && len(password) <= maxPasswordBytes
}

func (s *Service) newAudit(ctx *servicecontext.Context, actorID *int64, actor, action, targetType, targetID, result string, reason *string) *model.AuditLog {
	var sourceIP, requestID *string
	if ctx != nil {
		if ctx.ClientIP() != "" {
			sourceIP = stringPtr(ctx.ClientIP())
		}
		if ctx.RequestID() != "" {
			requestID = stringPtr(ctx.RequestID())
		}
	}
	return &model.AuditLog{EventID: eventID(), UserID: actorID, UserName: actor, Action: action, TargetType: targetType, TargetID: optional(targetID), Result: result, Reason: reason, SourceIP: sourceIP, RequestID: requestID, OccurredAt: time.Now().UTC()}
}

func (s *Service) loginLog(user *model.User, loginType, result, reason, sourceIP, userAgent, requestID, sessionID string) *model.LoginLog {
	log := &model.LoginLog{EventID: eventID(), LoginType: loginType, EventType: "login", Result: result, OccurredAt: time.Now().UTC(), SourceIP: optional(sourceIP), UserAgent: optional(userAgent), RequestID: optional(requestID), SessionID: optional(sessionID)}
	if reason != "" {
		log.ReasonCode = stringPtr(reason)
	}
	if user != nil {
		log.UserID, log.UserName = int64Ptr(user.ID), user.Username
	}
	return log
}

func (s *Service) throttleHash(dimension, value string) string {
	mac := hmac.New(sha256.New, s.opts.ThrottleHMACKey)
	_, _ = mac.Write([]byte(dimension + "\x00" + value))
	return hex.EncodeToString(mac.Sum(nil))
}

func hashPassword(password string) (string, string, error) {
	salt := make([]byte, passwordSaltLength)
	if _, err := rand.Read(salt); err != nil {
		return "", "", err
	}
	hash := derivePassword(password, salt)
	return base64.RawStdEncoding.EncodeToString(salt), base64.RawStdEncoding.EncodeToString(hash), nil
}

func verifyPassword(user *model.User, password string) bool {
	var saltText, hashText string
	var version int16
	if user != nil && user.PasswordSalt != nil && user.PasswordHash != nil && user.PasswordHashVersion != nil {
		saltText, hashText, version = *user.PasswordSalt, *user.PasswordHash, *user.PasswordHashVersion
	} else {
		saltText, hashText, version = "AAAAAAAAAAAAAAAAAAAAAA", "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA", PasswordHashVersion
	}
	salt, err := base64.RawStdEncoding.DecodeString(saltText)
	if err != nil || len(salt) < 16 || version != PasswordHashVersion {
		return false
	}
	expected, err := base64.RawStdEncoding.DecodeString(hashText)
	if err != nil || len(expected) != argonKeyLength {
		return false
	}
	actual := derivePassword(password, salt)
	return hmac.Equal(actual, expected)
}

func derivePassword(password string, salt []byte) []byte {
	passwordKDFSlots <- struct{}{}
	defer func() { <-passwordKDFSlots }()
	return argon2.IDKey([]byte(password), salt, argonIterations, argonMemory, argonParallelism, argonKeyLength)
}

func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func randomHex(size int) (string, error) {
	value := make([]byte, size)
	if _, err := rand.Read(value); err != nil {
		return "", err
	}
	return hex.EncodeToString(value), nil
}

func eventID() string {
	value, err := randomHex(16)
	if err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return value
}

func view(user *model.User) UserView {
	if user == nil {
		return UserView{}
	}
	return UserView{ID: user.ID, AccountType: user.AccountType, Username: user.Username, DisplayName: user.DisplayName, Email: user.Email, UserStatus: user.UserStatus, MustChangePassword: user.MustChangePassword, LastLoginAt: user.LastLoginAt, CreatedAt: user.CreatedAt}
}

func parseUserID(value string) (int64, error) {
	var id int64
	if _, err := fmt.Sscan(value, &id); err != nil || id <= 0 {
		return 0, errors.New("invalid user ID")
	}
	return id, nil
}

func actorFromContext(ctx *servicecontext.Context) (string, *int64) {
	if ctx == nil {
		return "system", nil
	}
	id, err := parseUserID(ctx.UserID())
	if err != nil {
		return "system", nil
	}
	return ctx.UserName(), int64Ptr(id)
}

func optional(value string) *string {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	return &value
}

func stringPtr(value string) *string { return &value }
func int64Ptr(value int64) *int64    { return &value }
func int16Ptr(value int16) *int16    { return &value }

func invalidLogin() error {
	return appErrors.NewAuthError("IAM_LOGIN_FAILED", appErrors.AuthUnauthenticated)
}
