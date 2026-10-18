package middleware

import (
	service "velotrust/management/internal/service"
	"velotrust/pkg/jrpc"
)

func ServiceContext(ctx *jrpc.Context) *service.Context {
	if ctx == nil {
		return service.NewSystemContext(nil)
	}
	var principal service.Principal
	if ctx.Identity != nil {
		principal = service.Principal{ID: ctx.Identity.ID(), Name: ctx.Identity.Name(), OrgID: ctx.Identity.OrgID(), EntityID: ctx.Identity.EntityID(), Type: ctx.Identity.Type(), Roles: ctx.Identity.Roles()}
	}
	if ctx.HttpRequest == nil {
		return service.NewContext(nil, principal, service.RequestMeta{})
	}
	parent := ctx.HttpRequest.Context()
	return service.NewContext(parent, principal, service.RequestMeta{Language: ctx.Language(), RequestID: ctx.RequestID(), ClientIP: ctx.ClientIP()})
}
