package ipam

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/netip"
	"regexp"
	"strconv"
	"strings"
	"time"

	servicecontext "velotrust/management/internal/context"
	model "velotrust/management/internal/model/ipam"
	"velotrust/management/internal/repository/ipam"
	appErrors "velotrust/pkg/errors"
)

const (
	readPermission  = "ipam.network.read"
	writePermission = "ipam.network.write"
)

var networkCodePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{1,63}$`)
var poolColorPattern = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)

type PermissionChecker interface {
	HasPermission(context.Context, int64, string) (bool, error)
}

type Service struct {
	store       ipam.Store
	permissions PermissionChecker
}

type CreateNetworkInput struct {
	NetworkName string
	NetworkCode string
	NetworkType string
	CIDR        string
	Region      string
	Environment string
	Description string
}

func New(store ipam.Store, permissions PermissionChecker) (*Service, error) {
	if store == nil || permissions == nil {
		return nil, errors.New("IPAM store and IAM permission checker are required")
	}
	return &Service{store: store, permissions: permissions}, nil
}

func (s *Service) ListNetworks(ctx *servicecontext.Context) ([]model.Network, error) {
	if _, err := s.authorize(ctx, readPermission); err != nil {
		return nil, err
	}
	return s.store.ListNetworks(ctx, false)
}

func (s *Service) CreateNetwork(ctx *servicecontext.Context, input CreateNetworkInput) (*model.Network, error) {
	actorID, err := s.authorize(ctx, writePermission)
	if err != nil {
		return nil, err
	}
	input.NetworkName = strings.TrimSpace(input.NetworkName)
	input.NetworkCode = strings.TrimSpace(input.NetworkCode)
	input.NetworkType = strings.TrimSpace(input.NetworkType)
	input.Region = strings.TrimSpace(input.Region)
	input.Environment = strings.TrimSpace(input.Environment)
	input.Description = strings.TrimSpace(input.Description)
	if input.NetworkName == "" || len(input.NetworkName) > 128 {
		return nil, invalid("network_name", "required and at most 128 characters")
	}
	if !networkCodePattern.MatchString(input.NetworkCode) {
		return nil, invalid("network_code", "must be 2 to 64 lowercase letters, digits, dots, underscores, or hyphens")
	}
	switch input.NetworkType {
	case "vpc", "subnet", "site", "segment":
	default:
		return nil, invalid("network_type", "must be vpc, subnet, site, or segment")
	}
	if len(input.Region) > 128 || len(input.Environment) > 32 || len(input.Description) > 512 {
		return nil, invalid("network_metadata", "region, environment, or description is too long")
	}
	prefix, err := netip.ParsePrefix(input.CIDR)
	if err != nil || prefix.Bits() == 0 {
		return nil, invalid("cidr", "must be a valid non-default IPv4 or IPv6 network prefix")
	}
	input.CIDR = prefix.Masked().String()
	network := &model.Network{
		NetworkName: input.NetworkName, NetworkCode: input.NetworkCode, NetworkType: input.NetworkType,
		CIDR: input.CIDR, Region: stringPointer(input.Region), Environment: stringPointer(input.Environment),
		Description: stringPointer(input.Description), Status: model.StatusActive,
	}
	changes, _ := json.Marshal(map[string]string{"network_code": input.NetworkCode, "network_type": input.NetworkType, "cidr": input.CIDR})
	audit, err := newAudit(actorID, "ipam.network.create", network.NetworkName, changes)
	if err != nil {
		return nil, err
	}
	if err := s.store.CreateNetwork(ctx, actorID, network, audit); err != nil {
		return nil, err
	}
	return network, nil
}

func (s *Service) SetNetworkStatus(ctx *servicecontext.Context, networkID int64, status string, expectedVersion int64) error {
	actorID, err := s.authorize(ctx, writePermission)
	if err != nil {
		return err
	}
	if networkID <= 0 || expectedVersion <= 0 || (status != model.StatusActive && status != model.StatusDisabled) {
		return invalid("network_status", "a valid network, version, and status are required")
	}
	changes, _ := json.Marshal(map[string]string{"status": status})
	audit, err := newAudit(actorID, "ipam.network.status", "", changes)
	if err != nil {
		return err
	}
	return s.store.SetNetworkStatus(ctx, actorID, networkID, status, expectedVersion, audit)
}

func (s *Service) ListSelectableNetworks(ctx *servicecontext.Context) ([]model.Network, error) {
	if _, err := s.authorize(ctx, readPermission); err != nil {
		return nil, err
	}
	return s.store.ListNetworks(ctx, true)
}

// ListSelectableSiteNetworks returns active physical-site CIDRs for gateway
// registration. These are routed LAN prefixes, not Overlay address pools.
func (s *Service) ListSelectableSiteNetworks(ctx *servicecontext.Context) ([]model.Network, error) {
	if _, err := s.authorize(ctx, readPermission); err != nil {
		return nil, err
	}
	return s.store.ListActiveSiteNetworks(ctx)
}

func (s *Service) Overview(ctx *servicecontext.Context) (*model.Overview, error) {
	if _, err := s.authorize(ctx, readPermission); err != nil {
		return nil, err
	}
	pools, err := s.store.ListPools(ctx)
	if err != nil {
		return nil, err
	}
	leases, err := s.store.ListLeases(ctx)
	if err != nil {
		return nil, err
	}
	overlayNetwork, err := s.store.GetOverlayNetwork(ctx)
	if err != nil {
		return nil, err
	}
	result := &model.Overview{Pools: pools}
	if overlayNetwork != nil {
		result.GlobalCIDR = overlayNetwork.NetworkCIDR
		result.GlobalNetworkVersion = overlayNetwork.VersionNo
	}
	activePoolIDs := make(map[int64]struct{}, len(pools))
	for i := range result.Pools {
		capacity, parseErr := allocatableCapacity(result.Pools[i].NetworkCIDR)
		if parseErr != nil {
			return nil, fmt.Errorf("invalid stored IPAM pool %q: %w", result.Pools[i].PoolName, parseErr)
		}
		result.Pools[i].Total = capacity
		if result.Pools[i].Status == model.StatusActive {
			activePoolIDs[result.Pools[i].ID] = struct{}{}
			result.TotalIPs += capacity
		}
	}
	active := make([]model.Lease, 0, len(leases))
	for _, lease := range leases {
		_, poolIsActive := activePoolIDs[lease.PoolID]
		if poolIsActive && (lease.Status == "Active" || lease.Status == "Reserved") {
			active = append(active, lease)
			result.UsedIPs++
		}
	}
	result.Leases = active
	result.FreeIPs = result.TotalIPs - result.UsedIPs
	if result.FreeIPs < 0 {
		result.FreeIPs = 0
	}
	return result, nil
}

func (s *Service) ListPools(ctx *servicecontext.Context) ([]model.Pool, error) {
	if _, err := s.authorize(ctx, readPermission); err != nil {
		return nil, err
	}
	pools, err := s.store.ListPools(ctx)
	if err != nil {
		return nil, err
	}
	for i := range pools {
		pools[i].Total, err = allocatableCapacity(pools[i].NetworkCIDR)
		if err != nil {
			return nil, fmt.Errorf("invalid stored IPAM pool %q: %w", pools[i].PoolName, err)
		}
	}
	return pools, nil
}

func (s *Service) SaveOverlayNetwork(ctx *servicecontext.Context, cidr string, expectedVersion int64) (*model.OverlayNetwork, error) {
	actorID, err := s.authorize(ctx, writePermission)
	if err != nil {
		return nil, err
	}
	if expectedVersion < 0 {
		return nil, invalid("expected_version", "must not be negative")
	}
	prefix, err := netip.ParsePrefix(strings.TrimSpace(cidr))
	if err != nil || !prefix.Addr().Is4() || prefix.Bits() < 8 || prefix.Bits() > 16 || prefix.Addr().As4()[0] != 10 {
		return nil, invalid("cidr", "must be an IPv4 private 10.0.0.0/8 prefix between /8 and /16")
	}
	cidr = prefix.Masked().String()
	changes, _ := json.Marshal(map[string]string{"cidr": cidr})
	audit, err := newAudit(actorID, "ipam.overlay_network.save", "Global Overlay", changes)
	if err != nil {
		return nil, err
	}
	return s.store.SaveOverlayNetwork(ctx, actorID, cidr, expectedVersion, audit)
}

func (s *Service) CreatePool(ctx *servicecontext.Context, pool model.Pool) (*model.Pool, error) {
	actorID, err := s.authorize(ctx, writePermission)
	if err != nil {
		return nil, err
	}
	if err := validatePool(&pool); err != nil {
		return nil, err
	}
	changes, _ := json.Marshal(map[string]string{"cidr": pool.NetworkCIDR, "type": pool.Type, "allocation_mode": pool.AllocationMode})
	audit, err := newAudit(actorID, "ipam.pool.create", pool.PoolName, changes)
	if err != nil {
		return nil, err
	}
	if err := s.store.CreatePool(ctx, actorID, &pool, audit); err != nil {
		return nil, err
	}
	pool.Total, _ = allocatableCapacity(pool.NetworkCIDR)
	return &pool, nil
}

func (s *Service) UpdatePool(ctx *servicecontext.Context, pool model.Pool) (*model.Pool, error) {
	actorID, err := s.authorize(ctx, writePermission)
	if err != nil {
		return nil, err
	}
	if pool.ID <= 0 || pool.VersionNo <= 0 {
		return nil, invalid("pool", "an existing pool ID and version are required")
	}
	if err := validatePool(&pool); err != nil {
		return nil, err
	}
	changes, _ := json.Marshal(map[string]string{"cidr": pool.NetworkCIDR, "type": pool.Type, "allocation_mode": pool.AllocationMode})
	audit, err := newAudit(actorID, "ipam.pool.update", pool.PoolName, changes)
	if err != nil {
		return nil, err
	}
	if err := s.store.UpdatePool(ctx, actorID, &pool, audit); err != nil {
		return nil, err
	}
	pool.VersionNo++
	pool.Total, _ = allocatableCapacity(pool.NetworkCIDR)
	return &pool, nil
}

func (s *Service) SetPoolStatus(ctx *servicecontext.Context, poolID int64, status string, expectedVersion int64) error {
	actorID, err := s.authorize(ctx, writePermission)
	if err != nil {
		return err
	}
	if poolID <= 0 || expectedVersion <= 0 || (status != model.StatusActive && status != model.StatusDisabled) {
		return invalid("pool_status", "a valid pool, version, and status are required")
	}
	changes, _ := json.Marshal(map[string]string{"status": status})
	audit, err := newAudit(actorID, "ipam.pool.status", "", changes)
	if err != nil {
		return err
	}
	return s.store.SetPoolStatus(ctx, actorID, poolID, status, expectedVersion, audit)
}

func (s *Service) ListLeases(ctx *servicecontext.Context) ([]model.Lease, error) {
	if _, err := s.authorize(ctx, readPermission); err != nil {
		return nil, err
	}
	return s.store.ListLeases(ctx)
}

func (s *Service) AllocateAddress(ctx *servicecontext.Context, input model.Allocation) (*model.Lease, error) {
	actorID, err := s.authorize(ctx, writePermission)
	if err != nil {
		return nil, err
	}
	input.TargetName = strings.TrimSpace(input.TargetName)
	input.NodeType = strings.TrimSpace(input.NodeType)
	input.RequestedIP = strings.TrimSpace(input.RequestedIP)
	input.Endpoint = strings.TrimSpace(input.Endpoint)
	input.IDPGroup = strings.TrimSpace(input.IDPGroup)
	input.WireGuardPubKey = strings.TrimSpace(input.WireGuardPubKey)
	if input.PoolID <= 0 || input.TargetName == "" || len(input.TargetName) > 128 {
		return nil, invalid("target", "pool and target name are required; target name is at most 128 characters")
	}
	switch input.NodeType {
	case "Gateway", "Peer", "Server":
	default:
		return nil, invalid("node_type", "must be Gateway, Peer, or Server")
	}
	if len(input.Endpoint) > 255 || len(input.IDPGroup) > 128 {
		return nil, invalid("allocation", "endpoint or identity group is too long")
	}
	if input.WireGuardPubKey != "" {
		key, decodeErr := base64.StdEncoding.DecodeString(input.WireGuardPubKey)
		if decodeErr != nil || len(key) != 32 {
			return nil, invalid("wireguard_pubkey", "must be a valid 32-byte WireGuard public key in base64")
		}
		input.WireGuardPubKey = base64.StdEncoding.EncodeToString(key)
	}
	pools, err := s.store.ListPools(ctx)
	if err != nil {
		return nil, err
	}
	var selected *model.Pool
	for i := range pools {
		if pools[i].ID == input.PoolID {
			selected = &pools[i]
			break
		}
	}
	if selected == nil || selected.Status != model.StatusActive {
		return nil, invalid("pool_id", "selected address pool is missing or disabled")
	}
	switch selected.AllocationMode {
	case "static":
		if input.RequestedIP == "" {
			return nil, invalid("requested_ip", "a static address pool requires an explicit IPv4 address")
		}
		address, parseErr := netip.ParseAddr(input.RequestedIP)
		prefix, prefixErr := netip.ParsePrefix(selected.NetworkCIDR)
		if parseErr != nil || !address.Is4() || prefixErr != nil || !prefix.Contains(address) || address == prefix.Masked().Addr() || address == lastIPv4Address(prefix.Masked()) {
			return nil, invalid("requested_ip", "must be an available host address inside the selected pool")
		}
		input.RequestedIP = address.String()
	case "dynamic":
		if input.RequestedIP != "" {
			return nil, invalid("requested_ip", "dynamic pools automatically choose the next free address")
		}
	default:
		return nil, invalid("pool_id", "selected address pool has an unsupported allocation mode")
	}
	group := input.IDPGroup
	if group == "" && selected.IDPGroup != nil {
		group = *selected.IDPGroup
	}
	lease := &model.Lease{PoolID: input.PoolID, VirtualIP: input.RequestedIP, NodeType: input.NodeType, TargetName: input.TargetName, Endpoint: input.Endpoint, AllocationMode: selected.AllocationMode, WireGuardPubKey: input.WireGuardPubKey}
	lease.IDPGroup = stringPointer(group)
	changes, _ := json.Marshal(map[string]string{"target_name": input.TargetName, "pool_id": strconv.FormatInt(input.PoolID, 10), "node_type": input.NodeType})
	audit, err := newAudit(actorID, "ipam.lease.allocate", input.TargetName, changes)
	if err != nil {
		return nil, err
	}
	if err := s.store.AllocateAddress(ctx, actorID, lease, audit); err != nil {
		return nil, err
	}
	return lease, nil
}

func (s *Service) ReleaseLease(ctx *servicecontext.Context, leaseID int64) error {
	actorID, err := s.authorize(ctx, writePermission)
	if err != nil {
		return err
	}
	if leaseID <= 0 {
		return invalid("lease_id", "a valid lease ID is required")
	}
	audit, err := newAudit(actorID, "ipam.lease.release", "", []byte(`{"status":"released"}`))
	if err != nil {
		return err
	}
	return s.store.ReleaseLease(ctx, actorID, leaseID, audit)
}

func (s *Service) CheckPrefix(ctx *servicecontext.Context, cidr string) (model.Conflict, error) {
	if _, err := s.authorize(ctx, readPermission); err != nil {
		return model.Conflict{}, err
	}
	return s.store.CheckPrefix(ctx, cidr)
}

func (s *Service) ListNATMappings(ctx *servicecontext.Context) ([]model.NATMapping, error) {
	if _, err := s.authorize(ctx, readPermission); err != nil {
		return nil, err
	}
	return s.store.ListNATMappings(ctx)
}

func (s *Service) SaveNATMapping(ctx *servicecontext.Context, mapping model.NATMapping) (*model.NATMapping, error) {
	actorID, err := s.authorize(ctx, writePermission)
	if err != nil {
		return nil, err
	}
	mapping.NetworkName = strings.TrimSpace(mapping.NetworkName)
	mapping.RealCIDR = strings.TrimSpace(mapping.RealCIDR)
	mapping.NATType = strings.TrimSpace(mapping.NATType)
	if mapping.NetworkID <= 0 {
		return nil, invalid("network_id", "an IPAM business network is required")
	}
	switch mapping.NATType {
	case "1:1_Bidirectional", "SNAT_Masquerade", "DNAT_VirtualIP":
	default:
		return nil, invalid("nat_type", "unsupported NAT mapping type")
	}
	if mapping.VirtualCIDR != nil {
		value := strings.TrimSpace(*mapping.VirtualCIDR)
		if value == "" {
			mapping.VirtualCIDR = nil
		} else {
			prefix, parseErr := netip.ParsePrefix(value)
			if parseErr != nil || prefix.Bits() == 0 {
				return nil, invalid("virtual_cidr", "must be a valid non-default network prefix")
			}
			value = prefix.Masked().String()
			mapping.VirtualCIDR = &value
		}
	}
	if mapping.NATType != "SNAT_Masquerade" && mapping.VirtualCIDR == nil {
		return nil, invalid("virtual_cidr", "is required for this NAT mode")
	}
	if mapping.NATType == "1:1_Bidirectional" && mapping.VirtualCIDR != nil {
		networks, listErr := s.store.ListNetworks(ctx, true)
		if listErr != nil {
			return nil, listErr
		}
		var realBits = -1
		for _, network := range networks {
			if network.ID == mapping.NetworkID {
				p, _ := netip.ParsePrefix(network.CIDR)
				realBits = p.Bits()
				break
			}
		}
		virtual, _ := netip.ParsePrefix(*mapping.VirtualCIDR)
		if realBits < 0 || virtual.Bits() != realBits {
			return nil, invalid("virtual_cidr", "1:1 NAT requires matching real and virtual prefix lengths")
		}
	}
	description := ""
	if mapping.Description != nil {
		description = strings.TrimSpace(*mapping.Description)
	}
	if len(description) > 512 {
		return nil, invalid("description", "must be at most 512 characters")
	}
	mapping.Description = stringPointer(description)
	changes, _ := json.Marshal(map[string]interface{}{"network_id": mapping.NetworkID, "nat_type": mapping.NATType, "virtual_cidr": mapping.VirtualCIDR, "enabled": mapping.Enabled})
	audit, err := newAudit(actorID, "ipam.nat_mapping.save", mapping.NetworkName, changes)
	if err != nil {
		return nil, err
	}
	if err := s.store.SaveNATMapping(ctx, actorID, &mapping, audit); err != nil {
		return nil, err
	}
	return &mapping, nil
}

func validatePool(pool *model.Pool) error {
	pool.PoolName = strings.TrimSpace(pool.PoolName)
	pool.NetworkCIDR = strings.TrimSpace(pool.NetworkCIDR)
	pool.Type = strings.TrimSpace(pool.Type)
	pool.AllocationMode = strings.TrimSpace(pool.AllocationMode)
	pool.Color = strings.TrimSpace(pool.Color)
	if pool.PoolName == "" || len(pool.PoolName) > 128 {
		return invalid("pool_name", "required and at most 128 characters")
	}
	prefix, err := netip.ParsePrefix(pool.NetworkCIDR)
	if err != nil || !prefix.Addr().Is4() || prefix.Bits() < 16 || prefix.Bits() > 30 {
		return invalid("cidr", "must be an IPv4 pool between /16 and /30")
	}
	pool.NetworkCIDR = prefix.Masked().String()
	switch pool.Type {
	case "Gateway", "User-Device", "Service-VIP":
	default:
		return invalid("type", "must be Gateway, User-Device, or Service-VIP")
	}
	switch pool.AllocationMode {
	case "static", "dynamic":
	default:
		return invalid("allocation_mode", "must be static or dynamic")
	}
	if pool.Color == "" {
		pool.Color = "#10b981"
	}
	if !poolColorPattern.MatchString(pool.Color) {
		return invalid("color", "must be a six-digit HTML color such as #10b981")
	}
	pool.Color = strings.ToLower(pool.Color)
	if pool.IDPGroup != nil {
		value := strings.TrimSpace(*pool.IDPGroup)
		pool.IDPGroup = stringPointer(value)
		if len(value) > 128 {
			return invalid("idp_group", "must be at most 128 characters")
		}
	}
	if pool.GatewayBinding != nil {
		value := strings.TrimSpace(*pool.GatewayBinding)
		pool.GatewayBinding = stringPointer(value)
		if len(value) > 128 {
			return invalid("gateway_binding", "must be at most 128 characters")
		}
	}
	if pool.Description != nil {
		value := strings.TrimSpace(*pool.Description)
		pool.Description = stringPointer(value)
		if len(value) > 512 {
			return invalid("description", "must be at most 512 characters")
		}
	}
	return nil
}

func allocatableCapacity(cidr string) (int64, error) {
	prefix, err := netip.ParsePrefix(cidr)
	if err != nil || !prefix.Addr().Is4() || prefix.Bits() < 16 || prefix.Bits() > 30 {
		return 0, errors.New("expected an IPv4 pool between /16 and /30")
	}
	return int64((uint64(1) << uint(32-prefix.Bits())) - 2), nil
}

func lastIPv4Address(prefix netip.Prefix) netip.Addr {
	bytes := prefix.Addr().As4()
	address := uint64(bytes[0])<<24 | uint64(bytes[1])<<16 | uint64(bytes[2])<<8 | uint64(bytes[3])
	hostCount := uint64(1) << uint(32-prefix.Bits())
	last := address + hostCount - 1
	return netip.AddrFrom4([4]byte{byte(last >> 24), byte(last >> 16), byte(last >> 8), byte(last)})
}

func (s *Service) authorize(ctx *servicecontext.Context, permission string) (int64, error) {
	if ctx == nil {
		return 0, appErrors.NewAuthError("IAM_UNAUTHENTICATED", appErrors.AuthUnauthenticated)
	}
	actorID, err := strconv.ParseInt(ctx.UserID(), 10, 64)
	if err != nil || actorID <= 0 {
		return 0, appErrors.NewAuthError("IAM_UNAUTHENTICATED", appErrors.AuthUnauthenticated)
	}
	allowed, err := s.permissions.HasPermission(ctx, actorID, permission)
	if err != nil {
		return 0, fmt.Errorf("check IPAM permission: %w", err)
	}
	if !allowed {
		return 0, appErrors.NewAuthError("IAM_PERMISSION_DENIED", appErrors.AuthPermissionDenied)
	}
	return actorID, nil
}

func newAudit(actorID int64, action, targetName string, changes []byte) (*model.AuditEvent, error) {
	var id [16]byte
	if _, err := rand.Read(id[:]); err != nil {
		return nil, fmt.Errorf("generate IPAM audit event ID: %w", err)
	}
	return &model.AuditEvent{EventID: hex.EncodeToString(id[:]), ActorID: actorID, Action: action, TargetName: targetName, ChangesJSON: changes, OccurredAt: time.Now().UTC()}, nil
}

func stringPointer(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}

func invalid(field, message string) error {
	return appErrors.NewClientError("IPAM_INVALID_ARGUMENT", map[string]string{field: message})
}
