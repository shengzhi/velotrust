package report

import (
	"context"
	"errors"
	"time"
)

var ErrInvalidRuntimeStatus = errors.New("invalid Gateway runtime status")

// NodeStatus is a sanitized runtime report from a Client or Gateway Agent.
type NodeStatus struct {
	NodeID                  string
	RegisteredPublicKey     []byte
	AgentVersion            string
	AppliedConfigHash       string
	WireGuardStatus         string
	HealthStatus            string
	ConnectedPeerCount      uint32
	ConnectedPeerCountKnown bool
	LastErrorCode           string
	ReportedAt              time.Time
}

// Reporter persists or forwards node status without exposing the Management
// persistence model to HUB transport handlers.
type Reporter interface {
	Report(context.Context, NodeStatus) error
}

// HeartbeatRecorder persists the last authenticated heartbeat while checking
// that the node is still active and its registered key has not changed.
type HeartbeatRecorder interface {
	RecordHeartbeat(context.Context, string, []byte, time.Time) error
}
