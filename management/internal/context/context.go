// Package servicecontext carries request identity and metadata through the
// application and repository layers.
package context

import (
	"context"
)

type Principal struct {
	ID, Name, OrgID, EntityID, Type string
	Roles                           []string
}

type RequestMeta struct {
	Language, RequestID, ClientIP string
}

// Context carries immutable request identity and metadata. It implements
// context.Context so it can be passed directly through repository APIs.
type Context struct {
	context.Context
	principal Principal
	meta      RequestMeta
}

func New(parent context.Context, principal Principal, meta RequestMeta) *Context {
	if parent == nil {
		parent = context.Background()
	}
	principal.Roles = append([]string(nil), principal.Roles...)
	return &Context{Context: parent, principal: principal, meta: meta}
}

func NewSystem(parent context.Context) *Context {
	return New(parent, Principal{ID: "system", Name: "system"}, RequestMeta{})
}

// FromContext preserves cancellation and deadline while adding an explicit
// system identity to background work that has no authenticated principal.
func FromContext(parent context.Context) *Context {
	if current, ok := parent.(*Context); ok {
		return current
	}
	return NewSystem(parent)
}

// WithoutCancel retains the service identity and request metadata while
// detaching the context from its cancellation signal.
func (c *Context) WithoutCancel() *Context {
	if c == nil {
		return NewSystem(context.Background())
	}
	return New(context.WithoutCancel(c.Context), c.principal, c.meta)
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
