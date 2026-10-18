package iam

import (
	"errors"

	servicecontext "velotrust/management/internal/context"
	"velotrust/management/internal/model/iam"
)

var (
	ErrMenuNotFound        = errors.New("IAM menu not found")
	ErrMenuVersionConflict = errors.New("IAM menu was changed by another operator")
	ErrMenuCodeExists      = errors.New("IAM menu code already exists")
	ErrMenuParentInvalid   = errors.New("IAM menu parent has an invalid type or status")
	ErrMenuHasChildren     = errors.New("IAM menu has child entries")
	ErrMenuAssignedToRole  = errors.New("IAM menu is assigned to a role")
	ErrRoleNotFound        = errors.New("IAM role not found")
	ErrRoleMenuSystemRole  = errors.New("menus for a system role cannot be changed")
	ErrRoleMenuInvalid     = errors.New("IAM role menu assignment contains an invalid menu")
)

// Store is the IAM persistence boundary. Implementations own SQL and transaction
// details; services receive models only through this interface.
type Store interface {
	FindUserByUsername(*servicecontext.Context, string) (*iam.User, error)
	FindUserByID(*servicecontext.Context, int64) (*iam.User, error)
	CreateUser(*servicecontext.Context, *iam.User, *iam.AuditLog) error
	CreateLoginSession(*servicecontext.Context, *iam.UserSession, *iam.LoginLog, string, string, string) error
	RecordLoginFailure(*servicecontext.Context, *iam.LoginLog, string, string, int, int, int) error
	LoginBlocked(*servicecontext.Context, string, string) (bool, error)
	ClearLoginThrottle(*servicecontext.Context, string, string) error
	ResolveSession(*servicecontext.Context, string) (*iam.UserSession, *iam.User, error)
	TouchSession(*servicecontext.Context, string, string, string) error
	RevokeSession(*servicecontext.Context, string, string) error
	RevokeUserSessions(*servicecontext.Context, int64, string) error
	HasPermission(*servicecontext.Context, int64, string) (bool, error)
	ListPermissions(*servicecontext.Context, int64) ([]string, error)
	ListUsers(*servicecontext.Context, int, int) ([]iam.User, int64, error)
	CreateRole(*servicecontext.Context, *iam.Role, *iam.AuditLog) error
	ListRoles(*servicecontext.Context) ([]iam.Role, error)
	ListUserRoles(*servicecontext.Context, int64) ([]iam.Role, error)
	ListRolePermissions(*servicecontext.Context, int64) ([]iam.Permission, error)
	ListRoleMenus(*servicecontext.Context, int64) ([]iam.Menu, error)
	ListUserMenus(*servicecontext.Context, int64) ([]iam.Menu, error)
	SetRoleMenus(*servicecontext.Context, int64, int64, []int64, *iam.AuditLog) error
	ListMenus(*servicecontext.Context) ([]iam.Menu, error)
	CreateMenu(*servicecontext.Context, *iam.Menu, *iam.AuditLog) error
	UpdateMenu(*servicecontext.Context, *iam.Menu, int64, *iam.AuditLog) error
	DeleteMenu(*servicecontext.Context, int64, *iam.AuditLog) error
	GrantRole(*servicecontext.Context, int64, int64, int64, *iam.AuditLog) error
	RevokeRole(*servicecontext.Context, int64, int64, int64, *iam.AuditLog) error
	ListPermissionCatalog(*servicecontext.Context) ([]iam.Permission, error)
	ListResourceCatalog(*servicecontext.Context) ([]iam.Resource, error)
	GrantPermission(*servicecontext.Context, int64, int64, int64, *iam.AuditLog) error
	RevokePermission(*servicecontext.Context, int64, int64, int64, *iam.AuditLog) error
	SetUserStatus(*servicecontext.Context, int64, int64, string, *iam.AuditLog) error
	ResetUserPassword(*servicecontext.Context, int64, int64, string, string, int16, *iam.AuditLog) error
	ChangeOwnPassword(*servicecontext.Context, int64, string, string, string, int16, string, *iam.AuditLog) error
	FindTOTPAuthenticator(*servicecontext.Context, int64) (*iam.UserAuthenticator, error)
	SavePendingTOTP(*servicecontext.Context, *iam.UserAuthenticator, *iam.AuditLog) error
	ActivateTOTP(*servicecontext.Context, int64, int64, *iam.AuditLog) error
	ConsumeTOTP(*servicecontext.Context, int64, int64) error
	DisableTOTP(*servicecontext.Context, int64, int64, *iam.AuditLog) error
	WriteAudit(*servicecontext.Context, *iam.AuditLog) error
	BootstrapAdmin(*servicecontext.Context, *iam.User, *iam.AuditLog) error
	ListWebAuthnCredentials(*servicecontext.Context, int64, string) ([]iam.WebAuthnCredential, error)
	CreateAuthChallenge(*servicecontext.Context, *iam.AuthChallenge) error
	ConsumeAuthChallenge(*servicecontext.Context, string, string) (*iam.AuthChallenge, error)
	CreateWebAuthnCredential(*servicecontext.Context, *iam.WebAuthnCredential, *iam.AuditLog) error
	UpdateWebAuthnCredentialUse(*servicecontext.Context, *iam.WebAuthnCredential) error
	RevokeWebAuthnCredential(*servicecontext.Context, int64, int64, *iam.AuditLog) error
	FindUserByCredential(*servicecontext.Context, string, []byte) (*iam.User, []iam.WebAuthnCredential, error)
}
