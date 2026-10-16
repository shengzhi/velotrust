package config

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func validConfig() Config {
	return Config{
		SchemaVersion: 1,
		GatewayID:     "gw-west-1",
		Tunnel: Tunnel{
			Interface:  "vtrust0",
			Address:    "10.100.0.1/24",
			ListenPort: 51820,
			PrivateKey: testKey(1),
			Peers: []Peer{{
				ID:         "site-east",
				PublicKey:  testKey(2),
				Endpoint:   "203.0.113.2:51820",
				AllowedIPs: []string{"10.110.0.0/16", "192.0.2.0/24", "198.51.100.0/24"},
			}},
		},
		Routes: []string{"192.0.2.7/24"},
		NAT:    []NATMapping{{ShadowPrefix: "100.64.0.0/24", RealPrefix: "10.20.0.0/24"}},
		Rules: []ACLRule{{
			ID:           "allow-app",
			SourcePrefix: "10.110.0.0/16",
			Destination:  "10.20.0.0/24",
			Protocol:     "tcp",
			PortFrom:     443,
			PortTo:       443,
			Action:       "allow",
		}},
	}
}

func testKey(value byte) string {
	return base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{value}, 32))
}

func TestConfigValidateAndCanonicalize(t *testing.T) {
	cfg := validConfig()
	cfg.Routes = []string{"198.51.100.7/24", "192.0.2.7/24"}
	if err := cfg.Validate(); err != nil {
		t.Fatal(err)
	}
	canonical := cfg.Canonical()
	if got, want := strings.Join(canonical.Routes, ","), "192.0.2.0/24,198.51.100.0/24"; got != want {
		t.Fatalf("canonical routes = %q, want %q", got, want)
	}
	if got, want := canonical.Tunnel.Peers[0].AllowedIPs[0], "10.110.0.0/16"; got != want {
		t.Fatalf("canonical peer prefix = %q, want %q", got, want)
	}
	canonical.Routes[0] = "203.0.113.0/24"
	if cfg.Routes[0] == canonical.Routes[0] {
		t.Fatal("Canonical returned slices that alias the original config")
	}
}

func TestValidateFailsClosedOnMalformedOrAmbiguousConfig(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*Config)
	}{
		{name: "missing key", mutate: func(c *Config) { c.Tunnel.PrivateKey = "" }},
		{name: "unsafe interface", mutate: func(c *Config) { c.Tunnel.Interface = "wg0;touch" }},
		{name: "zero-length tunnel prefix", mutate: func(c *Config) { c.Tunnel.Address = "10.100.0.1/0" }},
		{name: "default route without opt-in", mutate: func(c *Config) { c.Routes = []string{"0.0.0.0/0"} }},
		{name: "duplicate peer key", mutate: func(c *Config) { c.Tunnel.Peers = append(c.Tunnel.Peers, c.Tunnel.Peers[0]) }},
		{name: "overlapping NAT shadow", mutate: func(c *Config) {
			c.NAT = append(c.NAT, NATMapping{ShadowPrefix: "100.64.0.0/25", RealPrefix: "10.30.0.0/25"})
		}},
		{name: "invalid ACL port interval", mutate: func(c *Config) { c.Rules[0].PortFrom, c.Rules[0].PortTo = 5000, 1000 }},
		{name: "unknown ACL action", mutate: func(c *Config) { c.Rules[0].Action = "permit" }},
		{name: "duplicate ACL priority", mutate: func(c *Config) {
			c.Rules = append(c.Rules, ACLRule{ID: "deny-other", SourcePrefix: "10.0.0.0/8", Destination: "10.20.0.0/24", Protocol: "any", Action: "deny"})
		}},
		{name: "mixed ACL address families", mutate: func(c *Config) { c.Rules[0].Destination = "2001:db8::/32" }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cfg := validConfig()
			test.mutate(&cfg)
			if err := cfg.Validate(); err == nil {
				t.Fatal("Validate() accepted invalid config")
			}
		})
	}
}

