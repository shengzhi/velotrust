package wgaddr

import (
	"net/netip"
	"testing"
)

func TestParseWGAddress(t *testing.T) {
	got, err := ParseWGAddress("100.64.0.7/16")
	if err != nil {
		t.Fatalf("ParseWGAddress() error = %v", err)
	}
	if got.IP != netip.MustParseAddr("100.64.0.7") {
		t.Errorf("IP = %s, want 100.64.0.7", got.IP)
	}
	if got.Prefix() != netip.MustParsePrefix("100.64.0.7/16") {
		t.Errorf("Prefix() = %s, want 100.64.0.7/16", got.Prefix())
	}
	if got.HostPrefix() != netip.MustParsePrefix("100.64.0.7/32") {
		t.Errorf("HostPrefix() = %s, want 100.64.0.7/32", got.HostPrefix())
	}
}

func TestParseWGAddressRejectsInvalidAndIPv6PrimaryAddresses(t *testing.T) {
	for _, input := range []string{"", "100.64.0.7", "2001:db8::7/64", "192.0.2.1/33"} {
		if _, err := ParseWGAddress(input); err == nil {
			t.Errorf("ParseWGAddress(%q) succeeded, want error", input)
		}
	}
}

func TestIPv6OverlayAddressLifecycle(t *testing.T) {
	addr := MustParseWGAddress("100.64.0.7/16")
	if err := addr.SetIPv6Prefix(netip.MustParsePrefix("fd00::7/64")); err != nil {
		t.Fatalf("SetIPv6Prefix() error = %v", err)
	}
	if !addr.HasIPv6() {
		t.Fatal("HasIPv6() = false after assignment")
	}
	if got, want := addr.IPv6String(), "fd00::7/64"; got != want {
		t.Errorf("IPv6String() = %q, want %q", got, want)
	}
	if got, want := addr.IPv6HostPrefix(), netip.MustParsePrefix("fd00::7/128"); got != want {
		t.Errorf("IPv6HostPrefix() = %s, want %s", got, want)
	}
	addr.ClearIPv6()
	if addr.HasIPv6() || addr.IPv6String() != "" || addr.IPv6HostPrefix().IsValid() {
		t.Fatal("IPv6 fields remain set after ClearIPv6()")
	}
}

func TestSetIPv6PrefixRejectsOtherAddressFamilies(t *testing.T) {
	addr := MustParseWGAddress("100.64.0.7/16")
	for _, input := range []string{"192.0.2.1/24", "::ffff:192.0.2.1/120"} {
		if err := addr.SetIPv6Prefix(netip.MustParsePrefix(input)); err == nil {
			t.Errorf("SetIPv6Prefix(%q) succeeded, want error", input)
		}
	}
}
