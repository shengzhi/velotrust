package jrpc

import (
	"errors"
	"fmt"
	"strings"

	appErrors "velotrust/pkg/errors"
	"velotrust/pkg/jrpc/codec"
	jsonrpc "velotrust/pkg/jrpc/error"
)

// ErrorLocalizer translates a stable public business code for the request.
type ErrorLocalizer func(*Context, string, []any) string

// ErrorMapper converts an application error to the public RPC error model.
// Returning nil is treated as an internal error.
type ErrorMapper func(*Context, error) *jsonrpc.Error

func (ctx *Context) encodeError(err error) {
	if ctx == nil || ctx.Writer == nil || ctx.Writer.Written() {
		return
	}
	logMappedError(ctx, err)
	if ctx.errorMapper != nil {
		if mapped := ctx.errorMapper(ctx, err); mapped != nil {
			err = mapped
		} else {
			err = jsonrpc.ErrInternal()
		}
	}
	_ = codec.WriteJSONResponse(ctx.Writer, ctx.RequestID(), nil, err)
}

func logMappedError(ctx *Context, err error) {
	if ctx == nil || ctx.ilog == nil || err == nil {
		return
	}
	args := []interface{}{"request_id", ctx.RequestID(), "method", ctx.ServiceName(), "error", fmt.Sprintf("%+v", err)}
	var classified appErrors.ClassifiedError
	if errors.As(err, &classified) {
		args = append(args, "category", classified.Category(), "business_code", classified.GetCode())
		switch classified.Category() {
		case appErrors.CategoryClient, appErrors.CategoryAuth, appErrors.CategoryResource:
			ctx.ilog.Debug("rpc request rejected", args...)
		case appErrors.CategoryNetwork:
			ctx.ilog.Warn("rpc dependency error", args...)
		default:
			ctx.ilog.Error("rpc request failed", args...)
		}
		return
	}
	var rpcErr jsonrpc.RpcError
	if errors.As(err, &rpcErr) {
		ctx.ilog.Debug("rpc protocol error", args...)
		return
	}
	ctx.ilog.Error("rpc request failed", args...)
}

func defaultErrorMapper(localize ErrorLocalizer) ErrorMapper {
	return func(ctx *Context, err error) *jsonrpc.Error {
		if err == nil {
			return jsonrpc.ErrInternal()
		}
		var rpcErr jsonrpc.RpcError
		if errors.As(err, &rpcErr) {
			if value, ok := rpcErr.(*jsonrpc.Error); ok {
				return value
			}
			return &jsonrpc.Error{Code: jsonrpc.ErrorCode(rpcErr.ErrCode()), Message: rpcErr.Error()}
		}

		var clientErr *appErrors.ClientError
		if errors.As(err, &clientErr) {
			return mappedBusinessError(jsonrpc.ErrorCodeInvalidParams, appErrors.CategoryClient, clientErr.GetCode(), clientErr.GetArgs(), clientErr.InvalidFields, ctx, localize, "Invalid params")
		}
		var authErr *appErrors.AuthError
		if errors.As(err, &authErr) {
			rpcCode, message := jsonrpc.ErrorCodeUnauthorized, "Authentication failed"
			if authErr.Reason == appErrors.AuthPermissionDenied {
				rpcCode, message = jsonrpc.ErrorCodeForbidden, "Forbidden"
			}
			return mappedBusinessError(rpcCode, appErrors.CategoryAuth, authErr.GetCode(), authErr.GetArgs(), nil, ctx, localize, message)
		}
		var resourceErr *appErrors.ResourceError
		if errors.As(err, &resourceErr) {
			rpcCode, message := jsonrpc.ErrorCodeConflict, "Conflict"
			if resourceErr.IsNotFound {
				rpcCode, message = jsonrpc.ErrorCodeNotFound, "Not found"
			}
			return mappedBusinessError(rpcCode, appErrors.CategoryResource, resourceErr.GetCode(), resourceErr.GetArgs(), nil, ctx, localize, message)
		}
		var classified appErrors.ClassifiedError
		if errors.As(err, &classified) {
			return mappedBusinessError(jsonrpc.ErrorCodeInternal, classified.Category(), classified.GetCode(), classified.GetArgs(), nil, ctx, localize, "Internal error")
		}
		return mappedBusinessError(jsonrpc.ErrorCodeInternal, "internal", "INTERNAL_ERROR", nil, nil, ctx, localize, "Internal error")
	}
}

func mappedBusinessError(rpcCode jsonrpc.ErrorCode, category appErrors.Category, code string, args []any, fields map[string]string, ctx *Context, localize ErrorLocalizer, fallback string) *jsonrpc.Error {
	if code == "" {
		code = "INTERNAL_ERROR"
	}
	message := fallback
	if localize != nil {
		if translated := strings.TrimSpace(localize(ctx, code, args)); translated != "" {
			message = translated
		}
	}
	return &jsonrpc.Error{Code: rpcCode, Message: message, Data: &jsonrpc.Data{Category: string(category), Code: code, Args: args, Fields: fields}}
}
