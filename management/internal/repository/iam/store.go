package iam

import (
	"context"
	"errors"

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
	FindUserByUsername(context.Context, string) (*iam.User, error)
	FindUserByID(context.Context, int64) (*iam.User, error)
	CreateUser(context.Context, *iam.User, *iam.AuditLog) error
	CreateLoginSession(context.Context, *iam.UserSession, *iam.LoginLog, string, string, string) error
	RecordLoginFailure(context.Context, *iam.LoginLog, string, string, int, int, int) error
	LoginBlocked(context.Context, string, string) (bool, error)
	ClearLoginThrottle(context.Context, string, string) error
	ResolveSession(context.Context, string) (*iam.UserSession, *iam.User, error)
	TouchSession(context.Context, string, string, string) error
	RevokeSession(context.Context, string, string) error
	RevokeUserSessions(context.Context, int64, string) error
	HasPermission(context.Context, int64, string) (bool, error)
	ListPermissions(context.Context, int64) ([]string, error)
	ListUsers(context.Context, int, int) ([]iam.User, int64, error)
	CreateRole(context.Context, *iam.Role, *iam.AuditLog) error
	ListRoles(context.Context) ([]iam.Role, error)
	ListUserRoles(context.Context, int64) ([]iam.Role, error)
	ListRolePermissions(context.Context, int64) ([]iam.Permission, error)
	ListRoleMenus(context.Context, int64) ([]iam.Menu, error)
	ListUserMenus(context.Context, int64) ([]iam.Menu, error)
	SetRoleMenus(context.Context, int64, int64, []int64, *iam.AuditLog) error
	ListMenus(context.Context) ([]iam.Menu, error)
	CreateMenu(context.Context, *iam.Menu, *iam.AuditLog) error
	UpdateMenu(context.Context, *iam.Menu, int64, *iam.AuditLog) error
	DeleteMenu(context.Context, int64, *iam.AuditLog) error
	GrantRole(context.Context, int64, int64, int64, *iam.AuditLog) error
	RevokeRole(context.Context, int64, int64, int64, *iam.AuditLog) error
	ListPermissionCatalog(context.Context) ([]iam.Permission, error)
	GrantPermission(context.Context, int64, int64, int64, *iam.AuditLog) error
	RevokePermission(context.Context, int64, int64, int64, *iam.AuditLog) error
	SetUserStatus(context.Context, int64, int64, string, string, *iam.AuditLog) error
	ResetUserPassword(context.Context, int64, int64, string, string, int16, string, *iam.AuditLog) error
	ChangeOwnPassword(context.Context, int64, string, string, string, int16, string, *iam.AuditLog) error
	FindTOTPAuthenticator(context.Context, int64) (*iam.UserAuthenticator, error)
	SavePendingTOTP(context.Context, *iam.UserAuthenticator, *iam.AuditLog) error
	ActivateTOTP(context.Context, int64, int64, *iam.AuditLog) error
	ConsumeTOTP(context.Context, int64, int64) error
	DisableTOTP(context.Context, int64, int64, *iam.AuditLog) error
	WriteAudit(context.Context, *iam.AuditLog) error
	BootstrapAdmin(context.Context, *iam.User, *iam.AuditLog) error
	ListWebAuthnCredentials(context.Context, int64, string) ([]iam.WebAuthnCredential, error)
	CreateAuthChallenge(context.Context, *iam.AuthChallenge) error
	ConsumeAuthChallenge(context.Context, string, string) (*iam.AuthChallenge, error)
	CreateWebAuthnCredential(context.Context, *iam.WebAuthnCredential, *iam.AuditLog) error
	UpdateWebAuthnCredentialUse(context.Context, *iam.WebAuthnCredential) error
	RevokeWebAuthnCredential(context.Context, int64, int64, *iam.AuditLog) error
	FindUserByCredential(context.Context, string, []byte) (*iam.User, []iam.WebAuthnCredential, error)
}
