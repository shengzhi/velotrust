package model

import (
	"time"

	"velotrust/shared/repository/database"
)

type Device struct {
	database.BaseModel `xorm:"extends"`
	ID                 int64      `xorm:"pk autoincr 'id'"`
	DeviceName         string     `xorm:"'device_name'"`
	DeviceUUID         string     `xorm:"'device_uuid'"`
	DeviceType         string     `xorm:"'device_type'"`
	Platform           string     `xorm:"'platform'"`
	AgentVersion       *string    `xorm:"'agent_version'"`
	NodeID             *int64     `xorm:"'node_id'"`
	OwnerID            *int64     `xorm:"'owner_user_id'"`
	Status             string     `xorm:"'status'"`
	TrustStatus        string     `xorm:"'trust_status'"`
	RegisteredAt       *time.Time `xorm:"'registered_at'"`
	LastSeenAt         *time.Time `xorm:"'last_seen_at'"`
	LastSeenIP         *string    `xorm:"'last_seen_ip'"`
}

func (Device) TableName() string { return "t_ztna_device" }

type DeviceCredential struct {
	database.BaseModel `xorm:"extends"`
	ID                 int64     `xorm:"pk autoincr 'id'"`
	DeviceID           int64     `xorm:"'device_id'"`
	CredentialType     string    `xorm:"'credential_type'"`
	PublicKey          string    `xorm:"'public_key'"`
	Fingerprint        string    `xorm:"'fingerprint'"`
	KeyVersion         int       `xorm:"'key_version'"`
	Status             string    `xorm:"'status'"`
	NotBefore          time.Time `xorm:"'not_before'"`
}

func (DeviceCredential) TableName() string { return "t_ztna_device_credential" }

type TunnelPeer struct {
	database.BaseModel  `xorm:"extends"`
	ID                  int64      `xorm:"pk autoincr 'id'"`
	TunnelID            int64      `xorm:"'tunnel_id'"`
	NodeID              int64      `xorm:"'node_id'"`
	DeviceID            *int64     `xorm:"'device_id'"`
	PublicKey           string     `xorm:"'public_key'"`
	TunnelIP            string     `xorm:"'tunnel_ip'"`
	AllowedIPs          []byte     `xorm:"'allowed_ips'"`
	Endpoint            *string    `xorm:"'endpoint'"`
	PersistentKeepalive int        `xorm:"'persistent_keepalive'"`
	Status              string     `xorm:"'status'"`
	KeyVersion          int        `xorm:"'key_version'"`
	LastHandshakeAt     *time.Time `xorm:"'last_handshake_at'"`
	LastSeenAt          *time.Time `xorm:"'last_seen_at'"`
}

func (TunnelPeer) TableName() string { return "t_ztna_tunnel_peer" }

type AddressLease struct {
	database.BaseModel `xorm:"extends"`
	ID                 int64      `xorm:"pk autoincr 'id'"`
	PoolID             int64      `xorm:"'pool_id'"`
	Address            string     `xorm:"'address'"`
	NodeType           string     `xorm:"'node_type'"`
	TargetName         string     `xorm:"'target_name'"`
	Endpoint           *string    `xorm:"'endpoint'"`
	AllocationMode     string     `xorm:"'allocation_mode'"`
	IDPGroup           *string    `xorm:"'idp_group'"`
	WireGuardPublicKey *string    `xorm:"'wireguard_pubkey'"`
	Status             string     `xorm:"'status'"`
	AllocatedAt        time.Time  `xorm:"'allocated_at'"`
	ReleasedAt         *time.Time `xorm:"'released_at'"`
}

func (AddressLease) TableName() string { return "t_ztna_address_lease" }

var (
	_ database.IModel = (*Device)(nil)
	_ database.IModel = (*DeviceCredential)(nil)
	_ database.IModel = (*TunnelPeer)(nil)
	_ database.IModel = (*AddressLease)(nil)
)
