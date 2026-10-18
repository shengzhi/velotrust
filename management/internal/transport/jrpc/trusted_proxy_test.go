package jrpc

import "testing"

func TestParseTrustedProxies(t *testing.T) {
	proxies, err := parseTrustedProxies([]string{"127.0.0.1", "10.10.0.0/16"})
	if err != nil {
		t.Fatal(err)
	}
	if len(proxies) != 2 || proxies[0].String() != "127.0.0.1/32" || proxies[1].String() != "10.10.0.0/16" {
		t.Fatalf("unexpected trusted proxy ranges: %v", proxies)
	}
	if _, err := parseTrustedProxies([]string{"not-an-ip"}); err == nil {
		t.Fatal("invalid trusted proxy address was accepted")
	}
}
