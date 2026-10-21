package session

import (
	"context"
	"time"
)

// Record describes a renewable node connection lease owned by one HUB
// instance. Implementations may use a shared database for cross-instance
// coordination; process memory must not be the sole authority in HA mode.
type Record struct {
	SessionID  string
	NodeID     string
	InstanceID string
	LeaseUntil time.Time
}

// Store coordinates active node sessions and expires abandoned leases.
type Store interface {
	Acquire(context.Context, Record) error
	Renew(context.Context, string, string, time.Time) error
	WithCurrent(context.Context, string, string, time.Time, func() error) error
	Release(context.Context, string, string) error
	ExpireBefore(context.Context, time.Time) (int64, error)
}
