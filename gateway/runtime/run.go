package runtime

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"time"

	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
	"velotrust/gateway/config"
	"velotrust/gateway/internal/agent"
	"velotrust/gateway/platform"
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
		"gateway_id", cfg.GatewayID,
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
	privateKey, err := wgtypes.ParseKey(cfg.Tunnel.PrivateKey)
	if err != nil {
		return fmt.Errorf("parse gateway WireGuard private key: %w", err)
	}
	slog.Info("gateway WireGuard public key", "gateway_id", cfg.GatewayID, "public_key", privateKey.PublicKey().String())
	reconciler, err := agent.NewReconciler(driver)
	if err != nil {
		return err
	}
	if err := reconciler.Start(ctx); err != nil {
		return fmt.Errorf("block gateway forwarding on startup: %w", err)
	}
	if err := reconciler.Apply(ctx, cfg); err != nil {
		return fmt.Errorf("apply gateway configuration: %w", err)
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
	stopTunnelDiagnostics := startDebugStatusReporter(ctx, driver)
	stopDockerFirewallRefresher := startDockerFirewallRefresher(ctx, driver)

	var result error
	if source != nil {
		ticker := time.NewTicker(configReloadInterval)
		result = serveUntilStopped(ctx, serveResult, reconciler, source, ticker)
		ticker.Stop()
	} else {
		select {
		case <-ctx.Done():
		case err := <-serveResult:
			if !errors.Is(err, http.ErrServerClosed) {
				result = fmt.Errorf("gateway health server stopped: %w", err)
			}
		}
	}
	stopDockerFirewallRefresher()
	stopTunnelDiagnostics()

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

func serveUntilStopped(ctx context.Context, serveResult <-chan error, reconciler *agent.Reconciler, source *fileConfigSource, ticker *time.Ticker) error {
	retryApply := false
	for {
		select {
		case <-ctx.Done():
			return nil
		case err := <-serveResult:
			if errors.Is(err, http.ErrServerClosed) {
				return nil
			}
			return fmt.Errorf("gateway health server stopped: %w", err)
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

func stopReconciler(reconciler *agent.Reconciler) error {
	ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	if err := reconciler.Stop(ctx); err != nil {
		return fmt.Errorf("stop gateway dataplane: %w", err)
	}
	return nil
}
