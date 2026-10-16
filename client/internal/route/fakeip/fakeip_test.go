package fakeip

import (
	"net/netip"
	"testing"
)

func TestFakeIPAllocationIsStableAndReversible(t *testing.T) {
	manager := NewManager()
	real := netip.MustParseAddr("192.0.2.15")

	fake, err := manager.AllocateFakeIP(real)
	if err != nil {
		t.Fatal(err)
	}
	if fake != netip.MustParseAddr("240.0.0.1") {
		t.Fatalf("first fake IPv4 = %s", fake)
	}
	if again, err := manager.AllocateFakeIP(real); err != nil || again != fake {
		t.Fatalf("repeat allocation = %s, %v; want %s", again, err, fake)
	}
	if got, ok := manager.GetRealIP(fake); !ok || got != real {
		t.Fatalf("GetRealIP() = %s, %t; want %s", got, ok, real)
	}
	if got, ok := manager.GetFakeIP(real); !ok || got != fake {
		t.Fatalf("GetFakeIP() = %s, %t; want %s", got, ok, fake)
	}
}

func TestFakeIPAllocationUsesSeparateIPv6Pool(t *testing.T) {
	manager := NewManager()
	real := netip.MustParseAddr("2001:db8::15")
	fake, err := manager.AllocateFakeIP(real)
	if err != nil {
		t.Fatal(err)
	}
	if fake != netip.MustParseAddr("100::1") {
		t.Fatalf("fake IPv6 = %s", fake)
	}
	if !manager.GetFakeIPv6Block().Contains(fake) {
		t.Fatalf("IPv6 fake address %s outside pool %s", fake, manager.GetFakeIPv6Block())
	}
}

func TestFakeIPRejectsInvalidAddress(t *testing.T) {
	if _, err := NewManager().AllocateFakeIP(netip.Addr{}); err == nil {
		t.Fatal("AllocateFakeIP(invalid) succeeded, want error")
	}
}
