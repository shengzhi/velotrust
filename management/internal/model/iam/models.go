// Package iam contains persistence models mapped to the VeloTrust IAM schema.
// These structs intentionally do not expose credentials through JSON.
package iam

import "time"

const (
	TableUser               = "t_iam_user"
	TableUserProfile        = "t_iam_user_profile"
	TableGroup              = "t_iam_group"
	TableUserGroup          = "t_iam_user_group"
	TableRole               = "t_iam_role"
	TableUserRole           = "t_iam_user_role"
	TableResource           = "t_iam_resource"
	TablePermission         = "t_iam_permission"
	TableRolePermission     = "t_iam_role_permission"
	TableMenu               = "t_iam_menu"
	TableRoleMenu           = "t_iam_role_menu"
	TableRPCMethod          = "t_iam_rpc_method"
	TableUserSession        = "t_iam_user_session"
	TableAuthChallenge      = "t_iam_auth_challenge"
	TableUserAuthenticator  = "t_iam_user_authenticator"
	TableWebAuthnCredential = "t_iam_webauthn_credential"
	TableLoginLog           = "t_iam_login_log"
	TableAuditLog           = "t_iam_audit_log"
	TableAuthThrottle       = "t_iam_auth_throttle"
)

type User struct {
	ID                  int64      `xorm:"pk autoincr 'id'" json:"id"`
	Username            string     `xorm:"'username'" json:"username"`
	DisplayName         string     `xorm:"'display_name'" json:"display_name"`
	Email               *string    `xorm:"'email'" json:"email,omitempty"`
	PhoneCountryCode    *string    `xorm:"'phone_country_code'" json:"phone_country_code,omitempty"`
	PhoneNumber         *string    `xorm:"'phone_number'" json:"phone_number,omitempty"`
	WebAuthnUserHandle  []byte     `xorm:"'webauthn_user_handle'" json:"-"`
	PasswordSalt        *string    `xorm:"'password_salt'" json:"-"`
	PasswordHash        *string    `xorm:"'password_hash'" json:"-"`
	PasswordHashVersion *int16     `xorm:"'password_hash_version'" json:"-"`
	AuthSource          string     `xorm:"'auth_source'" json:"auth_source"`
	UserStatus          string     `xorm:"'user_status'" json:"user_status"`
	MustChangePassword  bool       `xorm:"'must_change_password'" json:"must_change_password"`
	LastLoginAt         *time.Time `xorm:"'last_login_at'" json:"last_login_at,omitempty"`
	LastLoginIP         *string    `xorm:"'last_login_ip'" json:"-"`
	Remark              *string    `xorm:"'remark'" json:"remark,omitempty"`
	CreatedAt           time.Time  `xorm:"'created_at'" json:"created_at"`
	CreatedBy           string     `xorm:"'created_by'" json:"-"`
	UpdatedAt           time.Time  `xorm:"'updated_at'" json:"updated_at"`
	UpdatedBy           string     `xorm:"'updated_by'" json:"-"`
	VersionNo           int64      `xorm:"'version_no'" json:"version_no"`
}

func (User) TableName() string { return TableUser }

type UserProfile struct {
	ID         int64     `xorm:"pk autoincr 'id'"`
	UserID     int64     `xorm:"'user_id'"`
	AvatarURL  *string   `xorm:"'avatar_url'"`
	JobTitle   *string   `xorm:"'job_title'"`
	EmployeeNo *string   `xorm:"'employee_no'"`
	Locale     *string   `xorm:"'locale'"`
	Timezone   *string   `xorm:"'timezone'"`
	ExtraJSON  []byte    `xorm:"'extra_json'"`
	CreatedAt  time.Time `xorm:"'created_at'"`
	CreatedBy  string    `xorm:"'created_by'"`
	UpdatedAt  time.Time `xorm:"'updated_at'"`
	UpdatedBy  string    `xorm:"'updated_by'"`
	VersionNo  int64     `xorm:"'version_no'"`
}

func (UserProfile) TableName() string { return TableUserProfile }

type Role struct {
	ID          int64     `xorm:"pk autoincr 'id'" json:"id"`
	RoleName    string    `xorm:"'role_name'" json:"role_name"`
	RoleCode    string    `xorm:"'role_code'" json:"role_code"`
	Description *string   `xorm:"'description'" json:"description,omitempty"`
	IsSystem    bool      `xorm:"'is_system'" json:"is_system"`
	Status      string    `xorm:"'status'" json:"status"`
	CreatedAt   time.Time `xorm:"'created_at'" json:"created_at"`
	CreatedBy   string    `xorm:"'created_by'" json:"-"`
	UpdatedAt   time.Time `xorm:"'updated_at'" json:"updated_at"`
	UpdatedBy   string    `xorm:"'updated_by'" json:"-"`
	VersionNo   int64     `xorm:"'version_no'" json:"version_no"`
}

func (Role) TableName() string { return TableRole }

type UserRole struct {
	ID        int64     `xorm:"pk autoincr 'id'"`
	UserID    int64     `xorm:"'user_id'"`
	RoleID    int64     `xorm:"'role_id'"`
	CreatedAt time.Time `xorm:"'created_at'"`
	CreatedBy string    `xorm:"'created_by'"`
}

