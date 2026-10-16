package sharedsock

import "testing"

func TestIncomingSTUNFilterBuildsIPv4AndIPv6Programs(t *testing.T) {
	filter := NewIncomingSTUNFilter()
	if filter == nil {
		t.Skip("BPF filters are unavailable on this platform")
	}
	ipv4, ipv6, err := filter.GetInstructions(3478)
	if err != nil {
		t.Fatal(err)
	}
	if len(ipv4) == 0 || len(ipv6) == 0 {
		t.Fatalf("expected both programs, got IPv4=%d IPv6=%d", len(ipv4), len(ipv6))
	}
}
