package agent

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"velotrust/gateway/config"
)

var (
	ErrAgentStopped = errors.New("gateway agent is stopped")
)

const cleanupTimeout = 5 * time.Second

// Driver applies complete desired-state snapshots. Each apply method must be
// idempotent and replace its previous state rather than append to it.
type Driver interface {
	BlockForwarding(context.Context) error
	ApplyTunnel(context.Context, config.Tunnel) error
	// ApplyFirewall replaces all gateway rules. An empty slice must install a
	// default-deny forwarding policy.
	ApplyFirewall(context.Context, []config.ACLRule) error
	ApplyRoutes(context.Context, []string) error
	ApplyNAT(context.Context, []config.NATMapping) error
	EnableForwarding(context.Context) error
	UnblockForwarding(context.Context) error
	// Close must deactivate the tunnel and remove or disable owned forwarding
	// state, so it remains a fail-closed fallback if BlockForwarding fails.
	Close(context.Context) error
}

type Phase string

const (
	PhasePending  Phase = "pending"
	PhaseReady    Phase = "ready"
	PhaseDegraded Phase = "degraded"
	PhaseStopped  Phase = "stopped"
)

// Snapshot contains operational status without exposing tunnel keys or raw
// backend errors through the health endpoint.
type Snapshot struct {
	Phase     Phase     `json:"phase"`
	UpdatedAt time.Time `json:"updated_at"`
	Degraded  bool      `json:"degraded"`
}

type Reconciler struct {
	mu           sync.Mutex
	driver       Driver
	phase        Phase
	configDigest [sha256.Size]byte
	updatedAt    time.Time
	started      bool
	stopped      bool
}

func NewReconciler(driver Driver) (*Reconciler, error) {
	if driver == nil {
		return nil, errors.New("gateway dataplane driver is required")
	}
	return &Reconciler{driver: driver, phase: PhasePending, updatedAt: time.Now().UTC()}, nil
}