func (UserRole) TableName() string { return TableUserRole }

type Resource struct {
	ID           int64     `xorm:"pk autoincr 'id'"`
	ResourceName string    `xorm:"'resource_name'"`
	ResourceCode string    `xorm:"'resource_code'"`
	ResourceType string    `xorm:"'resource_type'"`
	Description  *string   `xorm:"'description'"`
	Status       string    `xorm:"'status'"`
	CreatedAt    time.Time `xorm:"'created_at'"`
	CreatedBy    string    `xorm:"'created_by'"`
	UpdatedAt    time.Time `xorm:"'updated_at'"`
	UpdatedBy    string    `xorm:"'updated_by'"`
	VersionNo    int64     `xorm:"'version_no'"`
}

func (Resource) TableName() string { return TableResource }

type Permission struct {
	ID             int64     `xorm:"pk autoincr 'id'" json:"id"`
	PermissionName string    `xorm:"'permission_name'" json:"permission_name"`
	PermissionCode string    `xorm:"'permission_code'" json:"permission_code"`
	ResourceID     int64     `xorm:"'resource_id'" json:"resource_id"`
	ActionCode     string    `xorm:"'action_code'" json:"action_code"`
	Description    *string   `xorm:"'description'" json:"description,omitempty"`
	Status         string    `xorm:"'status'" json:"status"`
	CreatedAt      time.Time `xorm:"'created_at'" json:"created_at"`
	CreatedBy      string    `xorm:"'created_by'" json:"-"`
	UpdatedAt      time.Time `xorm:"'updated_at'" json:"updated_at"`
	UpdatedBy      string    `xorm:"'updated_by'" json:"-"`
	VersionNo      int64     `xorm:"'version_no'" json:"version_no"`
}

func (Permission) TableName() string { return TablePermission }

type RolePermission struct {
	ID           int64     `xorm:"pk autoincr 'id'"`
	RoleID       int64     `xorm:"'role_id'"`
	PermissionID int64     `xorm:"'permission_id'"`
	CreatedAt    time.Time `xorm:"'created_at'"`
	CreatedBy    string    `xorm:"'created_by'"`
}

func (RolePermission) TableName() string { return TableRolePermission }

// Menu is a navigation entry managed by the IAM console. menu_code is also
// the stable permission identifier consumed by frontend routing.
type Menu struct {
	ID        int64     `xorm:"pk autoincr 'id'" json:"id"`
	ParentID  *int64    `xorm:"'parent_id'" json:"parent_id"`
	MenuName  string    `xorm:"'menu_name'" json:"menu_name"`
	MenuCode  string    `xorm:"'menu_code'" json:"menu_code"`
	Path      *string   `xorm:"'path'" json:"path"`
	Component *string   `xorm:"'component'" json:"component"`
	Icon      *string   `xorm:"'icon'" json:"icon"`
	SortOrder int       `xorm:"'sort_order'" json:"sort_order"`
	MenuType  string    `xorm:"'menu_type'" json:"menu_type"`
	Visible   bool      `xorm:"'visible'" json:"visible"`
	Status    string    `xorm:"'status'" json:"status"`
	CreatedAt time.Time `xorm:"'created_at'" json:"created_at"`
	CreatedBy string    `xorm:"'created_by'" json:"-"`
	UpdatedAt time.Time `xorm:"'updated_at'" json:"updated_at"`
	UpdatedBy string    `xorm:"'updated_by'" json:"-"`
	VersionNo int64     `xorm:"'version_no'" json:"version_no"`
}

func (Menu) TableName() string { return TableMenu }

type UserSession struct {
	ID                 int64      `xorm:"pk autoincr 'id'"`
	SessionID          string     `xorm:"'session_id'"`
	UserID             int64      `xorm:"'user_id'"`
	TokenHash          string     `xorm:"'token_hash'"`
	AuthLevel          string     `xorm:"'auth_level'"`
	MFAAuthenticatedAt *time.Time `xorm:"'mfa_authenticated_at'"`
	IssuedAt           time.Time  `xorm:"'issued_at'"`
	ExpiresAt          time.Time  `xorm:"'expires_at'"`
	LastSeenAt         *time.Time `xorm:"'last_seen_at'"`
	RevokedAt          *time.Time `xorm:"'revoked_at'"`
	RevocationReason   *string    `xorm:"'revocation_reason'"`
	SourceIP           *string    `xorm:"'source_ip'"`
	UserAgent          *string    `xorm:"'user_agent'"`
	DeviceID           *int64     `xorm:"'device_id'"`
	Status             string     `xorm:"'status'"`
	CreatedAt          time.Time  `xorm:"'created_at'"`
}

func (UserSession) TableName() string { return TableUserSession }

