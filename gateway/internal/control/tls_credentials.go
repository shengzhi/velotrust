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
	inner        credentials.TransportCredentials
	shared       *tlsStateHolder
	captureState bool
}

type tlsStateHolder struct {
	mu    sync.RWMutex
	state *tls.ConnectionState
}

func newCapturingTLSCredentials(cfg *tls.Config) *tlsStateCredentials {
	return &tlsStateCredentials{
		inner:        credentials.NewTLS(cfg),
		shared:       &tlsStateHolder{},
		captureState: true,
	}
}

func (c *tlsStateCredentials) ClientHandshake(ctx context.Context, authority string, rawConn net.Conn) (net.Conn, credentials.AuthInfo, error) {
	conn, authInfo, err := c.inner.ClientHandshake(ctx, authority, rawConn)
	if err != nil {
		return nil, nil, err
	}
	if c.captureState {
		if err := c.capture(authInfo); err != nil {
			_ = conn.Close()
			return nil, nil, err
		}
	}
	return conn, authInfo, nil
}

func (c *tlsStateCredentials) ServerHandshake(rawConn net.Conn) (net.Conn, credentials.AuthInfo, error) {
	conn, authInfo, err := c.inner.ServerHandshake(rawConn)
	if err != nil {
		return nil, nil, err
	}
	if c.captureState {
		if err := c.capture(authInfo); err != nil {
			_ = conn.Close()
			return nil, nil, err
		}
	}
	return conn, authInfo, nil
}

func (c *tlsStateCredentials) capture(authInfo credentials.AuthInfo) error {
	var state tls.ConnectionState
	switch info := authInfo.(type) {
	case credentials.TLSInfo:
		state = info.State
	case *credentials.TLSInfo:
		if info == nil {
			return errors.New("gRPC TLS credentials returned an empty connection state")
		}
		state = info.State
	default:
		return errors.New("gRPC transport did not establish TLS")
	}
	c.shared.mu.Lock()
	c.shared.state = &state
	c.shared.mu.Unlock()
	return nil
}

func (c *tlsStateCredentials) Info() credentials.ProtocolInfo {
	return c.inner.Info()
}

func (c *tlsStateCredentials) Clone() credentials.TransportCredentials {
	return &tlsStateCredentials{inner: c.inner.Clone(), shared: c.shared, captureState: c.captureState}
}

func (c *tlsStateCredentials) OverrideServerName(serverName string) error {
	return c.inner.OverrideServerName(serverName)
}

func (c *tlsStateCredentials) ConnectionState() (tls.ConnectionState, bool) {
	c.shared.mu.RLock()
	defer c.shared.mu.RUnlock()
	if c.shared.state == nil {
		return tls.ConnectionState{}, false
	}
	return *c.shared.state, true
}

var _ credentials.TransportCredentials = (*tlsStateCredentials)(nil)
