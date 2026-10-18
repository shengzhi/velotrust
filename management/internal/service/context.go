// Package service defines application-service request context primitives.
package service

import "context"

type Principal struct {
	ID, Name, OrgID, EntityID, Type string
	Roles                           []string
}

type RequestMeta struct {
	Language, RequestID, ClientIP string
}

// Context carries immutable request identity and metadata through services.
type Context struct {
	context.Context
	principal Principal
	meta      RequestMeta
}

func NewContext(parent context.Context, principal Principal, meta RequestMeta) *Context {
	if parent == nil {
		parent = context.Background()
	}
	principal.Roles = append([]string(nil), principal.Roles...)
	return &Context{Context: parent, principal: principal, meta: meta}
}

func NewSystemContext(parent context.Context) *Context {
	return NewContext(parent, Principal{ID: "system", Name: "system"}, RequestMeta{})
}
func (c *Context) UserID() string       { return c.principal.ID }
func (c *Context) UserName() string     { return c.principal.Name }
func (c *Context) OrgID() string        { return c.principal.OrgID }
func (c *Context) EntityID() string     { return c.principal.EntityID }
func (c *Context) IdentityType() string { return c.principal.Type }
func (c *Context) Roles() []string      { return append([]string(nil), c.principal.Roles...) }
func (c *Context) HasRole(role string) bool {
	for _, item := range c.principal.Roles {
		if item == role {
			return true
		}
	}
	return false
}
func (c *Context) Language() string  { return c.meta.Language }
func (c *Context) RequestID() string { return c.meta.RequestID }
func (c *Context) ClientIP() string  { return c.meta.ClientIP }
