package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"velotrust/gateway/identity"
)

const privateKeyForTest = "AQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQE="

func TestValidateCommandOmitsPrivateKey(t *testing.T) {
	configData := `{"schema_version":1,"gateway_id":"gw-test","generation":3,"tunnel":{"interface":"vtrust0","address":"10.100.0.1/24","listen_port":51820,"private_key":"` + privateKeyForTest + `"}}`
	path := filepath.Join(t.TempDir(), "gateway.json")
	if err := os.WriteFile(path, []byte(configData), 0o600); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	code := run([]string{"validate", "--config", path}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run() = %d, stderr=%s", code, stderr.String())
	}
	if strings.Contains(stdout.String(), privateKeyForTest) || strings.Contains(stderr.String(), privateKeyForTest) {
		t.Fatal("validation command exposed private key material")
	}
	if !strings.Contains(stdout.String(), `"default_policy":"default-deny"`) {
		t.Fatalf("validation result omitted default-deny status: %s", stdout.String())
	}
}

func TestValidateCommandRejectsMissingConfig(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := run([]string{"validate"}, &stdout, &stderr); code != 2 {
		t.Fatalf("run() = %d, want 2", code)
	}
}

func TestRunCommandRequiresConfig(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := run([]string{"run"}, &stdout, &stderr); code != 2 {
		t.Fatalf("run() = %d, want 2", code)
	}
	if !strings.Contains(stderr.String(), "run requires --config") {
		t.Fatalf("missing config error = %q", stderr.String())
	}
}

func TestKeygenCommandPrintsOnlyPublicKeys(t *testing.T) {
	directory := filepath.Join(t.TempDir(), "keys")
	var stdout, stderr bytes.Buffer
	if code := run([]string{"keygen", "--dir", directory}, &stdout, &stderr); code != 0 {
		t.Fatalf("run() = %d, stderr=%s", code, stderr.String())
	}
	var public identity.PublicKey
	if err := json.Unmarshal(stdout.Bytes(), &public); err != nil {
		t.Fatalf("keygen output is not public-key JSON: %v", err)
	}
	if public.WireGuard == "" {
		t.Fatalf("keygen output omitted the WireGuard public key: %+v", public)
	}
	for _, name := range []string{"wireguard.key"} {
		data, err := os.ReadFile(filepath.Join(directory, name))
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(stdout.String(), strings.TrimSpace(string(data))) {
			t.Fatalf("keygen output exposed %s", name)
		}
		info, err := os.Stat(filepath.Join(directory, name))
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode().Perm() != 0o600 {
			t.Fatalf("%s permissions = %04o, want 0600", name, info.Mode().Perm())
		}
	}
	for _, name := range []string{"node-identity.key", "node-identity.pub"} {
		if _, err := os.Stat(filepath.Join(directory, name)); !os.IsNotExist(err) {
			t.Fatalf("keygen created unexpected legacy key file %s; stat error = %v", name, err)
		}
	}
}
