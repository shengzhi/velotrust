package jrpc

import (
	"net/http/httptest"
	"net/netip"
	"testing"
)

func TestClientIPIgnoresForwardedHeadersFromUntrustedPeer(t *testing.T) {
	request := httptest.NewRequest("POST", "http://management/jrpc", nil)
	request.RemoteAddr = "203.0.113.7:50000"
	request.Header.Set("X-Forwarded-For", "198.51.100.22")
	request.Header.Set("X-Real-IP", "192.0.2.44")
	ctx := &Context{HttpRequest: request}
	if got := ctx.ClientIP(); got != "203.0.113.7" {
		t.Fatalf("untrusted forwarded header changed client IP: got %q", got)
	}
}

func TestClientIPWalksForwardedChainFromTrustedProxy(t *testing.T) {
	request := httptest.NewRequest("POST", "http://management/jrpc", nil)
	request.RemoteAddr = "10.0.0.2:443"
	request.Header.Set("X-Forwarded-For", "198.51.100.22, 10.0.0.1")
	ctx := &Context{HttpRequest: request, trustedProxies: []netip.Prefix{netip.MustParsePrefix("10.0.0.0/8")}}
	if got := ctx.ClientIP(); got != "198.51.100.22" {
		t.Fatalf("wrong client IP resolved from trusted proxy chain: got %q", got)
	}
}

func TestClientIPStopsAtFirstUntrustedForwardedHop(t *testing.T) {
	request := httptest.NewRequest("POST", "http://management/jrpc", nil)
	request.RemoteAddr = "10.0.0.2:443"
	request.Header.Set("X-Forwarded-For", "198.51.100.22, 203.0.113.99")
	ctx := &Context{HttpRequest: request, trustedProxies: []netip.Prefix{netip.MustParsePrefix("10.0.0.0/8")}}
	if got := ctx.ClientIP(); got != "203.0.113.99" {
		t.Fatalf("client IP resolution trusted a spoofed left-hand hop: got %q", got)
	}
}
