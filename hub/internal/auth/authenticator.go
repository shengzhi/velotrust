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
)

// GatewayIdentity is the minimum Management-owned node state HUB needs to
// authorize a Gateway control stream. PublicKey is the registered WireGuard
// X25519 key, encoded as its raw 32-byte value.
type GatewayIdentity struct {
	NodeID     string
	NodeStatus string
	PublicKey  []byte
}

// GatewayDirectory reads the current authoritative Gateway identity and
// lifecycle state. Implementations belong to HUB's repository layer.
type GatewayDirectory interface {
	LookupGatewayByPublicKey(context.Context, []byte) (GatewayIdentity, error)
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
