// Package app is the HUB composition root. Keeping construction here allows
// cmd/hub and a future combined Management process to share the same service
// implementation while respecting Go internal-package boundaries.
package app

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/lib/pq"
	"velotrust/hub/config"
	"velotrust/hub/internal/auth"
	"velotrust/hub/internal/repository/postgres"
	"velotrust/hub/internal/session"
	hubruntime "velotrust/hub/runtime"
	hubgrpc "velotrust/hub/transport/grpc"
	"velotrust/shared/repository/database"
)

// Build creates the HUB listener and its PostgreSQL-backed Gateway identity
// verifier. The returned cleanup function closes the shared database pool.
func Build(cfg config.Config) (*hubruntime.Server, func() error, error) {
	if err := cfg.Validate(); err != nil {
		return nil, nil, err
	}
	db, err := database.NewDatabase(cfg.Database.Driver, cfg.Database.DSN)
	if err != nil {
		return nil, nil, fmt.Errorf("initialize HUB database: %w", err)
	}
	cleanup := db.Close
	if err := db.Ping(context.Background()); err != nil {
		_ = cleanup()
		return nil, nil, fmt.Errorf("verify HUB database: %w", err)
	}
	directory, err := postgres.NewGatewayDirectory(db)
	if err != nil {
		_ = cleanup()
		return nil, nil, fmt.Errorf("initialize HUB Gateway directory: %w", err)
	}
	clientDirectory, err := postgres.NewClientDirectory(db)
	if err != nil {
		_ = cleanup()
		return nil, nil, fmt.Errorf("initialize HUB Client directory: %w", err)
	}
	gatewayAuthenticator, err := auth.NewGatewayAuthenticator(directory, time.Duration(cfg.AuthenticationChallengeTTLSeconds)*time.Second)
	if err != nil {
		_ = cleanup()
		return nil, nil, fmt.Errorf("initialize HUB Gateway authentication: %w", err)
	}
	clientAuthenticator, err := auth.NewClientAuthenticator(clientDirectory, time.Duration(cfg.AuthenticationChallengeTTLSeconds)*time.Second)
	if err != nil {
		_ = cleanup()
		return nil, nil, fmt.Errorf("initialize HUB Client authentication: %w", err)
	}
	runtimeRepository, err := postgres.NewRuntimeRepository(db, cfg.InstanceID)
	if err != nil {
		_ = cleanup()
		return nil, nil, fmt.Errorf("initialize HUB runtime repository: %w", err)
	}
	registrationRepository, err := postgres.NewGatewayRegistrationRepository(db, cfg.InstanceID)
	if err != nil {
		_ = cleanup()
		return nil, nil, fmt.Errorf("initialize HUB Gateway registration repository: %w", err)
	}
	clientRegistrationRepository, err := postgres.NewClientRegistrationRepository(db, cfg.InstanceID)
	if err != nil {
		_ = cleanup()
		return nil, nil, fmt.Errorf("initialize HUB Client registration repository: %w", err)
	}
	configurationSource, err := postgres.NewGatewayConfigurationSource(db, cfg.InstanceID)
	if err != nil {
		_ = cleanup()
		return nil, nil, fmt.Errorf("initialize HUB Gateway configuration source: %w", err)
	}
	accessChanges, err := postgres.NewAccessChangeRepository(db, cfg.InstanceID)
	if err != nil {
		_ = cleanup()
		return nil, nil, fmt.Errorf("initialize HUB access-change repository: %w", err)
	}
	controlHandler, err := hubgrpc.NewControlHandler(
		cfg.InstanceID,
		gatewayAuthenticator,
		registrationRepository,
		session.NewMemoryStore(),
		runtimeRepository,
		runtimeRepository,
		configurationSource,
		time.Duration(cfg.AuthenticationChallengeTTLSeconds)*time.Second,
		time.Duration(cfg.HeartbeatIntervalSeconds)*time.Second,
		time.Duration(cfg.HeartbeatTimeoutSeconds)*time.Second,
		!cfg.TLS.IsEnabled(),
		hubgrpc.ClientControlDependencies{Authenticator: clientAuthenticator, Registrations: clientRegistrationRepository},
	)
	if err != nil {
		_ = cleanup()
		return nil, nil, fmt.Errorf("initialize HUB control stream: %w", err)
	}
	server, err := hubruntime.New(cfg, controlHandler)
	if err != nil {
		_ = cleanup()
		return nil, nil, err
	}
	workerCtx, stopWorker := context.WithCancel(context.Background())
	workerDone := make(chan struct{})
	go func() {
		defer close(workerDone)
		runAccessChangeWorker(workerCtx, cfg.Database.DSN, accessChanges, controlHandler)
	}()
	cleanup = func() error {
		stopWorker()
		<-workerDone
		return db.Close()
	}
	return server, cleanup, nil
}

func runAccessChangeWorker(ctx context.Context, dsn string, repository *postgres.AccessChangeRepository, control *hubgrpc.ControlHandler) {
	listener := pq.NewListener(dsn, time.Second, 30*time.Second, func(event pq.ListenerEventType, err error) {
		if err != nil {
			slog.Warn("HUB access-change listener state changed", "event", event, "error", err)
		}
	})
	defer listener.Close()
	if err := listener.Listen("velotrust_access_change"); err != nil {
		slog.Warn("HUB access-change LISTEN unavailable; polling outbox", "error", err)
	}
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	notifications := listener.Notify
	dispatch := func() {
		processCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		if err := repository.DispatchPending(processCtx, control.NotifyConfigurationChange); err != nil {
			slog.Warn("dispatch HUB access-change events", "error", err)
		}
	}
	dispatch()
	for {
		select {
		case <-ctx.Done():
			return
		case notification, ok := <-notifications:
			if !ok {
				notifications = nil
			} else if notification != nil {
				dispatch()
			}
		case <-ticker.C:
			dispatch()
		}
	}
}
