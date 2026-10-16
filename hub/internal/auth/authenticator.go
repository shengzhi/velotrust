package auth

import "context"

// Principal is the authenticated HUB identity, deliberately independent of
// Management database entities.
type Principal struct {
	NodeID string
	Kind   NodeKind
}

type NodeKind string

const (
	NodeKindClient  NodeKind = "client"
	NodeKindGateway NodeKind = "gateway"
)

// Authenticator validates a short-lived or renewable node credential. A
// successful transport TLS handshake alone must never authorize a node.
type Authenticator interface {
	Authenticate(context.Context, string) (Principal, error)
}
