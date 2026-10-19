// Package ipam contains persistent enterprise network catalog models.
package ipam

import (
	"time"

	"velotrust/shared/repository/database"
)

const (
	TableNetwork   = "t_ztna_network"
	StatusActive   = "active"
	StatusDisabled = "disabled"
)

// Network describes an enterprise routed network. It is distinct from an
// overlay address pool used to allocate WireGuard tunnel addresses.
type Network struct {
	database.DeletableModel `xorm:"extends"`
	ID                      int64   `xorm:"pk autoincr 'id'" json:"id"`
	NetworkName             string  `xorm:"'network_name'" json:"network_name"`
	NetworkCode             string  `xorm:"'network_code'" json:"network_code"`
	NetworkType             string  `xorm:"'network_type'" json:"network_type"`
	CIDR                    string  `xorm:"'cidr'" json:"cidr"`
	GatewayNodeID           *int64  `xorm:"'gateway_node_id'" json:"gateway_node_id,omitempty"`
	Region                  *string `xorm:"'region'" json:"region,omitempty"`
	Environment             *string `xorm:"'environment'" json:"environment,omitempty"`
	Description             *string `xorm:"'description'" json:"description,omitempty"`
	Status                  string  `xorm:"'status'" json:"status"`
}

func (Network) TableName() string { return TableNetwork }

type AuditEvent struct {
	EventID     string
	ActorID     int64
	Action      string
	TargetType  string
	TargetID    string
	TargetName  string
	ChangesJSON []byte
	OccurredAt  time.Time
}

type Pool struct {
	database.BaseModel `xorm:"extends"`
	ID                 int64   `xorm:"pk autoincr 'id'" json:"id"`
	PoolName           string  `xorm:"'pool_name'" json:"pool_name"`
	NetworkCIDR        string  `xorm:"'network_cidr'" json:"cidr"`
	Type               string  `xorm:"'pool_type'" json:"type"`
	AllocationMode     string  `xorm:"'allocation_mode'" json:"allocation_mode"`
	IDPGroup           *string `xorm:"'idp_group'" json:"idp_group,omitempty"`
	GatewayBinding     *string `xorm:"'gateway_binding'" json:"gateway_binding,omitempty"`
	Color              string  `xorm:"'color'" json:"color"`
	Description        *string `xorm:"'description'" json:"description,omitempty"`
	Status             string  `xorm:"'status'" json:"status"`
	Used               int64   `xorm:"-" json:"used"`
	Total              int64   `xorm:"-" json:"total"`
}

func (Pool) TableName() string { return "t_ztna_address_pool" }

type Lease struct {
	database.BaseModel `xorm:"extends"`
	ID                 int64      `xorm:"pk autoincr 'id'" json:"id"`
	PoolID             int64      `xorm:"'pool_id'" json:"pool_id"`
	VirtualIP          string     `xorm:"'address'" json:"virtual_ip"`
	NodeType           string     `xorm:"'node_type'" json:"node_type"`
	TargetName         string     `xorm:"'target_name'" json:"target_name"`
	Endpoint           string     `xorm:"'endpoint'" json:"endpoint"`
	AllocationMode     string     `xorm:"'allocation_mode'" json:"allocation_mode"`
	IDPGroup           *string    `xorm:"'idp_group'" json:"idp_group,omitempty"`
	WireGuardPubKey    string     `xorm:"-" json:"wireguard_pubkey"`
	WireGuardPubKeyRaw *string    `xorm:"'wireguard_pubkey'" json:"-"`
	Status             string     `xorm:"'status'" json:"status"`
	PoolName           string     `xorm:"-" json:"pool_name"`
	AllocatedAt        time.Time  `xorm:"'allocated_at'" json:"allocated_at"`
	ReleasedAt         *time.Time `xorm:"'released_at'" json:"released_at,omitempty"`
	LastHandshake      string     `xorm:"-" json:"last_handshake"`
}

func (Lease) TableName() string { return "t_ztna_address_lease" }

type NATMapping struct {
	database.BaseModel `xorm:"extends"`
	ID                 int64   `xorm:"pk autoincr 'id'" json:"id"`
	NetworkID          int64   `xorm:"'network_id'" json:"network_id"`
	NetworkName        string  `xorm:"'network_name'" json:"network_name"`
	RealCIDR           string  `xorm:"'real_cidr'" json:"real_cidr"`
	VirtualCIDR        *string `xorm:"'virtual_cidr'" json:"virtual_cidr,omitempty"`
	NATType            string  `xorm:"'nat_type'" json:"nat_type"`
	Enabled            bool    `xorm:"'enabled'" json:"enabled"`
	Description        *string `xorm:"'description'" json:"description,omitempty"`
}

func (NATMapping) TableName() string { return "t_ztna_nat_mapping" }

type OverlayNetwork struct {
	database.BaseModel `xorm:"extends"`
	ID                 int16  `xorm:"pk 'id'" json:"id"`
	NetworkCIDR        string `xorm:"'network_cidr'" json:"cidr"`
}

func (OverlayNetwork) TableName() string { return "t_ztna_overlay_network" }

type Overview struct {
	GlobalCIDR           string  `json:"global_cidr"`
	GlobalNetworkVersion int64   `json:"global_network_version"`
	TotalIPs             int64   `json:"total_ips"`
	UsedIPs              int64   `json:"used_ips"`
	FreeIPs              int64   `json:"free_ips"`
	Pools                []Pool  `json:"ipam_pools"`
	Leases               []Lease `json:"active_leases"`
}

type Allocation struct {
	TargetName      string
	NodeType        string
	PoolID          int64
	RequestedIP     string
	IDPGroup        string
	WireGuardPubKey string
	Endpoint        string
	ActorID         int64
}

type Conflict struct {
	IsValid         bool   `json:"isValid"`
	HasConflict     bool   `json:"hasConflict"`
	Message         string `json:"message"`
	ConflictTarget  string `json:"conflictTarget,omitempty"`
	IsWithinOverlay bool   `json:"isWithinOverlay,omitempty"`
}

var (
	_ database.IModel = (*Network)(nil)
	_ database.IModel = (*Pool)(nil)
	_ database.IModel = (*Lease)(nil)
	_ database.IModel = (*NATMapping)(nil)
	_ database.IModel = (*OverlayNetwork)(nil)
)
