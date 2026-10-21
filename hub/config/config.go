package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"strconv"
	"strings"
)

// Config contains process-level HUB settings. Management and HUB may use the
// same database pool when composed in one process, but retain separate service
// listeners and lifecycles.
type Config struct {
	InstanceID                        string         `json:"instance_id"`
	ListenAddr                        string         `json:"listen_addr"`
	TLS                               TLSConfig      `json:"tls"`
	Database                          DatabaseConfig `json:"database"`
	HeartbeatIntervalSeconds          uint32         `json:"heartbeat_interval_seconds"`
	HeartbeatTimeoutSeconds           uint32         `json:"heartbeat_timeout_seconds"`
	AuthenticationChallengeTTLSeconds uint32         `json:"authentication_challenge_ttl_seconds"`
}

// TLSConfig configures the server identity presented to connecting nodes.
type TLSConfig struct {
	Enabled  *bool  `json:"enabled,omitempty"`
	CertFile string `json:"cert_file"`
	KeyFile  string `json:"key_file"`
}

// IsEnabled reports whether HUB transport TLS is enabled. Legacy configs with
// certificate paths remain TLS-enabled; omitting the TLS block enables gRPC
// plaintext for deployments that explicitly choose it.
func (c TLSConfig) IsEnabled() bool {
	if c.Enabled != nil {
		return *c.Enabled
	}
	return strings.TrimSpace(c.CertFile) != "" || strings.TrimSpace(c.KeyFile) != ""
}

type DatabaseConfig struct {
	Driver string `json:"driver"`
	DSN    string `json:"dsn"`
}

// Load reads a strict JSON configuration file.
func Load(path string) (Config, error) {
	file, err := os.Open(path)
	if err != nil {
		return Config{}, fmt.Errorf("open HUB config: %w", err)
	}
	defer file.Close()

	decoder := json.NewDecoder(file)
	decoder.DisallowUnknownFields()

	var cfg Config
	if err := decoder.Decode(&cfg); err != nil {
		return Config{}, fmt.Errorf("decode HUB config: %w", err)
	}
	if err := decoder.Decode(new(any)); !errors.Is(err, io.EOF) {
		if err == nil {
			return Config{}, errors.New("decode HUB config: multiple JSON values")
		}
		return Config{}, fmt.Errorf("decode HUB config trailer: %w", err)
	}
	cfg.Database.Driver = strings.ToLower(strings.TrimSpace(cfg.Database.Driver))
	cfg.Database.DSN = os.ExpandEnv(cfg.Database.DSN)
	if cfg.HeartbeatIntervalSeconds == 0 {
		cfg.HeartbeatIntervalSeconds = 15
	}
	if cfg.HeartbeatTimeoutSeconds == 0 {
		cfg.HeartbeatTimeoutSeconds = 45
	}
	if cfg.AuthenticationChallengeTTLSeconds == 0 {
		cfg.AuthenticationChallengeTTLSeconds = 30
	}
	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

// Validate rejects incomplete settings before any network listener is opened.
func (c Config) Validate() error {
	instanceID := strings.TrimSpace(c.InstanceID)
	if instanceID == "" || len(instanceID) > 128 {
		return errors.New("HUB instance_id must contain 1 to 128 characters")
	}
	if strings.ContainsAny(instanceID, "\r\n\x00") {
		return errors.New("HUB instance_id contains invalid characters")
	}

	host, port, err := net.SplitHostPort(c.ListenAddr)
	if err != nil || host == "" && !strings.HasPrefix(c.ListenAddr, ":") {
		return fmt.Errorf("HUB listen_addr must be a host:port address: %q", c.ListenAddr)
	}
	portNumber, err := strconv.Atoi(port)
	if err != nil || portNumber < 1 || portNumber > 65535 {
		return fmt.Errorf("HUB listen_addr has an invalid port: %q", port)
	}
	if c.TLS.IsEnabled() && (strings.TrimSpace(c.TLS.CertFile) == "" || strings.TrimSpace(c.TLS.KeyFile) == "") {
		return errors.New("HUB TLS cert_file and key_file are required when tls.enabled is true")
	}
	if strings.ToLower(strings.TrimSpace(c.Database.Driver)) != "postgres" {
		return errors.New("HUB database.driver must be postgres")
	}
	if strings.TrimSpace(c.Database.DSN) == "" {
		return errors.New("HUB database.dsn is required; set it directly or through DATABASE_DSN")
	}
	if c.HeartbeatIntervalSeconds == 0 || c.HeartbeatTimeoutSeconds <= c.HeartbeatIntervalSeconds || c.AuthenticationChallengeTTLSeconds == 0 {
		return errors.New("HUB heartbeat timeout must exceed the positive heartbeat interval, and authentication challenge TTL must be positive")
	}
	return nil
}
