package grpc

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"time"

	grpc "google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
)

// Server wraps the transport implementation while keeping gRPC details out of
// HUB runtime and business packages.
type Server struct {
	server *grpc.Server
	health *health.Server
}

// NewServer creates a gRPC server with the standard health API. A nil TLS
// config explicitly selects plaintext HTTP/2 transport.
func NewServer(tlsConfig *tls.Config) *Server {
	healthServer := health.NewServer()
	healthServer.SetServingStatus("", healthpb.HealthCheckResponse_NOT_SERVING)
	options := make([]grpc.ServerOption, 0, 1)
	if tlsConfig != nil {
		options = append(options, grpc.Creds(credentials.NewTLS(tlsConfig)))
	}
	server := grpc.NewServer(options...)
	healthpb.RegisterHealthServer(server, healthServer)
	return &Server{server: server, health: healthServer}
}

// RegisterService registers a generated gRPC service implementation.
func (s *Server) RegisterService(register func(grpc.ServiceRegistrar)) {
	register(s.server)
}

// Serve runs the gRPC server on an already-open listener.
func (s *Server) Serve(ctx context.Context, listener net.Listener) error {
	s.health.SetServingStatus("", healthpb.HealthCheckResponse_SERVING)
	serveResult := make(chan error, 1)
	go func() { serveResult <- s.server.Serve(listener) }()

	select {
	case err := <-serveResult:
		if errors.Is(err, grpc.ErrServerStopped) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("serve HUB gRPC: %w", err)
		}
		return nil
	case <-ctx.Done():
		return s.Shutdown(context.Background())
	}
}

// Shutdown marks the service unavailable and allows active RPCs to finish.
func (s *Server) Shutdown(ctx context.Context) error {
	s.health.SetServingStatus("", healthpb.HealthCheckResponse_NOT_SERVING)
	done := make(chan struct{})
	go func() {
		s.server.GracefulStop()
		close(done)
	}()

	select {
	case <-done:
		return nil
	case <-ctx.Done():
		s.server.Stop()
		<-done
		return ctx.Err()
	case <-time.After(10 * time.Second):
		s.server.Stop()
		<-done
		return nil
	}
}
