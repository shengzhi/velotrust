package configsync

import (
	"context"
	"errors"
)

var (
	ErrTargetUnavailable           = errors.New("Gateway configuration target is no longer active")
	ErrConfigurationRemoved        = errors.New("Gateway current configuration was removed")
	ErrInvalidConfiguration        = errors.New("Gateway current configuration is invalid")
	ErrClientAddressNotAssigned    = errors.New("Client has no active Overlay address lease")
	ErrClientConfigurationNotReady = errors.New("Client configuration is waiting for an authorized Gateway")
)

// Snapshot is a versioned control-plane contract, not a Management database
// entity. Payload encoding is defined by the Agent configuration protocol.
type Snapshot struct {
	Revision string
	TaskID   string
	Payload  []byte
}

// Source resolves the latest authorized configuration for a node.
type Source interface {
	Current(context.Context, string, string) (Snapshot, bool, error)
	MarkSent(context.Context, string, Snapshot) error
	MarkApplied(context.Context, string, string) error
	Acknowledge(context.Context, string, string, string, bool, string, string) error
}
