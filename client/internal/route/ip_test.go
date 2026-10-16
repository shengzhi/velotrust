package route

import (
	"net"
	"net/netip"
	"testing"
)

func TestPrefixFromIP(t *testing.T) {
	tests := []struct {
		name string
		ip   net.IP
		want netip.Prefix
	}{
		{name: "IPv4", ip: net.ParseIP("192.0.2.7"), want: netip.MustParsePrefix("192.0.2.7/32")},
		{name: "IPv6", ip: net.ParseIP("2001:db8::7"), want: netip.MustParsePrefix("2001:db8::7/128")},
		{name: "mapped IPv4", ip: net.ParseIP("::ffff:192.0.2.7"), want: netip.MustParsePrefix("192.0.2.7/32")},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := PrefixFromIP(test.ip)
			if err != nil {
				t.Fatal(err)
			}
			if got != test.want {
				t.Fatalf("PrefixFromIP() = %s, want %s", got, test.want)
			}
		})
	}
	if _, err := PrefixFromIP(net.IP{1, 2, 3}); err == nil {
		t.Fatal("PrefixFromIP accepted an invalid IP")
	}
}
