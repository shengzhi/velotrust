package runtime

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"

	"velotrust/hub/config"
	hubgrpc "velotrust/hub/transport/grpc"
)

// Registrar attaches one logical HUB service to the shared gRPC server.
// Registrars are invoked once during construction, before the listener starts.
type Registrar interface {
	RegisterGRPC(*hubgrpc.Server)
}

// Server owns the HUB listener lifecycle. It can run alone or be embedded in a
// future combined Management/HUB process without duplicating service logic.
type Server struct {
	cfg       config.Config
	transport *hubgrpc.Server
}

// New constructs a TLS-protected HUB server and registers the supplied
// services. The caller controls when Serve is started.
func New(cfg config.Config, registrars ...Registrar) (*Server, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	certificate, err := tls.LoadX509KeyPair(cfg.TLS.CertFile, cfg.TLS.KeyFile)
	if err != nil {
		return nil, fmt.Errorf("load HUB TLS certificate: %w", err)
	}

	transport := hubgrpc.NewServer(&tls.Config{
		MinVersion:   tls.VersionTLS12,
		Certificates: []tls.Certificate{certificate},
	})
	for _, registrar := range registrars {
		if registrar == nil {
			return nil, errors.New("HUB service registrar is nil")
		}
		registrar.RegisterGRPC(transport)
	}
	return &Server{cfg: cfg, transport: transport}, nil
}

// InstanceID returns the configured stable instance identifier.
func (s *Server) InstanceID() string { return s.cfg.InstanceID }

// ListenAddr returns the configured gRPC listen address.
func (s *Server) ListenAddr() string { return s.cfg.ListenAddr }

// Serve starts the listener and gracefully stops active RPCs when ctx ends.
func (s *Server) Serve(ctx context.Context) error {
	listener, err := net.Listen("tcp", s.cfg.ListenAddr)
	if err != nil {
		return fmt.Errorf("listen for HUB gRPC: %w", err)
	}
	return s.transport.Serve(ctx, listener)
}

// Shutdown stops the server when its owner manages listener lifecycles itself.
func (s *Server) Shutdown(ctx context.Context) error {
	return s.transport.Shutdown(ctx)
}
