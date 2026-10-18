package app

import (
	"context"
	"fmt"
	"strings"

	"velotrust/management/migrations"
	"velotrust/shared/repository/database"
)

// Migrate applies versioned Management schema changes. Existing hand-created
// IAM schemas must be explicitly baselined after operator verification.
func Migrate(ctx context.Context, conf *Config, baselineExistingIAM bool) ([]string, error) {
	if conf == nil || conf.Database.DSN == "" {
		return nil, fmt.Errorf("database configuration is required")
	}
	driver := strings.ToLower(strings.TrimSpace(conf.Database.Driver))
	if driver != "postgres" && driver != "postgresql" && driver != "pgx" {
		return nil, fmt.Errorf("Management migrations require PostgreSQL, got %q", conf.Database.Driver)
	}
	db, err := database.NewDatabase("postgres", conf.Database.DSN)
	if err != nil {
		return nil, fmt.Errorf("initialize migration database: %w", err)
	}
	defer db.Close()
	if err := db.Ping(ctx); err != nil {
		return nil, fmt.Errorf("verify migration database: %w", err)
	}
	return migrations.Apply(ctx, db, baselineExistingIAM)
}
