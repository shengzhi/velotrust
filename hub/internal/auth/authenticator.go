package auth

import (
	"context"
	"crypto/subtle"
	"errors"
	"fmt"
	"time"

	"velotrust/pkg/hubauth"
)

var (
	ErrGatewayNotFound        = errors.New("Gateway node was not found")
	ErrGatewayNotRegistered   = errors.New("Gateway node has not completed registration")
	ErrGatewayApprovalPending = errors.New("Gateway registration is awaiting Management approval")
	ErrGatewayNotAllowed      = errors.New("Gateway node is not allowed to connect")
	ErrPublicKeyMismatch      = errors.New("Gateway X25519 public key does not match the registered key")
	ErrRegistrationNotAllowed = errors.New("Gateway registration credential is not valid for enrollment")
	ErrPublicKeyAlreadyBound  = errors.New("Gateway X25519 public key is already registered")
	ErrClientNotFound         = errors.New("Client node was not found")
	ErrClientNotRegistered    = errors.New("Client node has not completed registration")
	ErrClientApprovalPending  = errors.New("Client registration is awaiting Management approval")
	ErrClientNotAllowed       = errors.New("Client node or device is not allowed to connect")
	ErrClientLoginFailed      = errors.New("employee Client login failed")
)

// GatewayIdentity is the minimum Management-owned node state HUB needs to
// authorize a Gateway control stream. PublicKey is the registered WireGuard
// X25519 key, encoded as its raw 32-byte value.
type GatewayIdentity struct {
	NodeID     string
	NodeStatus string
	PublicKey  []byte
	OwnerID    int64
	DeviceID   int64
}

type ClientIdentity = GatewayIdentity

// GatewayDirectory reads the current authoritative Gateway identity and
// lifecycle state. Implementations belong to HUB's repository layer.
type GatewayDirectory interface {
	LookupGatewayByPublicKey(context.Context, []byte) (GatewayIdentity, error)
}

type ClientDirectory interface {
	LookupClientByPublicKey(context.Context, []byte) (ClientIdentity, error)
}

// GatewayAuthenticator verifies the claimed identity against the node
// directory, then prepares an X25519 one-use challenge.
type GatewayAuthenticator struct {
	directory GatewayDirectory
	ttl       time.Duration
	now       func() time.Time
}

func NewGatewayAuthenticator(directory GatewayDirectory, ttl time.Duration) (*GatewayAuthenticator, error) {
	if directory == nil {
		return nil, errors.New("Gateway node directory is required")
	}
	if ttl <= 0 {
		return nil, errors.New("Gateway authentication challenge TTL must be positive")
	}
	return &GatewayAuthenticator{directory: directory, ttl: ttl, now: time.Now}, nil
}

