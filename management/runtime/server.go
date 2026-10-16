package runtime

import (
	"context"

	transportjrpc "velotrust/management/internal/transport/jrpc"
	protocol "velotrust/pkg/jrpc"
)

// Registrar adds one Management logical service to the JRPC endpoint.
type Registrar = transportjrpc.Registrar

// Config contains the listener and protocol settings for Management.
type Config struct {
	ListenAddr       string
	Version          string
	MaxRequestBytes  int64
	IdentityResolver protocol.IdentityResolver
}

// Server owns the Management HTTP/JRPC lifecycle and can be started separately
// or alongside HUB by a combined process.
type Server struct {
	transport *transportjrpc.Server
}

// New constructs a Management server with the shared JRPC protocol runtime.
func New(cfg Config, registrars ...Registrar) (*Server, error) {
	transport, err := transportjrpc.NewServer(transportjrpc.Config{
		ListenAddr:       cfg.ListenAddr,
		Version:          cfg.Version,
		MaxRequestBytes:  cfg.MaxRequestBytes,
		IdentityResolver: cfg.IdentityResolver,
	}, registrars...)
	if err != nil {
		return nil, err
	}
	return &Server{transport: transport}, nil
}

// ListenAddr returns the configured JRPC listener address.
func (s *Server) ListenAddr() string { return s.transport.ListenAddr() }

// Serve starts the Management service until its context is canceled.
func (s *Server) Serve(ctx context.Context) error { return s.transport.Serve(ctx) }

// Shutdown gracefully stops the Management service.
func (s *Server) Shutdown(ctx context.Context) error { return s.transport.Shutdown(ctx) }
