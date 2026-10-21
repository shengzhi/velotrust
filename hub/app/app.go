// Package app is the HUB composition root. Keeping construction here allows
// cmd/hub and a future combined Management process to share the same service
// implementation while respecting Go internal-package boundaries.
package app

import (
	"context"
	"fmt"
	"time"

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
	gatewayAuthenticator, err := auth.NewGatewayAuthenticator(directory, time.Duration(cfg.AuthenticationChallengeTTLSeconds)*time.Second)
	if err != nil {
		_ = cleanup()
		return nil, nil, fmt.Errorf("initialize HUB Gateway authentication: %w", err)
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
	configurationSource, err := postgres.NewGatewayConfigurationSource(db, cfg.InstanceID)
	if err != nil {
		_ = cleanup()
		return nil, nil, fmt.Errorf("initialize HUB Gateway configuration source: %w", err)
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
	return server, cleanup, nil
}
