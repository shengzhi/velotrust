package postgres

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"

	"velotrust/hub/internal/auth"
	"velotrust/shared/repository/database"
)

// GatewayDirectory reads Gateway identity and lifecycle data from the
// Management-owned node inventory. It performs no lifecycle mutations.
type GatewayDirectory struct {
	db *database.Database
}

func NewGatewayDirectory(db *database.Database) (*GatewayDirectory, error) {
	if db == nil || db.Engine() == nil {
		return nil, errors.New("HUB database is required")
	}
	if db.Driver() != "postgres" {
		return nil, fmt.Errorf("HUB node directory requires PostgreSQL, got %q", db.Driver())
	}
	return &GatewayDirectory{db: db}, nil
}

func (r *GatewayDirectory) LookupGatewayByPublicKey(ctx context.Context, claimedPublicKey []byte) (auth.GatewayIdentity, error) {
	if ctx == nil {
		return auth.GatewayIdentity{}, errors.New("request context is required")
	}
	if len(claimedPublicKey) != 32 {
		return auth.GatewayIdentity{}, errors.New("Gateway X25519 public key must contain 32 bytes")
	}
	var row struct {
		NodeID     string `xorm:"'node_code'"`
		NodeStatus string `xorm:"'node_status'"`
		PublicKey  string `xorm:"'public_key'"`
	}
	session := r.db.Engine().NewSession().Context(ctx)
	defer session.Close()
	found, err := session.SQL(`
		SELECT node_code, node_status, public_key
		FROM public.t_ztna_node
		WHERE public_key = ? AND node_type = 'gateway'
	`, base64.StdEncoding.EncodeToString(claimedPublicKey)).Get(&row)
	if err != nil {
		return auth.GatewayIdentity{}, fmt.Errorf("query Gateway node directory: %w", err)
	}
	if !found {
		return auth.GatewayIdentity{}, auth.ErrGatewayNotFound
	}
	var publicKey []byte
	if row.PublicKey != "" {
		publicKey, err = base64.StdEncoding.DecodeString(row.PublicKey)
		if err != nil || len(publicKey) != 32 {
			return auth.GatewayIdentity{}, errors.New("registered Gateway X25519 public key is invalid")
		}
	} else if row.NodeStatus != "pending" {
		return auth.GatewayIdentity{}, errors.New("non-pending Gateway has no X25519 public key")
	}
	return auth.GatewayIdentity{NodeID: row.NodeID, NodeStatus: row.NodeStatus, PublicKey: publicKey}, nil
}
