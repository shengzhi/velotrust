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
	InstanceID string    `json:"instance_id"`
	ListenAddr string    `json:"listen_addr"`
	TLS        TLSConfig `json:"tls"`
}

// TLSConfig configures the server identity presented to connecting nodes.
type TLSConfig struct {
	CertFile string `json:"cert_file"`
	KeyFile  string `json:"key_file"`
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
	if strings.TrimSpace(c.TLS.CertFile) == "" || strings.TrimSpace(c.TLS.KeyFile) == "" {
		return errors.New("HUB TLS cert_file and key_file are required")
	}
	return nil
}
