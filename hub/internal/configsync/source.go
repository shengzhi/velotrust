package configsync

import "context"

// Snapshot is a versioned control-plane contract, not a Management database
// entity. Payload encoding is defined by the Agent configuration protocol.
type Snapshot struct {
	Revision string
	SHA256   []byte
	Payload  []byte
}

// Source resolves the latest authorized configuration for a node.
type Source interface {
	Current(context.Context, string, string) (Snapshot, bool, error)
}
