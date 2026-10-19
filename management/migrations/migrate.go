// Package migrations applies the versioned PostgreSQL schema used by Management.
package migrations

import (
	"context"
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"path"
	"sort"
	"strings"

	"velotrust/shared/repository/database"
	"xorm.io/xorm"
)

//go:embed postgresql/*.sql
var migrationFiles embed.FS

const (
	migrationTable = "velotrust_schema_migrations"
	initialSchema  = "001_iam_initial.sql"
)

type record struct {
	Checksum string `xorm:"'checksum'"`
}

// Apply applies every unapplied PostgreSQL migration. Set baselineExistingIAM
// only after verifying that an existing database already matches the IAM schema
// snapshot; this records the initial schema without recreating its tables.
func Apply(ctx context.Context, db *database.Database, baselineExistingIAM bool) ([]string, error) {
	if db == nil || db.Engine() == nil || db.Driver() != "postgres" {
		return nil, errors.New("PostgreSQL database is required for Management migrations")
	}
	entries, err := fs.ReadDir(migrationFiles, "postgresql")
	if err != nil {
		return nil, err
	}
	var names []string
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".sql") {
			names = append(names, entry.Name())
		}
	}
	sort.Strings(names)
	var applied []string
	for _, name := range names {
		contents, err := migrationFiles.ReadFile(path.Join("postgresql", name))
		if err != nil {
			return applied, err
		}
		sum := sha256.Sum256(contents)
		checksum := hex.EncodeToString(sum[:])
		baseline := false
		newlyApplied := false
		err = db.WithTransaction(ctx, func(s *xorm.Session) error {
			if _, err := s.Exec("SELECT pg_advisory_xact_lock(?)", int64(0x56454c4f54525553)); err != nil {
				return fmt.Errorf("lock migration runner: %w", err)
			}
			if _, err := s.Exec("CREATE TABLE IF NOT EXISTS " + migrationTable + " (version VARCHAR(128) PRIMARY KEY, checksum VARCHAR(64) NOT NULL, applied_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP)"); err != nil {
				return fmt.Errorf("initialize migration history: %w", err)
			}
			var prior record
			found, err := s.Table(migrationTable).Where("version=?", name).Get(&prior)
			if err != nil {
				return err
			}
			if found {
				if prior.Checksum != "baseline" && prior.Checksum != checksum {
					return fmt.Errorf("migration %s checksum differs from applied version", name)
				}
				return nil
			}
			if name == initialSchema {
				var existing struct {
					Present bool `xorm:"'present'"`
				}
				if _, err := s.SQL("SELECT to_regclass('public.t_iam_user') IS NOT NULL AS present").Get(&existing); err != nil {
					return err
				}
				if existing.Present {
					if !baselineExistingIAM {
						return errors.New("IAM tables already exist without migration history; verify the schema and rerun with --baseline-existing-iam")
					}
					baseline = true
				} else {
					baselineExistingIAM = false
				}
			}
			if !baseline {
				if _, err := s.Exec(migrationSQL(contents)); err != nil {
					return fmt.Errorf("execute migration %s: %w", name, err)
				}
			}
			storedChecksum := checksum
			if baseline {
				storedChecksum = "baseline"
			}
			if _, err := s.Exec("INSERT INTO "+migrationTable+" (version, checksum) VALUES (?, ?)", name, storedChecksum); err != nil {
				return fmt.Errorf("record migration %s: %w", name, err)
			}
			newlyApplied = true
			return nil
		})
		if err != nil {
			return applied, err
		}
		if newlyApplied {
			applied = append(applied, name)
		}
	}
	return applied, nil
}

// migrationSQL removes transaction wrappers left in older migration files.
// Apply already runs each migration and its history record in one transaction;
// executing an inner COMMIT would end that transaction before the runner can
// record the migration.
func migrationSQL(contents []byte) string {
	lines := strings.Split(string(contents), "\n")
	firstSQL, lastSQL := -1, -1
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "--") {
			continue
		}
		if firstSQL == -1 {
			firstSQL = i
		}
		lastSQL = i
	}
	if firstSQL >= 0 && strings.EqualFold(strings.TrimSpace(lines[firstSQL]), "BEGIN;") {
		lines[firstSQL] = ""
	}
	if lastSQL >= 0 && strings.EqualFold(strings.TrimSpace(lines[lastSQL]), "COMMIT;") {
		lines[lastSQL] = ""
	}
	return strings.Join(lines, "\n")
}
