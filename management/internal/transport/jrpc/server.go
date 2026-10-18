package jrpc

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"strings"
	"sync/atomic"
	"time"

	protocol "velotrust/pkg/jrpc"
)

const defaultMaxRequestBytes = 1 << 20

// Registrar binds one Management logical service to the JRPC method registry.
type Registrar func(*protocol.Server) error

// Config contains the HTTP transport settings for Management JRPC.
type Config struct {
	ListenAddr       string
	Version          string
	MaxRequestBytes  int64
	IdentityResolver protocol.IdentityResolver
	TrustedProxies   []string
	HTTPMiddleware   func(http.Handler) http.Handler
}

// Server serves the template-compatible JRPC envelope at POST /jrpc.
type Server struct {
	addr      string
	http      *http.Server
	ready     atomic.Bool
	listening atomic.Bool
}

// NewServer creates the JRPC endpoint, health endpoints, and registered
// application services. Unregistered methods remain unavailable; registered
// methods require an identity unless a service marks them public.
func NewServer(cfg Config, registrars ...Registrar) (*Server, error) {
	if err := validateListenAddr(cfg.ListenAddr); err != nil {
		return nil, err
	}
	if cfg.MaxRequestBytes <= 0 {
		cfg.MaxRequestBytes = defaultMaxRequestBytes
	}

	trustedProxies, err := parseTrustedProxies(cfg.TrustedProxies)
	if err != nil {
		return nil, fmt.Errorf("configure trusted proxies: %w", err)
	}
	rpcServer := protocol.NewServer(protocol.WithIdentityResolver(cfg.IdentityResolver), protocol.WithTrustedProxies(trustedProxies))
	rpcServer.Use(protocol.Logger(false))
	if err := RegisterSystemService(rpcServer, cfg.Version); err != nil {
		return nil, fmt.Errorf("register Management system service: %w", err)
	}
	for _, register := range registrars {
		if register == nil {
			return nil, errors.New("Management JRPC registrar is nil")
		}
		if err := register(rpcServer); err != nil {
			return nil, fmt.Errorf("register Management JRPC service: %w", err)
		}
	}

	mux := http.NewServeMux()
	var rpcHandler http.Handler = http.MaxBytesHandler(rpcServer, cfg.MaxRequestBytes)
	if cfg.HTTPMiddleware != nil {
		rpcHandler = cfg.HTTPMiddleware(rpcHandler)
	}
	mux.Handle("/jrpc", rpcHandler)
	mux.HandleFunc("/healthz", healthHandler)

	server := &Server{addr: cfg.ListenAddr}
	mux.HandleFunc("/readyz", func(w http.ResponseWriter, _ *http.Request) {
		if !server.ready.Load() {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
	})
	server.http = &http.Server{
		Addr:              cfg.ListenAddr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    1 << 20,
	}
	return server, nil
}

func parseTrustedProxies(values []string) ([]netip.Prefix, error) {
	proxies := make([]netip.Prefix, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if prefix, err := netip.ParsePrefix(value); err == nil {
			proxies = append(proxies, prefix.Masked())
			continue
		}
		addr, err := netip.ParseAddr(value)
		if err != nil {
			return nil, fmt.Errorf("invalid trusted proxy %q", value)
		}
		bits := 128
		if addr.Is4() {
			bits = 32
		}
		proxies = append(proxies, netip.PrefixFrom(addr, bits))
	}
	return proxies, nil
}

// RegisterSystemService binds a public System_Version method for clients to
// verify the Management JRPC endpoint and protocol envelope.
func RegisterSystemService(server *protocol.Server, version string) error {
	if strings.TrimSpace(version) == "" {
		version = "dev"
	}
	return server.RegisterServiceWithOptions(&systemService{version: version}, "System", protocol.ServiceOptions{
		PublicMethods: []string{"Version"},
	})
}

type systemService struct{ version string }

type VersionArgs struct{}

type VersionReply struct {
	Service string `json:"service"`
	Version string `json:"version"`
}

func (s *systemService) Version(_ *protocol.Context, _ *VersionArgs) (*VersionReply, error) {
	return &VersionReply{Service: "management", Version: s.version}, nil
}

// ListenAddr returns the configured HTTP listener address.
func (s *Server) ListenAddr() string { return s.addr }

// Serve starts Management JRPC and stops accepting requests when ctx ends.
func (s *Server) Serve(ctx context.Context) error {
	listener, err := net.Listen("tcp", s.addr)
	if err != nil {
		return fmt.Errorf("listen for Management JRPC: %w", err)
	}
	s.listening.Store(true)
	s.ready.Store(true)
	serveErr := make(chan error, 1)
	go func() { serveErr <- s.http.Serve(listener) }()

	select {
	case err := <-serveErr:
		s.ready.Store(false)
		s.listening.Store(false)
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return s.Shutdown(shutdownCtx)
	}
}

// Shutdown gracefully stops the HTTP listener.
func (s *Server) Shutdown(ctx context.Context) error {
	s.ready.Store(false)
	if !s.listening.Load() {
		return nil
	}
	if err := s.http.Shutdown(ctx); err != nil {
		return fmt.Errorf("shutdown Management JRPC: %w", err)
	}
	s.listening.Store(false)
	return nil
}

func validateListenAddr(addr string) error {
	if _, _, err := net.SplitHostPort(addr); err != nil {
		return fmt.Errorf("Management listen address must be host:port: %w", err)
	}
	return nil
}

func healthHandler(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok\n"))
}
