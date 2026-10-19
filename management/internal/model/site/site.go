// Package site contains API view models for network sites.
//
// A site is persisted as a t_ztna_network row with network_type='site'. These
// structs are response/input shapes only; IPAM Network is the persistence
// model and therefore remains the single source of truth.
package site

import "time"

const (
	StatusActive   = "active"
	StatusDisabled = "disabled"
)

type PhysicalSite struct {
	ID            int64       `json:"id"`
	VersionNo     int64       `json:"version_no"`
	SiteCode      string      `json:"site_code"`
	Name          string      `json:"name"`
	GatewayNodeID *int64      `json:"gateway_node_id,omitempty"`
	GatewayName   string      `json:"gateway_name"`
	Status        string      `json:"status"`
	CreatedAt     time.Time   `json:"created_at"`
	NetworkID     int64       `json:"network_id"`
	NetworkCode   string      `json:"network_code"`
	LocalSubnet   string      `json:"local_subnet"`
	NodeCount     int64       `json:"node_count"`
	NATMapping    *NATMapping `json:"nat_mapping,omitempty"`
}

type NATMapping struct {
	ID          int64   `json:"id"`
	VersionNo   int64   `json:"version_no"`
	Enabled     bool    `json:"enabled"`
	VirtualCIDR *string `json:"virtual_cidr,omitempty"`
	NATType     string  `json:"nat_type"`
	Description *string `json:"description,omitempty"`
}

type GatewayOption struct {
	ID          int64  `json:"id"`
	GatewayID   string `json:"gateway_id"`
	GatewayName string `json:"gateway_name"`
	Status      string `json:"status"`
}
