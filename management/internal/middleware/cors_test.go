package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCORSHandlesAllowedPreflight(t *testing.T) {
	h := CORS(CORSConfig{AllowedOrigins: []string{"https://console.example"}})(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { t.Fatal("next called") }))
	req := httptest.NewRequest(http.MethodOptions, "/jrpc", nil)
	req.Header.Set("Origin", "https://console.example")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusNoContent || rr.Header().Get("Access-Control-Allow-Origin") != "https://console.example" {
		t.Fatalf("status=%d headers=%v", rr.Code, rr.Header())
	}
}

func TestCORSDoesNotAllowUnknownOrigin(t *testing.T) {
	h := CORS(CORSConfig{AllowedOrigins: []string{"https://console.example"}})(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) }))
	req := httptest.NewRequest(http.MethodPost, "/jrpc", nil)
	req.Header.Set("Origin", "https://untrusted.example")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Fatalf("headers=%v", rr.Header())
	}
}

func TestCORSUsesPermissiveDefaults(t *testing.T) {
	h := CORS(CORSConfig{})(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	req := httptest.NewRequest(http.MethodPost, "/jrpc", nil)
	req.Header.Set("Origin", "https://client.example")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if got := rr.Header().Get("Access-Control-Allow-Origin"); got != "https://client.example" {
		t.Fatalf("allow-origin = %q, want request origin", got)
	}
	if got := rr.Header().Get("Access-Control-Allow-Methods"); got != "POST, OPTIONS" {
		t.Fatalf("allow-methods = %q", got)
	}
}
