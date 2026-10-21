package runtime

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"velotrust/gateway/config"
)

type lifecycleDriver struct{ closed bool }

func (*lifecycleDriver) BlockForwarding(context.Context) error                 { return nil }
func (*lifecycleDriver) ApplyTunnel(context.Context, config.Tunnel) error      { return nil }
func (*lifecycleDriver) ApplyFirewall(context.Context, []config.ACLRule) error { return nil }
func (*lifecycleDriver) ApplyPeerRoutes(context.Context, []config.Peer) error  { return nil }
func (*lifecycleDriver) ApplyRoutes(context.Context, []string) error           { return nil }
func (*lifecycleDriver) ApplyNAT(context.Context, []config.NATMapping) error   { return nil }
func (*lifecycleDriver) EnableForwarding(context.Context) error                { return nil }
func (*lifecycleDriver) UnblockForwarding(context.Context) error               { return nil }
func (d *lifecycleDriver) Close(context.Context) error {
	d.closed = true
	return nil
}

func TestRunWithDriverCleansUpWhenHealthListenerCannotStart(t *testing.T) {
	driver := &lifecycleDriver{}
	cfg := config.Config{
		SchemaVersion: 1,
		Tunnel: config.Tunnel{
			Interface:  "vtrust0",
			Address:    "10.100.0.1/24",
			ListenPort: 51820,
			PrivateKey: "AQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQE=",
		},
	}
	err := runWithDriver(context.Background(), cfg, "invalid-listener-address", driver)
	if err == nil {
		t.Fatal("Run should fail when the health listener address is invalid")
	}
	if !driver.closed {
		t.Fatal("dataplane was not closed after health listener startup failed")
	}
}

func TestRunWithDriverRequiresContext(t *testing.T) {
	err := runWithDriver(nil, config.Config{}, "127.0.0.1:0", &lifecycleDriver{})
	if err == nil || err.Error() != "context is required" {
		t.Fatalf("Run error = %v, want context is required", err)
	}
}

func TestFileConfigSourceDetectsReplacementAndSuppressesRepeatedInvalidState(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "gateway.json")
	cfg := runtimeTestConfig()
	writeRuntimeConfig(t, path, cfg)
	_, digest, err := config.LoadFileWithDigest(path)
	if err != nil {
		t.Fatal(err)
	}
	source := newFileConfigSource(path, digest)
	if got, apply, changed, err := source.poll(false); err != nil || apply || changed || got != nil {
		t.Fatalf("unchanged initial poll = (%v, %t, %t, %v), want no update", got, apply, changed, err)
	}

	cfg.Tunnel.ListenPort = 51821
	writeRuntimeConfig(t, path, cfg)
	got, apply, changed, err := source.poll(false)
	if err != nil || !apply || !changed || got == nil || got.Tunnel.ListenPort != 51821 {
		t.Fatalf("replacement poll = (%v, %t, %t, %v), want changed config", got, apply, changed, err)
	}

	if err := os.WriteFile(path, []byte("{"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, apply, changed, err := source.poll(false); err == nil || apply || !changed {
		t.Fatalf("invalid replacement poll = (apply %t, changed %t, %v), want one rejected change", apply, changed, err)
	}
	if _, apply, changed, err := source.poll(false); err == nil || apply || changed {
		t.Fatalf("repeated invalid poll = (apply %t, changed %t, %v), want suppressed repeated state", apply, changed, err)
	}

	cfg.Tunnel.ListenPort = 51822
	writeRuntimeConfig(t, path, cfg)
	got, apply, changed, err = source.poll(false)
	if err != nil || !apply || !changed || got == nil || got.Tunnel.ListenPort != 51822 {
		t.Fatalf("repaired file poll = (%v, %t, %t, %v), want repaired config", got, apply, changed, err)
	}
}

func TestFileConfigSourceRetriesUnchangedValidConfigWhenRequested(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "gateway.json")
	cfg := runtimeTestConfig()
	writeRuntimeConfig(t, path, cfg)
	_, digest, err := config.LoadFileWithDigest(path)
	if err != nil {
		t.Fatal(err)
	}
	source := newFileConfigSource(path, digest)
	got, apply, changed, err := source.poll(true)
	if err != nil || !apply || changed || got == nil || got.Tunnel.Interface != cfg.Tunnel.Interface {
		t.Fatalf("retry poll = (%v, %t, %t, %v), want unchanged config retry", got, apply, changed, err)
	}
}

func runtimeTestConfig() config.Config {
	return config.Config{
		SchemaVersion: 1,
		Tunnel: config.Tunnel{
			Interface:  "vtrust0",
			Address:    "10.100.0.1/24",
			ListenPort: 51820,
			PrivateKey: "AQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQE=",
		},
	}
}

func writeRuntimeConfig(t *testing.T, path string, cfg config.Config) {
	t.Helper()
	data, err := json.Marshal(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
}
