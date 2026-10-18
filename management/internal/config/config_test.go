package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/viper"
)

func TestLoadConfigMissingFileUsesEnv(t *testing.T) {
	viper.Reset()
	t.Setenv("APP_APP_NAME", "template-app")
	t.Setenv("APP_APP_VERSION", "1.2.3")

	cfg, err := LoadConfig(filepath.Join(t.TempDir(), "missing.toml"))
	if err != nil {
		t.Fatalf("LoadConfig returned error: %v", err)
	}

	if cfg.App.Name != "template-app" {
		t.Fatalf("expected app.name from env, got %q", cfg.App.Name)
	}
	if cfg.App.Version != "1.2.3" {
		t.Fatalf("expected app.version from env, got %q", cfg.App.Version)
	}
}

func TestLoadConfigEnvOverridesFile(t *testing.T) {
	viper.Reset()
	t.Setenv("APP_APP_NAME", "template-app")
	t.Setenv("APP_APP_VERSION", "1.2.3")

	dir := t.TempDir()
	confPath := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(confPath, []byte(`[app]
name = "file-app"
version = "9.9.9"
`), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := LoadConfig(confPath)
	if err != nil {
		t.Fatalf("LoadConfig returned error: %v", err)
	}

	if cfg.App.Name != "template-app" {
		t.Fatalf("expected app.name from env, got %q", cfg.App.Name)
	}
	if cfg.App.Version != "1.2.3" {
		t.Fatalf("expected app.version from env, got %q", cfg.App.Version)
	}
}

func TestLoadConfigCORS(t *testing.T) {
	viper.Reset()
	dir := t.TempDir()
	confPath := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(confPath, []byte(`[http]
port = 8080

[http.cors]
allowed_origins = ["*"]
allowed_methods = ["POST", "OPTIONS"]
allowed_headers = ["Content-Type", "Authorization"]
max_age_seconds = 600

[database]
driver = "sqlite"
dsn = "./template.db"
show_sql = true
`), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := LoadConfig(confPath)
	if err != nil {
		t.Fatalf("LoadConfig returned error: %v", err)
	}
	if cfg.HTTP.Port != 8080 || len(cfg.HTTP.CORS.AllowedOrigins) != 1 || cfg.HTTP.CORS.AllowedOrigins[0] != "*" || cfg.HTTP.CORS.MaxAgeSeconds != 600 {
		t.Fatalf("unexpected HTTP config: %+v", cfg.HTTP)
	}
	if cfg.Database.Driver != "sqlite" || cfg.Database.DSN != "./template.db" || !cfg.Database.ShowSQL {
		t.Fatalf("unexpected database config: %+v", cfg.Database)
	}
}
