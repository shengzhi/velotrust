package ztna

import "time"

// VirtualNodeView is a read projection for the virtual-node management API.
// It intentionally contains public identity and runtime metadata only.
type VirtualNodeView struct {
	ID                 int64                `json:"id"`
	VersionNo          int64                `json:"version_no"`
	NodeName           string               `json:"node_name"`
	NodeCode           string               `json:"node_code"`
	NodeType           string               `json:"node_type"`
	NodeStatus         string               `json:"node_status"`
	ConnectionStatus   string               `json:"connection_status"`
	AgentVersion       *string              `json:"agent_version,omitempty"`
	ControlEndpoint    *string              `json:"control_endpoint,omitempty"`
	PublicEndpoint     *string              `json:"public_endpoint,omitempty"`
	PrivateEndpoint    *string              `json:"private_endpoint,omitempty"`
	IPAMAddress        *string              `json:"ipam_address,omitempty"`
	PublicKey          *string              `json:"public_key,omitempty"`
	LastHeartbeatAt    *time.Time           `json:"last_heartbeat_at,omitempty"`
	LastConfigHash     *string              `json:"last_config_hash,omitempty"`
	ActualConfigHash   *string              `json:"actual_config_hash,omitempty"`
	Capabilities       map[string]any       `json:"capabilities"`
	HealthStatus       string               `json:"health_status"`
	GatewayType        *string              `json:"gateway_type,omitempty"`
	GatewayNetworkName *string              `json:"gateway_network_name,omitempty"`
	RequireApproval    bool                 `json:"require_approval"`
	RegisteredAt       *time.Time           `json:"registered_at,omitempty"`
	CreatedAt          time.Time            `json:"created_at"`
	Networks           []VirtualNodeNetwork `json:"networks"`
	Peers              []VirtualNodePeer    `json:"peers"`
	DeviceID           *int64               `json:"device_id,omitempty"`
	DeviceName         *string              `json:"device_name,omitempty"`
	DeviceType         *string              `json:"device_type,omitempty"`
	DevicePlatform     *string              `json:"device_platform,omitempty"`
	DeviceStatus       *string              `json:"device_status,omitempty"`
	OwnerUserID        *int64               `json:"owner_user_id,omitempty"`
	OwnerUsername      *string              `json:"owner_username,omitempty"`
}

type VirtualNodeNetwork struct {
	ID     int64  `json:"id"`
	Name   string `json:"network_name"`
	Code   string `json:"network_code"`
	Type   string `json:"network_type"`
	CIDR   string `json:"cidr"`
	Status string `json:"status"`
}

type VirtualNodePeer struct {
	ID              int64      `json:"id"`
	TunnelID        int64      `json:"tunnel_id"`
	TunnelCode      string     `json:"tunnel_code"`
	TunnelIP        string     `json:"tunnel_ip"`
	AllowedIPs      []string   `json:"allowed_ips"`
	Endpoint        *string    `json:"endpoint,omitempty"`
	Status          string     `json:"status"`
	LastHandshakeAt *time.Time `json:"last_handshake_at,omitempty"`
}

type GatewayTunnelOption struct {
	ID          int64  `xorm:"'id'" json:"id"`
	Code        string `xorm:"'code'" json:"code"`
	NetworkName string `xorm:"'network_name'" json:"network_name,omitempty"`
}
