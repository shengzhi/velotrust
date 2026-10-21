package runtime

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"time"

	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
	"velotrust/gateway/config"
	"velotrust/gateway/internal/agent"
	"velotrust/gateway/internal/control"
	"velotrust/gateway/platform"
	hubv1 "velotrust/shared/gen/go/hub/v1"
)

const (
	shutdownTimeout               = 5 * time.Second
	dockerFirewallRefreshInterval = 15 * time.Second
)

// Run applies one validated local desired-state config, serves health
// endpoints, and keeps the gateway process alive until its context is canceled.
func Run(ctx context.Context, cfg config.Config, healthAddress string) error {
	if ctx == nil {
		return errors.New("context is required")
	}
	driver, err := platform.NewDriver(cfg.Tunnel.Interface)
	if err != nil {
		return err
	}
	return runWithDriver(ctx, cfg, healthAddress, driver)
}

// RunFile loads a private config file, applies it, and watches for complete
// replacements while keeping the gateway process alive.
func RunFile(ctx context.Context, configPath, healthAddress string) error {
	if ctx == nil {
		return errors.New("context is required")
	}
	cfg, digest, err := config.LoadFileWithDigest(configPath)
	if err != nil {
		return fmt.Errorf("load gateway config: %w", err)
	}
	slog.Debug("loaded gateway configuration",
		"interface", cfg.Tunnel.Interface,
		"listen_port", cfg.Tunnel.ListenPort,
		"peer_count", len(cfg.Tunnel.Peers),
		"route_count", len(cfg.Routes),
		"rule_count", len(cfg.Rules),
		"nat_count", len(cfg.NAT),
	)
	driver, err := platform.NewDriver(cfg.Tunnel.Interface)
	if err != nil {
		return err
	}
	return runWithDriverAndSource(ctx, cfg, healthAddress, driver, newFileConfigSource(configPath, digest))
}

func runWithDriver(ctx context.Context, cfg config.Config, healthAddress string, driver agent.Driver) error {
	return runWithDriverAndSource(ctx, cfg, healthAddress, driver, nil)
}

func runWithDriverAndSource(ctx context.Context, cfg config.Config, healthAddress string, driver agent.Driver, source *fileConfigSource) error {
	if ctx == nil {
		return errors.New("context is required")
	}
	waitForHubAuthentication := cfg.Hub != nil
	privateKey, err := wgtypes.ParseKey(cfg.Tunnel.PrivateKey)
	if err != nil {
		return fmt.Errorf("parse gateway WireGuard private key: %w", err)
	}
	slog.Info("gateway WireGuard public key", "public_key", privateKey.PublicKey().String())
	reconciler, err := agent.NewReconciler(driver)
	if err != nil {
		return err
	}
	if err := reconciler.Start(ctx); err != nil {
		return fmt.Errorf("block gateway forwarding on startup: %w", err)
	}
	if !waitForHubAuthentication {
		if err := reconciler.Apply(ctx, cfg); err != nil {
			return fmt.Errorf("apply gateway configuration: %w", err)
		}
	} else {
		slog.Info("Gateway forwarding remains blocked until HUB authenticates this node")
	}

	listener, err := net.Listen("tcp", healthAddress)
	if err != nil {
		_ = stopReconciler(reconciler)
		return fmt.Errorf("listen for gateway health checks: %w", err)
	}
	server := &http.Server{Handler: reconciler.HealthHandler(), ReadHeaderTimeout: 5 * time.Second}
	serveResult := make(chan error, 1)
	go func() {
		serveResult <- server.Serve(listener)
	}()
	slog.Info("gateway health listener started", "address", listener.Addr().String())
	controlManager := &control.Manager{}
	applySnapshot, onDisconnected := controlSnapshotHandlers(cfg, reconciler)
	if err := controlManager.Update(ctx, cfg, reconciler, applySnapshot, onDisconnected); err != nil {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancel()
		_ = server.Shutdown(shutdownCtx)
		_ = stopReconciler(reconciler)
		return fmt.Errorf("start Gateway HUB control client: %w", err)
	}
	clearRegistrationToken(&cfg)
	stopTunnelDiagnostics := startDebugStatusReporter(ctx, driver)
	stopDockerFirewallRefresher := startDockerFirewallRefresher(ctx, driver)

	var result error
	if source != nil {
		ticker := time.NewTicker(configReloadInterval)
		result = serveUntilStopped(ctx, serveResult, reconciler, source, ticker, controlManager, cfg, waitForHubAuthentication)
		ticker.Stop()
	} else {
		if waitForHubAuthentication {
			select {
			case <-controlManager.Ready():
				// HUB mode stays blocked until its desired configuration snapshot
				// has been validated and applied by the control client.
			case <-ctx.Done():
			case err := <-serveResult:
				if !errors.Is(err, http.ErrServerClosed) {
					result = fmt.Errorf("gateway health server stopped: %w", err)
				}
			}
		}
		if result == nil && ctx.Err() == nil {
			select {
			case <-ctx.Done():
			case err := <-serveResult:
				if !errors.Is(err, http.ErrServerClosed) {
					result = fmt.Errorf("gateway health server stopped: %w", err)
				}
			}
		}
	}
	stopDockerFirewallRefresher()
	stopTunnelDiagnostics()
	controlManager.Stop()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		result = errors.Join(result, fmt.Errorf("stop gateway health server: %w", err))
	}
	result = errors.Join(result, stopReconciler(reconciler))
	return result
}