// Begin validates current node state before creating any proof challenge. It
// only accepts active Gateway nodes with the exact registered public key.
func (a *GatewayAuthenticator) Begin(ctx context.Context, hubInstanceID string, claimedPublicKey, challengeID, channelBinding []byte) (GatewayIdentity, hubauth.Challenge, *hubauth.PendingChallenge, error) {
	if a == nil || a.directory == nil {
		return GatewayIdentity{}, hubauth.Challenge{}, nil, errors.New("Gateway authenticator is not configured")
	}
	if len(claimedPublicKey) != 32 {
		return GatewayIdentity{}, hubauth.Challenge{}, nil, errors.New("Gateway X25519 public key must contain 32 bytes")
	}
	identity, err := a.directory.LookupGatewayByPublicKey(ctx, claimedPublicKey)
	if err != nil {
		if errors.Is(err, ErrGatewayNotFound) {
			return GatewayIdentity{}, hubauth.Challenge{}, nil, ErrGatewayNotRegistered
		}
		return GatewayIdentity{}, hubauth.Challenge{}, nil, fmt.Errorf("load Gateway identity: %w", err)
	}
	if identity.NodeID == "" {
		return GatewayIdentity{}, hubauth.Challenge{}, nil, ErrGatewayNotFound
	}
	if identity.NodeStatus == "pending" {
		if len(identity.PublicKey) == 0 {
			return GatewayIdentity{}, hubauth.Challenge{}, nil, ErrGatewayNotRegistered
		}
		if len(claimedPublicKey) != len(identity.PublicKey) || subtle.ConstantTimeCompare(claimedPublicKey, identity.PublicKey) != 1 {
			return GatewayIdentity{}, hubauth.Challenge{}, nil, ErrPublicKeyMismatch
		}
		return GatewayIdentity{}, hubauth.Challenge{}, nil, ErrGatewayApprovalPending
	}
	if identity.NodeStatus != "active" {
		return GatewayIdentity{}, hubauth.Challenge{}, nil, ErrGatewayNotAllowed
	}
	if len(identity.PublicKey) != 32 {
		return GatewayIdentity{}, hubauth.Challenge{}, nil, errors.New("active Gateway has no valid X25519 public key")
	}
	if len(claimedPublicKey) != len(identity.PublicKey) || subtle.ConstantTimeCompare(claimedPublicKey, identity.PublicKey) != 1 {
		return GatewayIdentity{}, hubauth.Challenge{}, nil, ErrPublicKeyMismatch
	}
	challenge, pending, err := hubauth.NewGatewayChallenge(identity.NodeID, hubInstanceID, challengeID, identity.PublicKey, channelBinding, a.now(), a.ttl)
	if err != nil {
		return GatewayIdentity{}, hubauth.Challenge{}, nil, err
	}
	return identity, challenge, pending, nil
}

// ClientAuthenticator validates the active device and user before issuing an
// X25519 challenge. Its proof transcript is distinct from Gateway auth.
type ClientAuthenticator struct {
	directory ClientDirectory
	ttl       time.Duration
	now       func() time.Time
}

func NewClientAuthenticator(directory ClientDirectory, ttl time.Duration) (*ClientAuthenticator, error) {
	if directory == nil || ttl <= 0 {
		return nil, errors.New("Client directory and positive authentication challenge TTL are required")
	}
	return &ClientAuthenticator{directory: directory, ttl: ttl, now: time.Now}, nil
}

func (a *ClientAuthenticator) Begin(ctx context.Context, hubInstanceID string, claimedPublicKey, challengeID, channelBinding []byte) (ClientIdentity, hubauth.Challenge, *hubauth.PendingChallenge, error) {
	if a == nil || a.directory == nil || len(claimedPublicKey) != 32 {
		return ClientIdentity{}, hubauth.Challenge{}, nil, errors.New("Client X25519 identity is invalid")
	}
	identity, err := a.directory.LookupClientByPublicKey(ctx, claimedPublicKey)
	if err != nil {
		if errors.Is(err, ErrClientNotFound) {
			return ClientIdentity{}, hubauth.Challenge{}, nil, ErrClientNotRegistered
		}
		return ClientIdentity{}, hubauth.Challenge{}, nil, fmt.Errorf("load Client identity: %w", err)
	}
	if identity.NodeID == "" {
		return ClientIdentity{}, hubauth.Challenge{}, nil, ErrClientNotFound
	}
	if identity.NodeStatus == "pending" {
		return identity, hubauth.Challenge{}, nil, ErrClientApprovalPending
	}
	if identity.NodeStatus != "active" || len(identity.PublicKey) != 32 || subtle.ConstantTimeCompare(claimedPublicKey, identity.PublicKey) != 1 {
		return ClientIdentity{}, hubauth.Challenge{}, nil, ErrClientNotAllowed
	}
	challenge, pending, err := hubauth.NewClientChallenge(identity.NodeID, hubInstanceID, challengeID, identity.PublicKey, channelBinding, a.now(), a.ttl)
	if err != nil {
		return ClientIdentity{}, hubauth.Challenge{}, nil, err
	}
	return identity, challenge, pending, nil
}
