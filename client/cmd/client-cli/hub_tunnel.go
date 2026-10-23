package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"sync"

	clientconfig "velotrust/client/internal/config"
	clientcontrol "velotrust/client/internal/control"
	clienttunnel "velotrust/client/internal/tunnel/wireguard"
	hubv1 "velotrust/shared/gen/go/hub/v1"
)

type hubClientConfiguration struct {
	SchemaVersion int `json:"schema_version"`
	Tunnel        struct {
		Address string              `json:"address"`
		MTU     int                 `json:"mtu"`
		Peers   []clientconfig.Peer `json:"peers"`
	} `json:"tunnel"`
}

type managedTunnelRun struct {
	cancel context.CancelFunc
	done   chan error
}

type hubTunnelManager struct {
	mu          sync.Mutex
	instanceID  string
	configPath  string
	keyFile     string
	output      io.Writer
	active      *managedTunnelRun
	appliedHash string
	wireguard   string
	health      string
	lastError   string
}

func newHubTunnelManager(instanceID, configPath, keyFile string, output io.Writer) *hubTunnelManager {
	return &hubTunnelManager{
		instanceID: instanceID, configPath: configPath, keyFile: keyFile, output: output,
		wireguard: "down", health: "degraded", lastError: "client_tunnel_not_configured",
	}
}

func (m *hubTunnelManager) Status() clientcontrol.RuntimeStatus {
	m.mu.Lock()
	defer m.mu.Unlock()
	return clientcontrol.RuntimeStatus{
		AppliedConfigHash: m.appliedHash, WireGuardStatus: m.wireguard,
		HealthStatus: m.health, LastErrorCode: m.lastError,
	}
}

func (m *hubTunnelManager) Apply(ctx context.Context, snapshot *hubv1.ConfigSnapshot) error {
	if snapshot == nil || len(snapshot.GetPayload()) == 0 {
		return errors.New("HUB Client snapshot is empty")
	}
	decoder := json.NewDecoder(bytes.NewReader(snapshot.GetPayload()))
	decoder.DisallowUnknownFields()
	var configuration hubClientConfiguration
	if err := decoder.Decode(&configuration); err != nil {
		return fmt.Errorf("decode HUB Client configuration: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return errors.New("HUB Client configuration contains trailing JSON data")
	}
	if configuration.SchemaVersion != 1 {
		return fmt.Errorf("unsupported HUB Client configuration schema version %d", configuration.SchemaVersion)
	}
	profile := clientconfig.Tunnel{
		InterfaceName: "VeloTrust", Address: configuration.Tunnel.Address,
		PrivateKeyFile: m.keyFile, MTU: configuration.Tunnel.MTU,
		Peers: configuration.Tunnel.Peers,
	}
	if err := profile.Validate(); err != nil {
		return fmt.Errorf("validate HUB Client tunnel configuration: %w", err)
	}
	m.mu.Lock()
	if m.appliedHash == snapshot.GetConfigHash() && m.active != nil {
		m.mu.Unlock()
		return nil
	}
	m.mu.Unlock()
	if err := m.stop(); err != nil {
		return fmt.Errorf("clean up previous Client tunnel before configuration update: %w", err)
	}
	runCtx, cancel := context.WithCancel(ctx)
	ready := make(chan error, 1)
	done := make(chan error, 1)
	go func() {
		err := clienttunnel.ConnectWithReady(runCtx, m.instanceID, m.configPath, profile, m.output, ready)
		done <- err
		m.mu.Lock()
		if m.active != nil && m.active.done == done {
			m.active = nil
			if err != nil && runCtx.Err() == nil {
				m.wireguard, m.health, m.lastError = "error", "unhealthy", "client_tunnel_runtime_failed"
			}
		}
		m.mu.Unlock()
	}()
	m.mu.Lock()
	m.active = &managedTunnelRun{cancel: cancel, done: done}
	m.wireguard, m.health, m.lastError = "starting", "unknown", ""
	m.mu.Unlock()
	select {
	case setupErr := <-ready:
		if setupErr != nil {
			cancel()
			cleanupErr := <-done
			m.mu.Lock()
			m.active = nil
			m.appliedHash = ""
			m.wireguard, m.health, m.lastError = "error", "unhealthy", "client_tunnel_apply_failed"
			m.mu.Unlock()
			return errors.Join(setupErr, cleanupErr)
		}
		m.mu.Lock()
		m.appliedHash = snapshot.GetConfigHash()
		m.wireguard, m.health, m.lastError = "up", "healthy", ""
		m.mu.Unlock()
		return nil
	case runErr := <-done:
		cancel()
		if runErr == nil {
			runErr = errors.New("Client tunnel stopped during configuration")
		}
		m.mu.Lock()
		m.active = nil
		m.wireguard, m.health, m.lastError = "error", "unhealthy", "client_tunnel_apply_failed"
		m.mu.Unlock()
		return runErr
	case <-ctx.Done():
		cancel()
		<-done
		return ctx.Err()
	}
}

func (m *hubTunnelManager) Stop() error {
	return m.stop()
}

func (m *hubTunnelManager) stop() error {
	m.mu.Lock()
	active := m.active
	if active == nil {
		m.appliedHash = ""
		m.wireguard, m.health, m.lastError = "down", "degraded", "client_tunnel_stopped"
		m.mu.Unlock()
		return nil
	}
	m.active = nil
	active.cancel()
	m.mu.Unlock()
	err := <-active.done
	if errors.Is(err, context.Canceled) || err == nil {
		err = nil
	}
	m.mu.Lock()
	m.appliedHash = ""
	m.wireguard, m.health, m.lastError = "down", "degraded", "client_tunnel_stopped"
	m.mu.Unlock()
	if err != nil {
		slog.Error("Client tunnel cleanup after HUB control event failed", "error", err)
	}
	return err
}