func TestDecodeIsStrictAndSingleDocument(t *testing.T) {
	cfg := validConfig()
	data, err := json.Marshal(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Decode(bytes.NewReader(data)); err != nil {
		t.Fatal(err)
	}

	unknownField := bytes.Replace(data, []byte(`"schema_version"`), []byte(`"unknown":true,"schema_version"`), 1)
	if _, err := Decode(bytes.NewReader(unknownField)); err == nil {
		t.Fatal("Decode() accepted an unknown field")
	}
	duplicateField := bytes.Replace(data, []byte(`"gateway_id":"gw-west-1"`), []byte(`"gateway_id":"gw-west-1","gateway_id":"gw-west-2"`), 1)
	if _, err := Decode(bytes.NewReader(duplicateField)); err == nil {
		t.Fatal("Decode() accepted a duplicate field")
	}
	if _, err := Decode(strings.NewReader(string(data) + "\n{}")); err == nil {
		t.Fatal("Decode() accepted multiple JSON values")
	}
}

func TestLoadFileRequiresPrivateRegularFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "gateway.json")
	data, err := json.Marshal(validConfig())
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadFile(path); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, 0o640); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadFile(path); err == nil {
		t.Fatal("LoadFile() accepted group-readable key material")
	}
	link := filepath.Join(dir, "gateway-link.json")
	if err := os.Symlink(path, link); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadFile(link); err == nil {
		t.Fatal("LoadFile() accepted a symlink")
	}
}

func TestLoadFileWithDigestTracksExactContentsIncludingInvalidJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "gateway.json")
	data, err := json.Marshal(validConfig())
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, digest, err := LoadFileWithDigest(path); err != nil {
		t.Fatal(err)
	} else if digest != sha256.Sum256(data) {
		t.Fatal("LoadFileWithDigest() returned a fingerprint that differs from the file contents")
	}
	invalid := []byte("not json")
	if err := os.WriteFile(path, invalid, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, digest, err := LoadFileWithDigest(path); err == nil {
		t.Fatal("LoadFileWithDigest() accepted invalid JSON")
	} else if digest != sha256.Sum256(invalid) {
		t.Fatal("invalid file did not return a fingerprint")
	}
}

func TestLoadFileResolvesSeparatePrivateKeyFromSameDirectory(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "gateway.json")
	keyPath := filepath.Join(dir, "wireguard.key")
	cfg := validConfig()
	privateKey := cfg.Tunnel.PrivateKey
	cfg.Tunnel.PrivateKey = ""
	cfg.Tunnel.PrivateKeyFile = "wireguard.key"
	data, err := json.Marshal(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(configPath, data, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(keyPath, []byte(privateKey+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	loaded, digest, err := LoadFileWithDigest(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Tunnel.PrivateKey != privateKey || loaded.Tunnel.PrivateKeyFile != "" {
		t.Fatalf("loaded key reference was not resolved: private_key=%q private_key_file=%q", loaded.Tunnel.PrivateKey, loaded.Tunnel.PrivateKeyFile)
	}
	if digest == sha256.Sum256(data) {
		t.Fatal("fingerprint did not include the referenced private key contents")
	}
	if _, err := Decode(bytes.NewReader(data)); err == nil {
		t.Fatal("Decode() accepted a local key-file reference without secure file loading")
	}
}

func TestLoadFileRejectsUnsafeReferencedPrivateKey(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "gateway.json")
	cfg := validConfig()
	cfg.Tunnel.PrivateKey = ""
	cfg.Tunnel.PrivateKeyFile = "../outside.key"
	data, err := json.Marshal(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(configPath, data, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadFile(configPath); err == nil {
		t.Fatal("LoadFile() accepted a path outside the config directory")
	}

	cfg.Tunnel.PrivateKeyFile = "wireguard.key"
	data, err = json.Marshal(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(configPath, data, 0o600); err != nil {
		t.Fatal(err)
	}
	keyPath := filepath.Join(dir, "wireguard.key")
	if err := os.WriteFile(keyPath, []byte(validConfig().Tunnel.PrivateKey), 0o640); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadFile(configPath); err == nil {
		t.Fatal("LoadFile() accepted a group-readable private key file")
	}
}
