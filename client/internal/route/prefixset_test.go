package route

import (
	"net/netip"
	"testing"
)

func TestPrefixSetsEqual(t *testing.T) {
	a := []netip.Prefix{
		netip.MustParsePrefix("192.0.2.7/24"),
		netip.MustParsePrefix("2001:db8::/32"),
		netip.MustParsePrefix("192.0.2.0/24"),
	}
	b := []netip.Prefix{
		netip.MustParsePrefix("2001:db8::/32"),
		netip.MustParsePrefix("192.0.2.0/24"),
	}
	equal, err := PrefixSetsEqual(a, b)
	if err != nil {
		t.Fatal(err)
	}
	if !equal {
		t.Fatal("equivalent route prefix sets were considered different")
	}

	equal, err = PrefixSetsEqual(a, []netip.Prefix{netip.MustParsePrefix("192.0.2.0/24")})
	if err != nil {
		t.Fatal(err)
	}
	if equal {
		t.Fatal("different route prefix sets were considered equal")
	}
}

func TestPrefixSetsEqualRejectsInvalidPrefixes(t *testing.T) {
	if _, err := PrefixSetsEqual([]netip.Prefix{{}}, nil); err == nil {
		t.Fatal("PrefixSetsEqual accepted an invalid current prefix")
	}
	if _, err := PrefixSetsEqual(nil, []netip.Prefix{{}}); err == nil {
		t.Fatal("PrefixSetsEqual accepted an invalid desired prefix")
	}
}