type dockerFirewallRefresher interface {
	RefreshDockerFirewall(context.Context) error
}

func startDockerFirewallRefresher(ctx context.Context, driver agent.Driver) func() {
	refresher, ok := driver.(dockerFirewallRefresher)
	if !ok {
		return func() {}
	}
	refreshCtx, cancel := context.WithCancel(ctx)
	done := make(chan struct{})
	go func() {
		defer close(done)
		ticker := time.NewTicker(dockerFirewallRefreshInterval)
		defer ticker.Stop()
		for {
			select {
			case <-refreshCtx.Done():
				return
			case <-ticker.C:
				if err := refresher.RefreshDockerFirewall(refreshCtx); err != nil && !errors.Is(err, context.Canceled) {
					slog.Warn("failed to refresh Docker forwarding integration; will retry", "error", err)
				}
			}
		}
	}()
	return func() {
		cancel()
		<-done
	}
}

type debugStatusReporter interface {
	DebugStatus(context.Context)
}

func startDebugStatusReporter(ctx context.Context, driver agent.Driver) func() {
	reporter, ok := driver.(debugStatusReporter)
	if !ok || !slog.Default().Enabled(ctx, slog.LevelDebug) {
		return func() {}
	}
	statusCtx, cancel := context.WithCancel(ctx)
	done := make(chan struct{})
	go func() {
		defer close(done)
		reporter.DebugStatus(statusCtx)
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-statusCtx.Done():
				return
			case <-ticker.C:
				reporter.DebugStatus(statusCtx)
			}
		}
	}()
	return func() {
		cancel()
		<-done
	}
}

func serveUntilStopped(ctx context.Context, serveResult <-chan error, reconciler *agent.Reconciler, source *fileConfigSource, ticker *time.Ticker, controlManager *control.Manager, initialConfig config.Config, waitForHubAuthentication bool) error {
	retryApply := false
	currentConfig := initialConfig
	var ready <-chan struct{}
	if waitForHubAuthentication {
		ready = controlManager.Ready()
	}
	for {
		select {
		case <-ctx.Done():
			return nil
		case err := <-serveResult:
			if errors.Is(err, http.ErrServerClosed) {
				return nil
			}
			return fmt.Errorf("gateway health server stopped: %w", err)
		case <-ready:
			ready = nil
			latest, apply, changed, err := source.poll(false)
			if err != nil {
				if changed {
					if rejectErr := reconciler.RejectConfiguration(ctx, err); rejectErr != nil {
						slog.Error("gateway configuration rejected; forwarding remains blocked", "error", rejectErr)
					} else {
						slog.Error("gateway configuration rejected; forwarding remains blocked", "error", err)
					}
				}
				waitForHubAuthentication = false
				continue
			}
			if apply {
				currentConfig = *latest
				applySnapshot, onDisconnected := controlSnapshotHandlers(currentConfig, reconciler)
				if err := controlManager.Update(ctx, currentConfig, reconciler, applySnapshot, onDisconnected); err != nil {
					slog.Error("failed to update Gateway HUB control client before initial config apply", "error", err)
				}
				clearRegistrationToken(&currentConfig)
			}
			if currentConfig.Hub != nil {
				ready = controlManager.Ready()
				waitForHubAuthentication = !channelIsClosed(ready)
				if waitForHubAuthentication {
					continue
				}
				ready = nil
			} else {
				waitForHubAuthentication = false
			}
			if currentConfig.Hub == nil {
				if err := reconciler.Apply(ctx, currentConfig); err != nil {
					slog.Error("gateway configuration apply failed after config reload; forwarding remains blocked", "error", err)
					retryApply = true
				} else {
					retryApply = false
				}
			}
		case <-ticker.C:
			cfg, apply, changed, err := source.poll(retryApply)
			if err != nil {
				if changed {
					if rejectErr := reconciler.RejectConfiguration(ctx, err); rejectErr != nil {
						slog.Error("gateway configuration rejected; forwarding remains blocked", "error", rejectErr)
					} else {
						slog.Error("gateway configuration rejected; forwarding remains blocked", "error", err)
					}
					retryApply = false
				}
				continue
			}
			if !apply {
				continue
			}
			slog.Debug("gateway configuration reload detected", "changed", changed)
			currentConfig = *cfg
			applySnapshot, onDisconnected := controlSnapshotHandlers(currentConfig, reconciler)
			if err := controlManager.Update(ctx, currentConfig, reconciler, applySnapshot, onDisconnected); err != nil {
				slog.Error("failed to update Gateway HUB control client after config reload", "error", err)
			}
			clearRegistrationToken(&currentConfig)
			if currentConfig.Hub != nil {
				ready = controlManager.Ready()
				waitForHubAuthentication = !channelIsClosed(ready)
				if waitForHubAuthentication {
					continue
				}
				ready = nil
			} else {
				waitForHubAuthentication = false
				ready = nil
			}
			if currentConfig.Hub == nil {
				if err := reconciler.Apply(ctx, *cfg); err != nil {
					if changed {
						slog.Error("gateway configuration apply failed; forwarding remains blocked", "error", err)
					}
					retryApply = true
				} else {
					retryApply = false
				}
			}
		}
	}
}