// Start puts the dataplane into a blocked state before the agent connects to
// its configuration source. It is safe to call more than once.
func (r *Reconciler) Start(ctx context.Context) error {
	if ctx == nil {
		return errors.New("context is required")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.stopped {
		return ErrAgentStopped
	}
	if r.started {
		return nil
	}
	slog.Debug("blocking gateway forwarding during startup")
	if err := r.driver.BlockForwarding(ctx); err != nil {
		slog.Error("failed to block gateway forwarding during startup", "error", err)
		return r.failClosedLocked(ctx, fmt.Errorf("block forwarding on startup: %w", err))
	}
	r.started = true
	r.updatedAt = time.Now().UTC()
	slog.Debug("gateway forwarding blocked during startup")
	return nil
}

// Apply validates and applies a full desired-state config. Traffic is
// blocked before mutation, firewall rules are installed before routes, and
// forwarding is enabled only after every stage succeeds.
func (r *Reconciler) Apply(ctx context.Context, desired config.Config) error {
	if ctx == nil {
		return errors.New("context is required")
	}
	desired = desired.Canonical()
	if err := desired.Validate(); err != nil {
		r.mu.Lock()
		defer r.mu.Unlock()
		if r.stopped {
			return ErrAgentStopped
		}
		return r.failClosedLocked(ctx, fmt.Errorf("validate gateway config: %w", err))
	}
	digest, err := digestConfig(desired)
	if err != nil {
		return fmt.Errorf("fingerprint gateway config: %w", err)
	}
	slog.Debug("applying gateway configuration",
		"gateway_id", desired.GatewayID,
		"interface", desired.Tunnel.Interface,
		"listen_port", desired.Tunnel.ListenPort,
		"peer_count", len(desired.Tunnel.Peers),
		"route_count", len(desired.Routes),
		"routes", desired.Routes,
		"rule_count", len(desired.Rules),
		"nat_count", len(desired.NAT),
	)

	r.mu.Lock()
	defer r.mu.Unlock()
	if r.stopped {
		return ErrAgentStopped
	}
	if digest == r.configDigest && r.phase == PhaseReady {
		return nil
	}

	stages := []struct {
		name string
		fn   func() error
	}{
		{name: "block forwarding", fn: func() error { return r.driver.BlockForwarding(ctx) }},
		{name: "apply tunnel", fn: func() error { return r.driver.ApplyTunnel(ctx, desired.Tunnel) }},
		{name: "apply firewall", fn: func() error { return r.driver.ApplyFirewall(ctx, append([]config.ACLRule(nil), desired.Rules...)) }},
		{name: "apply routes", fn: func() error { return r.driver.ApplyRoutes(ctx, append([]string(nil), desired.Routes...)) }},
		{name: "apply NAT", fn: func() error { return r.driver.ApplyNAT(ctx, append([]config.NATMapping(nil), desired.NAT...)) }},
		{name: "enable forwarding", fn: func() error { return r.driver.EnableForwarding(ctx) }},
		{name: "unblock forwarding", fn: func() error { return r.driver.UnblockForwarding(ctx) }},
	}
	for _, stage := range stages {
		startedAt := time.Now()
		slog.Debug("gateway configuration stage started", "stage", stage.name)
		if err := stage.fn(); err != nil {
			slog.Error("gateway configuration stage failed", "stage", stage.name, "error", err)
			return r.failClosedLocked(ctx, fmt.Errorf("%s: %w", stage.name, err))
		}
		slog.Debug("gateway configuration stage completed", "stage", stage.name, "duration", time.Since(startedAt))
	}
	r.configDigest = digest
	r.phase = PhaseReady
	r.started = true
	r.updatedAt = time.Now().UTC()
	slog.Info("gateway configuration applied", "gateway_id", desired.GatewayID, "phase", r.phase)
	return nil
}

// RejectConfiguration immediately blocks and closes the dataplane when the
// configuration source becomes unreadable or returns invalid data.
func (r *Reconciler) RejectConfiguration(ctx context.Context, cause error) error {
	if ctx == nil {
		return errors.New("context is required")
	}
	if cause == nil {
		return errors.New("configuration rejection cause is required")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.stopped {
		return ErrAgentStopped
	}
	return r.failClosedLocked(ctx, fmt.Errorf("reject gateway configuration: %w", cause))
}

// Stop blocks forwarded traffic and closes the active dataplane. It can be
// retried if either operation fails.
func (r *Reconciler) Stop(ctx context.Context) error {
	if ctx == nil {
		return errors.New("context is required")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.stopped {
		return nil
	}
	err := r.failClosedLocked(ctx, nil)
	if err != nil {
		return err
	}
	r.stopped = true
	r.phase = PhaseStopped
	r.updatedAt = time.Now().UTC()
	return nil
}

func (r *Reconciler) Status() Snapshot {
	r.mu.Lock()
	defer r.mu.Unlock()
	return Snapshot{
		Phase:     r.phase,
		UpdatedAt: r.updatedAt,
		Degraded:  r.phase == PhaseDegraded,
	}
}

func (r *Reconciler) failClosedLocked(ctx context.Context, cause error) error {
	cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), cleanupTimeout)
	defer cancel()
	blockErr := r.driver.BlockForwarding(cleanupCtx)
	closeErr := r.driver.Close(cleanupCtx)
	combined := errors.Join(cause, wrapErr("block forwarding", blockErr), wrapErr("close dataplane", closeErr))
	r.phase = PhaseDegraded
	r.updatedAt = time.Now().UTC()
	return combined
}

func digestConfig(cfg config.Config) ([sha256.Size]byte, error) {
	data, err := json.Marshal(cfg)
	if err != nil {
		return [sha256.Size]byte{}, err
	}
	return sha256.Sum256(data), nil
}

func wrapErr(operation string, err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%s: %w", operation, err)
}
