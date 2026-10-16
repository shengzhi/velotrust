//go:build linux && !android

package sharedsock

import (
	"errors"
	"net/netip"
	"syscall"
	"testing"
)

func newTestSourceProbe(t *testing.T, family int) *SourceProbe {
	t.Helper()
	probe, err := NewSourceProbe(family, 0)
	if err != nil {
		var errno syscall.Errno
		if errors.As(err, &errno) && (errno == syscall.EPERM || errno == syscall.EACCES) {
			t.Skipf("socket creation is restricted by the environment: %v", err)
		}
		t.Fatal(err)
	}
	return probe
}

func TestSourceProbeResolve(t *testing.T) {
	probe := newTestSourceProbe(t, syscall.AF_INET)
	t.Cleanup(func() { _ = probe.Close() })

	want := netip.MustParseAddr("127.0.0.1")
	for i := 0; i < 3; i++ {
		got, err := probe.Resolve(want)
		if err != nil {
			t.Fatal(err)
		}
		if got != want {
			t.Fatalf("Resolve() = %s, want %s", got, want)
		}
	}
}

func TestSourceProbeRejectsInvalidAndMismatchedDestinations(t *testing.T) {
	probe := newTestSourceProbe(t, syscall.AF_INET)
	defer probe.Close()
	if _, err := probe.Resolve(netip.Addr{}); err == nil {
		t.Fatal("Resolve accepted an invalid destination")
	}
	if _, err := probe.Resolve(netip.MustParseAddr("::1")); err == nil {
		t.Fatal("Resolve accepted an address from another family")
	}
}

func TestSourceProbeCloseIsIdempotent(t *testing.T) {
	probe := newTestSourceProbe(t, syscall.AF_INET)
	if err := probe.Close(); err != nil {
		t.Fatal(err)
	}
	if err := probe.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := probe.Resolve(netip.MustParseAddr("127.0.0.1")); !errors.Is(err, ErrProbeClosed) {
		t.Fatalf("Resolve() error = %v, want ErrProbeClosed", err)
	}
}
