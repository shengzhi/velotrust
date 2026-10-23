package postgres

import (
	"errors"
	"net/netip"
	"testing"
)

func TestFirstAvailableClientAddressSkipsNetworkBroadcastAndLeases(t *testing.T) {
	prefix := netip.MustParsePrefix("10.80.1.0/29")
	occupied := map[netip.Addr]struct{}{
		netip.MustParseAddr("10.80.1.1"): {},
		netip.MustParseAddr("10.80.1.2"): {},
	}
	got, err := firstAvailableClientAddress(prefix, occupied)
	if err != nil {
		t.Fatalf("firstAvailableClientAddress() error = %v", err)
	}
	if want := netip.MustParseAddr("10.80.1.3"); got != want {
		t.Fatalf("firstAvailableClientAddress() = %s, want %s", got, want)
	}
}

func TestFirstAvailableClientAddressExhausted(t *testing.T) {
	prefix := netip.MustParsePrefix("10.80.1.0/30")
	occupied := map[netip.Addr]struct{}{
		netip.MustParseAddr("10.80.1.1"): {},
		netip.MustParseAddr("10.80.1.2"): {},
	}
	if _, err := firstAvailableClientAddress(prefix, occupied); !errors.Is(err, errClientAddressUnavailable) {
		t.Fatalf("firstAvailableClientAddress() error = %v, want unavailable", err)
	}
}

func TestClientPoolContainsMatchesPoolID(t *testing.T) {
	candidates := []clientAssignmentCandidate{{GroupID: 3, PoolID: 7}}
	if !clientPoolContains(candidates, 7) {
		t.Fatal("expected the assigned pool to match")
	}
	if clientPoolContains(candidates, 8) {
		t.Fatal("expected a different pool not to match")
	}
}
