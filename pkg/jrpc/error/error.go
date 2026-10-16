package jsonrpc

const (
	// ErrorCodeParse is parse error code.
	ErrorCodeParse ErrorCode = -32700
	// ErrorCodeInvalidRequest is invalid request error code.
	ErrorCodeInvalidRequest ErrorCode = -32600
	// ErrorCodeMethodNotFound is method not found error code.
	ErrorCodeMethodNotFound ErrorCode = -32601
	// ErrorCodeInvalidParams is invalid params error code.
	ErrorCodeInvalidParams ErrorCode = -32602
	// ErrorCodeInternal is internal error code.
	ErrorCodeInternal ErrorCode = -32603
	// ErrorCodeUnauthorized is authorization failure.
	ErrorCodeUnauthorized ErrorCode = -32001
	// ErrorCodeForbidden is permission failure.
	ErrorCodeForbidden ErrorCode = -32003
	// ErrorCodeNotFound is a requested domain resource was not found.
	ErrorCodeNotFound ErrorCode = -32004
	// ErrorCodeConflict is a domain state conflict.
	ErrorCodeConflict ErrorCode = -32009
)

type RpcError interface {
	Error() string
	ErrCode() int
}

type I18nString func(string) string

var _ RpcError = &Error{}

type (
	// A ErrorCode by JSON-RPC 2.0.
	ErrorCode int

	// An Error is a wrapper for a JSON interface value.
	Error struct {
		Code    ErrorCode `json:"code"`
		Message string    `json:"message"`
		Data    *Data     `json:"data,omitempty"`
	}

	// Data is the stable, safe business-error contract exposed to clients.
	Data struct {
		Category string            `json:"category"`
		Code     string            `json:"code"`
		Args     []any             `json:"args,omitempty"`
		Fields   map[string]string `json:"fields,omitempty"`
	}
)

func (e *Error) ErrCode() int {
	return int(e.Code)
}

// Error implements error interface.
func (e *Error) Error() string {
	return e.Message
}

// ErrParse returns parse error.
func ErrParse() *Error {
	return &Error{
		Code:    ErrorCodeParse,
		Message: "Parse error",
	}
}

// ErrInvalidRequest returns invalid request error.
func ErrInvalidRequest() *Error {
	return &Error{
		Code:    ErrorCodeInvalidRequest,
		Message: "BadRequest",
	}
}

// ErrMethodNotFound returns method not found error.
func ErrMethodNotFound() *Error {
	return &Error{
		Code:    ErrorCodeMethodNotFound,
		Message: "MethodNotFound",
	}
}

// ErrInvalidParams returns invalid params error.
func ErrInvalidParams() *Error {
	return &Error{
		Code:    ErrorCodeInvalidParams,
		Message: "InvalidParams",
	}
}

// ErrInternal returns internal error.
func ErrInternal() *Error {
	return &Error{
		Code:    ErrorCodeInternal,
		Message: "InternalError",
	}
}

// ErrUnauthorized returns authorization error.
func ErrUnauthorized() *Error {
	return &Error{
		Code:    ErrorCodeUnauthorized,
		Message: "Unauthorized",
	}
}

func ErrForbidden() *Error {
	return &Error{Code: ErrorCodeForbidden, Message: "Forbidden"}
}

func ErrNotFound() *Error {
	return &Error{Code: ErrorCodeNotFound, Message: "Not found"}
}

func ErrConflict() *Error {
	return &Error{Code: ErrorCodeConflict, Message: "Conflict"}
}
