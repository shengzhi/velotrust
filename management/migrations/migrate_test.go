package migrations

import (
	"strings"
	"testing"
)

func TestPostgreSQLMigrationsAreVersionedAndRunnerOwned(t *testing.T) {
	initial, err := migrationFiles.ReadFile("postgresql/001_iam_initial.sql")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(initial), "CREATE TABLE public.t_iam_user (") {
		t.Fatal("initial migration does not create IAM schema")
	}
	if strings.Contains(string(initial), "BEGIN;") || strings.Contains(string(initial), "COMMIT;") {
		t.Fatal("migration must leave transaction ownership to the runner")
	}
	replay, err := migrationFiles.ReadFile("postgresql/002_iam_totp_replay_protection.sql")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(replay), "ADD COLUMN IF NOT EXISTS last_used_step") {
		t.Fatal("TOTP replay protection migration is missing")
	}
}
