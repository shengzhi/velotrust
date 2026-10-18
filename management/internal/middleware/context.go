package middleware

import (
	"context"

	servicecontext "velotrust/management/internal/context"
	"velotrust/pkg/jrpc"
)

func ServiceContext(ctx *jrpc.Context) *servicecontext.Context {
	if ctx == nil {
		return servicecontext.NewSystem(nil)
	}
	var principal servicecontext.Principal
	if ctx.Identity != nil {
		principal = servicecontext.Principal{ID: ctx.Identity.ID(), Name: ctx.Identity.Name(), OrgID: ctx.Identity.OrgID(), EntityID: ctx.Identity.EntityID(), Type: ctx.Identity.Type(), Roles: ctx.Identity.Roles()}
	}
	if ctx.HttpRequest == nil {
		return servicecontext.New(nil, principal, servicecontext.RequestMeta{})
	}
	parent := ctx.HttpRequest.Context()
	return servicecontext.New(parent, principal, servicecontext.RequestMeta{Language: ctx.Language(), RequestID: ctx.RequestID(), ClientIP: ctx.ClientIP()})
}

// AuthenticationContext supplies a stable system actor for public credential
// ceremonies, which have no authenticated user yet.
func AuthenticationContext(ctx *jrpc.Context) *servicecontext.Context {
	var parentCtx = requestContext(ctx)
	meta := servicecontext.RequestMeta{}
	if ctx != nil {
		meta = servicecontext.RequestMeta{Language: ctx.Language(), RequestID: ctx.RequestID(), ClientIP: ctx.ClientIP()}
	}
	return servicecontext.New(parentCtx, servicecontext.Principal{ID: "authentication", Name: "authentication"}, meta)
}

func requestContext(ctx *jrpc.Context) context.Context {
	if ctx != nil && ctx.HttpRequest != nil {
		return ctx.HttpRequest.Context()
	}
	return context.Background()
}