type AuthChallenge struct {
	ID            int64      `xorm:"pk autoincr 'id'"`
	ChallengeID   string     `xorm:"'challenge_id'"`
	UserID        *int64     `xorm:"'user_id'"`
	SessionID     *string    `xorm:"'session_id'"`
	ChallengeType string     `xorm:"'challenge_type'"`
	ChallengeHash string     `xorm:"'challenge_hash'"`
	RPID          *string    `xorm:"'rp_id'"`
	Origin        *string    `xorm:"'origin'"`
	ExpiresAt     time.Time  `xorm:"'expires_at'"`
	AttemptCount  int        `xorm:"'attempt_count'"`
	MaxAttempts   int        `xorm:"'max_attempts'"`
	ConsumedAt    *time.Time `xorm:"'consumed_at'"`
	Status        string     `xorm:"'status'"`
	CreatedAt     time.Time  `xorm:"'created_at'"`
}

func (AuthChallenge) TableName() string { return TableAuthChallenge }

type UserAuthenticator struct {
	ID                   int64      `xorm:"pk autoincr 'id'"`
	UserID               int64      `xorm:"'user_id'"`
	AuthenticatorType    string     `xorm:"'authenticator_type'"`
	SecretCiphertext     string     `xorm:"'secret_ciphertext'"`
	EncryptionKeyVersion string     `xorm:"'encryption_key_version'"`
	Status               string     `xorm:"'status'"`
	VerifiedAt           *time.Time `xorm:"'verified_at'"`
	LastUsedAt           *time.Time `xorm:"'last_used_at'"`
	LastUsedStep         int64      `xorm:"'last_used_step'"`
	CreatedAt            time.Time  `xorm:"'created_at'"`
	UpdatedAt            time.Time  `xorm:"'updated_at'"`
}

func (UserAuthenticator) TableName() string { return TableUserAuthenticator }

type WebAuthnCredential struct {
	ID               int64      `xorm:"pk autoincr 'id'"`
	UserID           int64      `xorm:"'user_id'"`
	RPID             string     `xorm:"'rp_id'"`
	CredentialID     []byte     `xorm:"'credential_id'"`
	PublicKeyCOSE    []byte     `xorm:"'public_key_cose'"`
	SignatureCount   int64      `xorm:"'signature_count'"`
	AAGUID           []byte     `xorm:"'aaguid'"`
	Transports       string     `xorm:"'transports'"`
	BackupEligible   bool       `xorm:"'backup_eligible'"`
	BackupState      bool       `xorm:"'backup_state'"`
	CredentialName   string     `xorm:"'credential_name'"`
	Status           string     `xorm:"'status'"`
	CreatedAt        time.Time  `xorm:"'created_at'"`
	LastUsedAt       *time.Time `xorm:"'last_used_at'"`
	RevokedAt        *time.Time `xorm:"'revoked_at'"`
	RevocationReason *string    `xorm:"'revocation_reason'"`
}

func (WebAuthnCredential) TableName() string { return TableWebAuthnCredential }

type LoginLog struct {
	EventID    string    `xorm:"pk 'event_id'"`
	UserID     *int64    `xorm:"'user_id'"`
	UserName   string    `xorm:"'user_name'"`
	LoginType  string    `xorm:"'login_type'"`
	EventType  string    `xorm:"'event_type'"`
	Result     string    `xorm:"'result'"`
	ReasonCode *string   `xorm:"'reason_code'"`
	SourceIP   *string   `xorm:"'source_ip'"`
	UserAgent  *string   `xorm:"'user_agent'"`
	DeviceID   *int64    `xorm:"'device_id'"`
	SessionID  *string   `xorm:"'session_id'"`
	MFAUsed    bool      `xorm:"'mfa_used'"`
	RequestID  *string   `xorm:"'request_id'"`
	OccurredAt time.Time `xorm:"'occurred_at'"`
}

func (LoginLog) TableName() string { return TableLoginLog }

type AuditLog struct {
	EventID     string    `xorm:"pk 'event_id'"`
	UserID      *int64    `xorm:"'user_id'"`
	UserName    string    `xorm:"'user_name'"`
	Action      string    `xorm:"'action'"`
	TargetType  string    `xorm:"'target_type'"`
	TargetID    *string   `xorm:"'target_id'"`
	TargetName  *string   `xorm:"'target_name'"`
	Result      string    `xorm:"'result'"`
	Reason      *string   `xorm:"'reason'"`
	SourceIP    *string   `xorm:"'source_ip'"`
	RequestID   *string   `xorm:"'request_id'"`
	ChangesJSON []byte    `xorm:"'changes_json'"`
	OccurredAt  time.Time `xorm:"'occurred_at'"`
}

func (AuditLog) TableName() string { return TableAuditLog }

type AuthThrottle struct {
	ID              int64      `xorm:"pk autoincr 'id'"`
	BucketKeyHash   string     `xorm:"'bucket_key_hash'"`
	Dimension       string     `xorm:"'dimension'"`
	WindowStartedAt time.Time  `xorm:"'window_started_at'"`
	FailureCount    int        `xorm:"'failure_count'"`
	BlockedUntil    *time.Time `xorm:"'blocked_until'"`
	UpdatedAt       time.Time  `xorm:"'updated_at'"`
}

func (AuthThrottle) TableName() string { return TableAuthThrottle }
