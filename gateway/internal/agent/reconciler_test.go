package agent

import (
	"context"
	"errors"
	"net/http/httptest"
	"strings"
	"testing"

	"velotrust/gateway/config"
)

func validConfig() config.Config {
	return config.Config{
		SchemaVersion: 1,
		GatewayID:     "gw-west-1",
		Tunnel: config.Tunnel{
			Interface:  "vtrust0",
			Address:    "10.100.0.1/24",
			ListenPort: 51820,
			PrivateKey: "AQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQE=",
			Peers: []config.Peer{{
				ID:         "site-east",
				PublicKey:  "AgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgI=",
				AllowedIPs: []string{"10.110.0.0/16", "192.0.2.0/24", "198.51.100.0/24"},
			}},
		},
		Routes: []string{"192.0.2.0/24", "198.51.100.0/24"},
		Rules: []config.ACLRule{{
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

type fakeDriver struct {
	calls   []string
	failAt  string
	blocked bool
	enabled bool
	closed  bool
}

func (d *fakeDriver) record(name string) error {
	d.calls = append(d.calls, name)
	if d.failAt == name {
		return errors.New("simulated failure")
	}
	return nil
}

func (d *fakeDriver) BlockForwarding(context.Context) error {
	err := d.record("block")
	d.blocked = true
	d.enabled = false
	return err
}
func (d *fakeDriver) ApplyTunnel(context.Context, config.Tunnel) error {
	return d.record("tunnel")
}
func (d *fakeDriver) ApplyFirewall(context.Context, []config.ACLRule) error {
	return d.record("firewall")
}
func (d *fakeDriver) ApplyRoutes(context.Context, []string) error {
	return d.record("routes")
}
func (d *fakeDriver) ApplyNAT(context.Context, []config.NATMapping) error {
	return d.record("nat")
}
func (d *fakeDriver) EnableForwarding(context.Context) error {
	if err := d.record("enable"); err != nil {
		return err
	}
	d.enabled = true
	return nil
}
func (d *fakeDriver) UnblockForwarding(context.Context) error {
	if err := d.record("unblock"); err != nil {
		return err
	}
	d.blocked = false
	return nil
}
func (d *fakeDriver) Close(context.Context) error {
	if err := d.record("close"); err != nil {
		return err
	}
	d.closed = true
	return nil
}

func TestStartBlocksUntilAConfigIsApplied(t *testing.T) {
	driver := &fakeDriver{}
	reconciler, err := NewReconciler(driver)
	if err != nil {
		t.Fatal(err)
	}
	if err := reconciler.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := reconciler.Start(context.Background()); err != nil {
		t.Fatalf("second Start() failed: %v", err)
	}
	if !driver.blocked || reconciler.Status().Phase != PhasePending {
		t.Fatalf("startup did not remain blocked and pending: blocked=%t status=%+v", driver.blocked, reconciler.Status())
	}
	if strings.Join(driver.calls, ",") != "block" {
		t.Fatalf("startup calls = %v, want one block", driver.calls)
	}
	if err := reconciler.Apply(context.Background(), validConfig()); err != nil {
		t.Fatal(err)
	}
	if driver.blocked || reconciler.Status().Phase != PhaseReady {
		t.Fatalf("config apply did not activate dataplane: blocked=%t status=%+v", driver.blocked, reconciler.Status())
	}
}

func TestApplyOrdersFirewallBeforeRouteAndEnablesLast(t *testing.T) {
	driver := &fakeDriver{}
	reconciler, err := NewReconciler(driver)
	if err != nil {
		t.Fatal(err)
	}
	if err := reconciler.Apply(context.Background(), validConfig()); err != nil {
		t.Fatal(err)
	}
	want := []string{"block", "tunnel", "firewall", "routes", "nat", "enable", "unblock"}
	if strings.Join(driver.calls, ",") != strings.Join(want, ",") {
		t.Fatalf("call order = %v, want %v", driver.calls, want)
	}
	if driver.blocked || !driver.enabled || reconciler.Status().Phase != PhaseReady {
		t.Fatalf("unexpected post-apply state: blocked=%t enabled=%t status=%+v", driver.blocked, driver.enabled, reconciler.Status())
	}
}

func TestApplyFailureBlocksAndClosesDataplane(t *testing.T) {
	driver := &fakeDriver{failAt: "routes"}
	reconciler, err := NewReconciler(driver)
	if err != nil {
		t.Fatal(err)
	}
	if err := reconciler.Apply(context.Background(), validConfig()); err == nil {
		t.Fatal("Apply() succeeded despite route failure")
	}
	want := []string{"block", "tunnel", "firewall", "routes", "block", "close"}
	if strings.Join(driver.calls, ",") != strings.Join(want, ",") {
		t.Fatalf("call order = %v, want %v", driver.calls, want)
	}
	if !driver.blocked || driver.enabled || !driver.closed || reconciler.Status().Phase != PhaseDegraded {
		t.Fatalf("failure did not fail closed: blocked=%t enabled=%t closed=%t status=%+v", driver.blocked, driver.enabled, driver.closed, reconciler.Status())
	}
}

func TestApplyIsIdempotentAndReconcilesChangedConfig(t *testing.T) {
	driver := &fakeDriver{}
	reconciler, err := NewReconciler(driver)
	if err != nil {
		t.Fatal(err)
	}
	cfg := validConfig()
	if err := reconciler.Apply(context.Background(), cfg); err != nil {
		t.Fatal(err)
	}
	callCount := len(driver.calls)
	if err := reconciler.Apply(context.Background(), cfg); err != nil {
		t.Fatalf("duplicate apply failed: %v", err)
	}
	if len(driver.calls) != callCount {
		t.Fatal("identical config caused dataplane changes")
	}
	equivalent := cfg
	equivalent.Routes = []string{"198.51.100.8/24", "192.0.2.4/24"}
	if err := reconciler.Apply(context.Background(), equivalent); err != nil {
		t.Fatalf("canonical equivalent config failed: %v", err)
	}
	if len(driver.calls) != callCount {
		t.Fatal("canonically equivalent config caused dataplane changes")
	}

	changed := cfg
	changed.Rules = append([]config.ACLRule(nil), cfg.Rules...)
	changed.Rules[0].PortTo = 8443
	if err := reconciler.Apply(context.Background(), changed); err != nil {
		t.Fatal(err)
	}
	changedCallCount := len(driver.calls)
	if changedCallCount != callCount+7 {
		t.Fatalf("changed config made %d dataplane calls, want %d", changedCallCount-callCount, 7)
	}
	if err := reconciler.Apply(context.Background(), changed); err != nil {
		t.Fatalf("reapplying identical changed config failed: %v", err)
	}
	if len(driver.calls) != changedCallCount {
		t.Fatal("reapplying the same changed config caused dataplane changes")
	}
	if got := reconciler.Status().Phase; got != PhaseReady {
		t.Fatalf("changed config phase = %s, want %s", got, PhaseReady)
	}
}

func TestApplyInvalidConfigFailsClosed(t *testing.T) {
	driver := &fakeDriver{}
	reconciler, err := NewReconciler(driver)
	if err != nil {
		t.Fatal(err)
	}
	cfg := validConfig()
	cfg.Rules[0].Action = "unknown"
	if err := reconciler.Apply(context.Background(), cfg); err == nil {
		t.Fatal("Apply() accepted invalid policy")
	}
	if strings.Join(driver.calls, ",") != "block,close" || !driver.blocked || !driver.closed {
		t.Fatalf("invalid config was not quarantined: calls=%v blocked=%t closed=%t", driver.calls, driver.blocked, driver.closed)
	}
}

func TestUnchangedConfigCanRecoverAfterConfigurationRejection(t *testing.T) {
	driver := &fakeDriver{}
	reconciler, err := NewReconciler(driver)
	if err != nil {
		t.Fatal(err)
	}
	cfg := validConfig()
	if err := reconciler.Apply(context.Background(), cfg); err != nil {
		t.Fatal(err)
	}
	if err := reconciler.RejectConfiguration(context.Background(), errors.New("config file became invalid")); err == nil {
		t.Fatal("RejectConfiguration() did not report the rejection")
	}
	if got := reconciler.Status().Phase; got != PhaseDegraded {
		t.Fatalf("rejected config phase = %s, want %s", got, PhaseDegraded)
	}
	if err := reconciler.Apply(context.Background(), cfg); err != nil {
		t.Fatalf("reapplying the same config after recovery failed: %v", err)
	}
	if got := reconciler.Status().Phase; got != PhaseReady {
		t.Fatalf("recovered config phase = %s, want %s", got, PhaseReady)
	}
}

func TestStopIsIdempotentAndHealthDoesNotExposeSecrets(t *testing.T) {
	driver := &fakeDriver{}
	reconciler, err := NewReconciler(driver)
	if err != nil {
		t.Fatal(err)
	}
	cfg := validConfig()
	response := httptest.NewRecorder()
	reconciler.HealthHandler().ServeHTTP(response, httptest.NewRequest("GET", "/readyz", nil))
	if response.Code != 503 {
		t.Fatalf("unconfigured health status = %d, want 503", response.Code)
	}
	if err := reconciler.Apply(context.Background(), cfg); err != nil {
		t.Fatal(err)
	}
	response = httptest.NewRecorder()
	reconciler.HealthHandler().ServeHTTP(response, httptest.NewRequest("GET", "/healthz", nil))
	if response.Code != 200 {
		t.Fatalf("ready health status = %d, want 200", response.Code)
	}
	if strings.Contains(response.Body.String(), cfg.Tunnel.PrivateKey) {
		t.Fatal("health endpoint exposed private key material")
	}

	if err := reconciler.Stop(context.Background()); err != nil {
		t.Fatal(err)
	}
	callCount := len(driver.calls)
	if err := reconciler.Stop(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(driver.calls) != callCount || reconciler.Status().Phase != PhaseStopped {
		t.Fatal("Stop() was not idempotent")
	}
	if err := reconciler.Apply(context.Background(), cfg); !errors.Is(err, ErrAgentStopped) {
		t.Fatalf("Apply() after Stop() error = %v", err)
	}
}
