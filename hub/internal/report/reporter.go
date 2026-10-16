package report

import "context"

// NodeStatus is a sanitized runtime report from a Client or Gateway Agent.
type NodeStatus struct {
	NodeID           string
	AppliedRevision  string
	Connected        bool
	WireGuardPeersUp uint32
	LastErrorCode    string
}

// Reporter persists or forwards node status without exposing the Management
// persistence model to HUB transport handlers.
type Reporter interface {
	Report(context.Context, NodeStatus) error
}
