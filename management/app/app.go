// Package app exposes the Management composition root to executable commands.
package app

import (
	"context"
	"fmt"
	"strings"
	"time"

	"velotrust/management/internal/app/webapi"
	"velotrust/management/internal/config"
	appIAMRepo "velotrust/management/internal/repository/iam"
	appIAMService "velotrust/management/internal/service/iam"
	management "velotrust/management/runtime"
	"velotrust/shared/repository/database"
)

type Config = config.Config

func LoadConfig(path string) (*Config, error) { return config.LoadConfig(path) }

func Build(conf *Config, listenAddr, version string) (*management.Server, *database.Database, func() error, error) {
	return webapi.Build(conf, listenAddr, version)
}

// BootstrapAdmin creates the first platform administrator exactly once. The
// caller must obtain the initial password from a protected secret source.
func BootstrapAdmin(ctx context.Context, conf *Config, username, displayName, password string) error {
	if conf == nil || conf.Database.Driver == "" || conf.Database.DSN == "" {
		return fmt.Errorf("database configuration is required")
	}
	driver := strings.ToLower(strings.TrimSpace(conf.Database.Driver))
	if driver != "postgres" && driver != "postgresql" && driver != "pgx" {
		return fmt.Errorf("IAM currently requires PostgreSQL; unsupported database driver %q", conf.Database.Driver)
	}
	db, err := database.NewDatabase("postgres", conf.Database.DSN)
	if err != nil {
		return fmt.Errorf("initialize bootstrap database: %w", err)
	}
	defer db.Close()
	if err := db.Ping(ctx); err != nil {
		return fmt.Errorf("verify bootstrap database: %w", err)
	}
	store, err := appIAMRepo.New(db)
	if err != nil {
		return err
	}
	service, err := appIAMService.New(store, appIAMService.Options{
		SessionIdleTTL:     time.Duration(conf.Auth.Session.IdleTTLMinutes) * time.Minute,
		SessionAbsoluteTTL: time.Duration(conf.Auth.Session.AbsoluteTTLMinutes) * time.Minute,
		JWTSigningKey:      []byte(conf.Auth.JWTSigningKey),
		ThrottleHMACKey:    []byte(conf.Auth.ThrottleHMACKey),
		TOTPEncryptionKey:  []byte(conf.Auth.TOTPEncryptionKey),
		TOTPKeyVersion:     conf.Auth.TOTPKeyVersion,
	})
	if err != nil {
		return err
	}
	return service.BootstrapAdmin(ctx, username, displayName, password)
}
