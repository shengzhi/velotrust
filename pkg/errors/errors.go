package errors

import (
	"fmt"
)

// ==========================================
// 1. 客户端异常（参数、越权等不触发告警的错误）
// ==========================================

type ClientError struct {
	baseError
	InvalidFields map[string]string // 特有字段：记录表单单项校验失败的原因
}

func (*ClientError) Category() Category { return CategoryClient }

func (e *ClientError) Error() string {
	if e.cause != nil {
		return fmt.Sprintf("[ClientError] Code: %s, Fields: %v; Cause: %v", e.code, e.InvalidFields, e.cause)
	}
	return fmt.Sprintf("[ClientError] Code: %s, Fields: %v", e.code, e.InvalidFields)
}

func (e *ClientError) Format(s fmt.State, verb rune) { formatMessage(s, e.Error(), verb) }

func NewClientError(code string, fields map[string]string, args ...any) error {
	return &ClientError{
		baseError:     baseError{code: code, args: args, stack: captureStack(3)},
		InvalidFields: fields,
	}
}

// ==========================================
// 2. 数据库异常（核心系统隐患，需携带 SQL 与原始驱动错误）
// ==========================================

type DatabaseError struct {
	baseError
	Query string // 特有字段：触发异常的 SQL 语句快照（仅供日志追踪）
}

func (*DatabaseError) Category() Category { return CategoryDatabase }

func (e *DatabaseError) Error() string {
	return fmt.Sprintf("[DatabaseError] Code: %s, SQL: %s; Cause: %v", e.code, e.Query, e.cause)
}

func (e *DatabaseError) Format(s fmt.State, verb rune) { e.baseError.format(s, e.Error(), verb) }

func WrapDatabaseError(err error, code string, query string, args ...any) error {
	if err == nil {
		return nil
	}
	return &DatabaseError{
		baseError: baseError{code: code, args: args, cause: err, stack: captureStack(3)},
		Query:     query,
	}
}

// ==========================================
// 3. 网络/第三方向异常（支持重试判定）
// ==========================================

type NetworkError struct {
	baseError
	TargetURL string // 特有字段：请求的目标三方 API 地址
}

func (*NetworkError) Category() Category { return CategoryNetwork }

func (e *NetworkError) Error() string {
	return fmt.Sprintf("[NetworkError] Code: %s, URL: %s; Cause: %v", e.code, e.TargetURL, e.cause)
}

func (e *NetworkError) Format(s fmt.State, verb rune) { e.baseError.format(s, e.Error(), verb) }

func WrapNetworkError(err error, code string, url string, args ...any) error {
	if err == nil {
		return nil
	}
	return &NetworkError{
		baseError: baseError{code: code, args: args, cause: err, stack: captureStack(3)},
		TargetURL: url,
	}
}

type AuthReason uint8

const (
	AuthUnauthenticated AuthReason = iota
	AuthTokenExpired
	AuthPermissionDenied
)

type AuthError struct {
	baseError
	Reason AuthReason
}

func (*AuthError) Category() Category { return CategoryAuth }

func (e *AuthError) Error() string {
	switch e.Reason {
	case AuthTokenExpired:
		return "Token expired"
	case AuthPermissionDenied:
		return "Unauthorized"
	default:
		return "Unauthenticated"
	}
}

func (e *AuthError) Format(s fmt.State, verb rune) { formatMessage(s, e.Error(), verb) }

func NewAuthError(code string, reason AuthReason, args ...any) error {
	return &AuthError{baseError: baseError{code: code, args: args, stack: captureStack(3)}, Reason: reason}
}

type ResourceError struct {
	baseError
	ResourceName string // 特异字段：资源大类名（如: "Order", "Product"）
	IsNotFound   bool   // 区分是不存在(404)还是状态冲突(409)
}

func (*ResourceError) Category() Category { return CategoryResource }

func (e *ResourceError) Error() string {
	if e.IsNotFound {
		return fmt.Sprintf("resource %s not found", e.ResourceName)
	}
	return fmt.Sprintf("resource %s is changed already", e.ResourceName)
}

func (e *ResourceError) Format(s fmt.State, verb rune) { formatMessage(s, e.Error(), verb) }

func NewResourceError(code string, name string, isNotFound bool, args ...any) error {
	return &ResourceError{
		baseError: baseError{code: code, args: args, stack: captureStack(3)}, ResourceName: name, IsNotFound: isNotFound}
}
