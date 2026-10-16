package netiputil

import (
	"net/netip"
	"testing"
)

func TestEncodeDecodePrefix(t *testing.T) {
	for _, value := range []string{
		"100.64.0.1/32", "10.0.0.0/8", "0.0.0.0/0",
		"fd00::1/128", "fd00:1234:5678::/48", "::/0",
		"100.64.0.1/16", "fd00::abcd:1/64",
	} {
		t.Run(value, func(t *testing.T) {
			want := netip.MustParsePrefix(value)
			encoded, err := EncodePrefix(want)
			if err != nil {
				t.Fatal(err)
			}
			if want.Addr().Is4() && len(encoded) != 5 || want.Addr().Is6() && len(encoded) != 17 {
				t.Fatalf("encoded length = %d", len(encoded))
			}
			got, err := DecodePrefix(encoded)
			if err != nil || got != want {
				t.Fatalf("DecodePrefix() = %v, %v; want %v", got, err, want)
			}
		})
	}
}

func TestCompactPrefixRejectsInvalidValues(t *testing.T) {
	for _, prefix := range []netip.Prefix{{}, netip.MustParsePrefix("::ffff:10.0.0.0/120")} {
		if _, err := EncodePrefix(prefix); err == nil {
			t.Errorf("EncodePrefix(%v) succeeded, want error", prefix)
		}
	}
	badIPv6Prefix := make([]byte, 17)
	badIPv6Prefix[16] = 129
	for _, encoded := range [][]byte{nil, {1, 2, 3}, {10, 0, 0, 1, 33}, badIPv6Prefix} {
		if _, err := DecodePrefix(encoded); err == nil {
			t.Errorf("DecodePrefix(%v) succeeded, want error", encoded)
		}
	}
}

func TestEncodeDecodeAddress(t *testing.T) {
	for _, value := range []string{"100.64.0.5", "fd00::5"} {
		want := netip.MustParseAddr(value)
		got, err := DecodeAddr(EncodeAddr(want))
		if err != nil || got != want {
			t.Errorf("DecodeAddr() = %v, %v; want %v", got, err, want)
		}
	}
}
