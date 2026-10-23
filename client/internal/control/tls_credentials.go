package control

import (
	"context"
	"crypto/tls"
	"errors"
	"net"
	"sync"

	"google.golang.org/grpc/credentials"
)

type tlsStateCredentials struct {
	inner credentials.TransportCredentials
	state *tlsStateHolder
}

type tlsStateHolder struct {
	mu    sync.RWMutex
	state *tls.ConnectionState
}

func (c *tlsStateCredentials) ClientHandshake(ctx context.Context, authority string, raw net.Conn) (net.Conn, credentials.AuthInfo, error) {
	conn, info, err := c.inner.ClientHandshake(ctx, authority, raw)
	if err != nil {
		return nil, nil, err
	}
	var state tls.ConnectionState
	switch value := info.(type) {
	case credentials.TLSInfo:
		state = value.State
	case *credentials.TLSInfo:
		if value == nil {
			_ = conn.Close()
			return nil, nil, errors.New("gRPC TLS handshake returned an empty state")
		}
		state = value.State
	default:
		_ = conn.Close()
		return nil, nil, errors.New("gRPC transport did not establish TLS")
	}
	c.state.mu.Lock()
	c.state.state = &state
	c.state.mu.Unlock()
	return conn, info, nil
}

func (c *tlsStateCredentials) ServerHandshake(raw net.Conn) (net.Conn, credentials.AuthInfo, error) {
	return c.inner.ServerHandshake(raw)
}

func (c *tlsStateCredentials) Info() credentials.ProtocolInfo { return c.inner.Info() }

func (c *tlsStateCredentials) Clone() credentials.TransportCredentials {
	return &tlsStateCredentials{inner: c.inner.Clone(), state: c.state}
}

func (c *tlsStateCredentials) OverrideServerName(name string) error {
	return c.inner.OverrideServerName(name)
}

func (c *tlsStateCredentials) ConnectionState() (tls.ConnectionState, bool) {
	c.state.mu.RLock()
	defer c.state.mu.RUnlock()
	if c.state.state == nil {
		return tls.ConnectionState{}, false
	}
	return *c.state.state, true
}

var _ credentials.TransportCredentials = (*tlsStateCredentials)(nil)
