package configurer

import (
	"net"
	"net/netip"
	"testing"
)

func TestAllowedIPStoreTransfersPrefixOwnership(t *testing.T) {
	store := newAllowedIPStore[string]()
	prefix := netip.MustParsePrefix("10.20.0.1/16")
	store.set("peer-a", []netip.Prefix{prefix})
	store.add("peer-b", []netip.Prefix{netip.MustParsePrefix("10.20.0.0/16")})

	if got, ok := store.get("peer-a"); !ok || len(got) != 0 {
		t.Fatalf("old owner = %v, %t; want empty known peer", got, ok)
	}
	got, ok := store.get("peer-b")
	if !ok || len(got) != 1 || got[0] != netip.MustParsePrefix("10.20.0.0/16") {
		t.Fatalf("new owner = %v, %t", got, ok)
	}
	got[0] = netip.MustParsePrefix("192.0.2.0/24")
	if stored, _ := store.get("peer-b"); stored[0] != netip.MustParsePrefix("10.20.0.0/16") {
		t.Fatalf("get returned a mutable store slice: %v", stored)
	}
}

func TestAllowedIPStoreUpdateOnlyAndReset(t *testing.T) {
	store := newAllowedIPStore[string]()
	store.addExisting("missing", []netip.Prefix{netip.MustParsePrefix("192.0.2.0/24")})
	if _, ok := store.get("missing"); ok {
		t.Fatal("update-only add created an unknown peer")
	}
	store.ensure("peer")
	if prefixes, ok := store.get("peer"); !ok || len(prefixes) != 0 {
		t.Fatalf("ensure result = %v, %t", prefixes, ok)
	}
	store.reset()
	if _, ok := store.get("peer"); ok {
		t.Fatal("reset retained peer state")
	}
}

func TestIPNetsToPrefixesNormalizesMappedAddresses(t *testing.T) {
	got := ipNetsToPrefixes([]net.IPNet{
		{IP: net.ParseIP("192.0.2.7"), Mask: net.CIDRMask(24, 32)},
		{IP: net.ParseIP("::ffff:192.0.2.7"), Mask: net.CIDRMask(120, 128)},
		{IP: net.ParseIP("2001:db8::7"), Mask: net.CIDRMask(64, 128)},
	})
	want := []netip.Prefix{
		netip.MustParsePrefix("192.0.2.0/24"),
		netip.MustParsePrefix("192.0.2.0/24"),
		netip.MustParsePrefix("2001:db8::/64"),
	}
	if len(got) != len(want) {
		t.Fatalf("prefix count = %d, want %d (%v)", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("prefix %d = %s, want %s", i, got[i], want[i])
		}
	}
}
