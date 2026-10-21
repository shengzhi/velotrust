package control

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"log/slog"
	"sync"

	"velotrust/gateway/config"
	"velotrust/gateway/internal/agent"
)

// Manager follows valid local config replacements and restarts the control
// stream when its endpoint or key identity changes.
type Manager struct {
	mu          sync.Mutex
	cancel      context.CancelFunc
	done        chan struct{}
	current     target
	active      bool
	ready       chan struct{}
	readyTarget target
}

type target struct {
	endpoint      string
	interfaceName string
	serverName    string
	caFile        string
	tlsEnabled    bool
	privateKey    string
	tokenHash     [sha256.Size]byte
}

func (m *Manager) Update(ctx context.Context, cfg config.Config, statusProvider interface{ Status() agent.Snapshot }, applySnapshot SnapshotApplier, onDisconnected DisconnectHandler) error {
	if ctx == nil {
		return errors.New("Gateway control manager context is required")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.refreshLocked()
	if m.ready == nil {
		m.ready = make(chan struct{})
	}
	if cfg.Hub == nil {
		m.stopLocked()
		m.ready = nil
		m.readyTarget = target{}
		return nil
	}
	if statusProvider == nil {
		return errors.New("Gateway status provider is required when HUB control is enabled")
	}
	hubConfig := *cfg.Hub
	registrationToken := []byte(hubConfig.RegistrationToken)
	hubConfig.RegistrationToken = ""
	desired := target{
		endpoint:      hubConfig.Endpoint,
		interfaceName: cfg.Tunnel.Interface,
		serverName:    hubConfig.ServerName,
		caFile:        hubConfig.CAFile,
		tlsEnabled:    hubConfig.TLSEnabled(),
		privateKey:    cfg.Tunnel.PrivateKey,
		tokenHash:     sha256.Sum256(registrationToken),
	}
	if m.active && m.current == desired {
		clear(registrationToken)
		return nil
	}
	targetChanged := m.active && m.current != desired
	m.stopLocked()
	if targetChanged && onDisconnected != nil {
		if err := onDisconnected(); err != nil {
			clear(registrationToken)
			return fmt.Errorf("block Gateway dataplane while changing HUB target: %w", err)
		}
	}
	if m.ready == nil || m.readyTarget != desired {
		m.ready = make(chan struct{})
		m.readyTarget = desired
	}
	controlContext, cancel := context.WithCancel(ctx)
	done := make(chan struct{})
	m.cancel = cancel
	m.done = done
	m.current = desired
	m.active = true
	ready := m.ready
	go func() {
		defer close(done)
		defer clear(registrationToken)
		if err := Run(controlContext, hubConfig, desired.privateKey, registrationToken, statusProvider, applySnapshot, func() {
			select {
			case <-ready:
			default:
				close(ready)
			}
		}, onDisconnected); err != nil && controlContext.Err() == nil {
			slog.Error("Gateway HUB control client stopped", "endpoint", desired.endpoint, "error", err)
		}
	}()
	return nil
}

// Ready closes after the current HUB target has an authenticated session. It
// remains closed across reconnects and resets when the target identity changes.
func (m *Manager) Ready() <-chan struct{} {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.ready == nil {
		m.ready = make(chan struct{})
	}
	return m.ready
}

func (m *Manager) Stop() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.stopLocked()
}

func (m *Manager) stopLocked() {
	if m.cancel != nil {
		m.cancel()
	}
	if m.done != nil {
		<-m.done
	}
	m.cancel = nil
	m.done = nil
	m.current = target{}
	m.active = false
}

func (m *Manager) refreshLocked() {
	if !m.active || m.done == nil {
		return
	}
	select {
	case <-m.done:
		m.cancel = nil
		m.done = nil
		m.current = target{}
		m.active = false
	default:
	}
}
