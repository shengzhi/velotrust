package identity

import (
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGenerateCreatesOneWireGuardKeyWithoutOverwriting(t *testing.T) {
	directory := filepath.Join(t.TempDir(), "keys")
	public, err := Generate(directory)
	if err != nil {
		t.Fatal(err)
	}
	if public.WireGuard == "" {
		t.Fatalf("generated public key summary is empty: %+v", public)
	}
	info, err := os.Stat(directory)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o700 {
		t.Fatalf("key directory permissions = %04o, want 0700", info.Mode().Perm())
	}

	privateText, err := os.ReadFile(filepath.Join(directory, "wireguard.key"))
	if err != nil {
		t.Fatal(err)
	}
	privateKey, err := wgtypes.ParseKey(strings.TrimSpace(string(privateText)))
	if err != nil {
		t.Fatalf("parse generated WireGuard private key: %v", err)
	}
	if got := privateKey.PublicKey().String(); got != public.WireGuard {
		t.Fatalf("public key = %q, want derived key %q", public.WireGuard, got)
	}
	for _, name := range []string{"wireguard.key", "wireguard.pub"} {
		info, err := os.Stat(filepath.Join(directory, name))
		if err != nil {
			t.Fatal(err)
		}
		wantMode := os.FileMode(0o644)
		if name == "wireguard.key" {
			wantMode = 0o600
		}
		if info.Mode().Perm() != wantMode {
			t.Fatalf("%s permissions = %04o, want %04o", name, info.Mode().Perm(), wantMode)
		}
	}
	for _, name := range []string{"node-identity.key", "node-identity.pub"} {
		if _, err := os.Stat(filepath.Join(directory, name)); !os.IsNotExist(err) {
			t.Fatalf("unexpected legacy key file %s; stat error = %v", name, err)
		}
	}

	originalWireGuard, err := os.ReadFile(filepath.Join(directory, "wireguard.key"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Generate(directory); err == nil {
		t.Fatal("Generate() overwrote existing key files")
	}
	currentWireGuard, err := os.ReadFile(filepath.Join(directory, "wireguard.key"))
	if err != nil {
		t.Fatal(err)
	}
	if string(currentWireGuard) != string(originalWireGuard) {
		t.Fatal("existing WireGuard private key changed after refused overwrite")
	}
}

func TestGenerateRejectsSharedDirectoryPermissions(t *testing.T) {
	directory := t.TempDir()
	if err := os.Chmod(directory, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := Generate(directory); err == nil {
		t.Fatal("Generate() accepted a directory accessible to group or others")
	}
}