func clearRegistrationToken(cfg *config.Config) {
	if cfg != nil && cfg.Hub != nil {
		cfg.Hub.RegistrationToken = ""
	}
}

func controlSnapshotHandlers(local config.Config, reconciler *agent.Reconciler) (control.SnapshotApplier, control.DisconnectHandler) {
	if local.Hub != nil {
		hubConfig := *local.Hub
		hubConfig.RegistrationToken = ""
		local.Hub = &hubConfig
	}
	apply := func(ctx context.Context, snapshot *hubv1.ConfigSnapshot) error {
		if snapshot == nil {
			return reconciler.RejectConfiguration(ctx, errors.New("HUB configuration snapshot is missing"))
		}
		desired, err := config.DecodeHubSnapshot(local, snapshot.GetPayload())
		if err != nil {
			return reconciler.RejectConfiguration(ctx, err)
		}
		logHubConfiguration(snapshot, desired)
		return reconciler.ApplyWithHash(ctx, desired, snapshot.GetConfigHash())
	}
	disconnect := func() error {
		ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancel()
		return reconciler.BlockAndClose(ctx)
	}
	return apply, disconnect
}

// logHubConfiguration records the effective configuration received from HUB
// without logging the local WireGuard private key or registration credential.
func logHubConfiguration(snapshot *hubv1.ConfigSnapshot, desired config.Config) {
	desired = desired.Canonical()
	slog.Info("received Gateway configuration from HUB",
		"task_id", snapshot.GetTaskId(),
		"config_hash", snapshot.GetConfigHash(),
		"tunnel_interface", desired.Tunnel.Interface,
		"tunnel_address", desired.Tunnel.Address,
		"listen_port", desired.Tunnel.ListenPort,
		"mtu", desired.Tunnel.EffectiveMTU(),
		"peer_count", len(desired.Tunnel.Peers),
		"route_count", len(desired.Routes),
		"nat_count", len(desired.NAT),
		"rule_count", len(desired.Rules),
	)

	for _, peer := range desired.Tunnel.Peers {
		slog.Debug("HUB Gateway configuration peer",
			"task_id", snapshot.GetTaskId(),
			"peer_id", peer.ID,
			"public_key_fingerprint", gatewayPublicKeyFingerprint(peer.PublicKey),
			"endpoint", peer.Endpoint,
			"allowed_ips", peer.AllowedIPs,
			"persistent_keepalive_seconds", peer.PersistentKeepalive,
		)
	}
	for _, route := range desired.Routes {
		slog.Debug("HUB Gateway configuration route", "task_id", snapshot.GetTaskId(), "route", route)
	}
	for _, mapping := range desired.NAT {
		slog.Debug("HUB Gateway configuration NAT mapping",
			"task_id", snapshot.GetTaskId(),
			"type", mapping.Type,
			"shadow_prefix", mapping.ShadowPrefix,
			"real_prefix", mapping.RealPrefix,
			"source_prefix", mapping.SourcePrefix,
			"destination_prefix", mapping.DestinationPrefix,
			"egress_interface", mapping.EgressInterface,
			"to_address", mapping.ToAddress,
		)
	}
	for _, rule := range desired.Rules {
		slog.Debug("HUB Gateway configuration ACL rule",
			"task_id", snapshot.GetTaskId(),
			"id", rule.ID,
			"priority", rule.Priority,
			"source_prefix", rule.SourcePrefix,
			"destination_prefix", rule.Destination,
			"protocol", rule.Protocol,
			"port_from", rule.PortFrom,
			"port_to", rule.PortTo,
			"action", rule.Action,
		)
	}
}

func gatewayPublicKeyFingerprint(encoded string) string {
	key, err := wgtypes.ParseKey(encoded)
	if err != nil {
		return "invalid"
	}
	digest := sha256.Sum256(key[:])
	return hex.EncodeToString(digest[:6])
}

func channelIsClosed(channel <-chan struct{}) bool {
	select {
	case <-channel:
		return true
	default:
		return false
	}
}

func stopReconciler(reconciler *agent.Reconciler) error {
	ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	if err := reconciler.Stop(ctx); err != nil {
		return fmt.Errorf("stop gateway dataplane: %w", err)
	}
	return nil
}
