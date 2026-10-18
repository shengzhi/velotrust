// Package constants contains the application's canonical constant values.
//
// Database table names intentionally remain with their models because they
// describe persistence mappings rather than application-wide constants.
package constants

import "time"

type UserStatus string

const (
	UserStatusActive   UserStatus = "ACTIVE"
	UserStatusLocked   UserStatus = "LOCKED"
	UserStatusDisabled UserStatus = "DISABLED"
)

type RoleStatus string

const (
	RoleCodeAdmin                 = "ADMIN"
	RoleStatusActive   RoleStatus = "ENABLED"
	RoleStatusDisabled RoleStatus = "DISABLED"
)

type PermissionType string

const (
	PermissionBusiness PermissionType = "BUSINESS"
	PermissionSystem   PermissionType = "SYSTEM"
)

type ResourceType string

const (
	ResourceTypeRPC     ResourceType = "RPC"
	ResourceTypeAPI     ResourceType = "API"
	ResourceTypeEvent   ResourceType = "EVENT"
	ResourceTypeService ResourceType = "SERVICE"
)

type AuditType string

const (
	AuditTypeLogin    AuditType = "LOGIN"
	AuditTypeAuth     AuditType = "AUTH"
	AuditTypeData     AuditType = "DATA"
	AuditTypeSystem   AuditType = "SYSTEM"
	AuditTypeSecurity AuditType = "SECURITY"
)

type AuditResult string

const (
	AuditResultSuccess AuditResult = "SUCCESS"
	AuditResultFailed  AuditResult = "FAILED"
)

type ModelState int

const (
	ModelStateNone ModelState = iota
	ModelStateNew
	ModelStateModified
	ModelStateSoftDeleted
	ModelStateHardDeleted
)

const (
	AuthorizationDeniedCode = "AUTHORIZATION_DENIED"
	MaxLoginFailures        = 5

	DefaultScheduleSyncInterval = 15 * time.Second
	DefaultScheduleTaskTimeout  = 30 * time.Minute
)
