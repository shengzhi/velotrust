package postgres

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math/big"
	"net/netip"
	"sort"
	"strings"
	"time"

	"velotrust/hub/internal/configsync"
	"velotrust/hub/internal/model"
	"velotrust/shared/configpayload"
	"velotrust/shared/repository/database"
	"xorm.io/xorm"
)

const maxGatewayConfigPayload = 2 << 20

// GatewayConfigurationSource compiles a desired Gateway snapshot from the
// current ZTNA, tunnel, and IPAM tables. Configuration tasks retain delivery
// state; they do not store a second copy of desired configuration.
type GatewayConfigurationSource struct {
	db         *database.Database
	actor      string
	instanceID string
}

func NewGatewayConfigurationSource(db *database.Database, instanceID string) (*GatewayConfigurationSource, error) {
	if db == nil || db.Engine() == nil {
		return nil, errors.New("HUB database is required")
	}
	if db.Driver() != "postgres" {
		return nil, fmt.Errorf("HUB configuration source requires PostgreSQL, got %q", db.Driver())
	}
	if strings.TrimSpace(instanceID) == "" {
		return nil, errors.New("HUB instance ID is required")
	}
	return &GatewayConfigurationSource{db: db, actor: auditActor(instanceID), instanceID: instanceID}, nil
}

type gatewaySnapshotRow struct {
	NodeID                int64   `xorm:"'node_id'"`
	NodeCode              string  `xorm:"'node_code'"`
	TunnelAddress         *string `xorm:"'tunnel_address'"`
	PeersJSON             []byte  `xorm:"'peers_json'"`
	AuthorizedClientsJSON []byte  `xorm:"'authorized_clients_json'"`
	RoutesJSON            []byte  `xorm:"'routes_json'"`
	NATJSON               []byte  `xorm:"'nat_json'"`
	OverlayCIDR           *string `xorm:"'overlay_cidr'"`
}

type gatewaySnapshotPayload struct {
	SchemaVersion uint32               `json:"schema_version"`
	Tunnel        gatewayTunnelPayload `json:"tunnel"`
	Routes        []string             `json:"routes"`
	NAT           []gatewayNATPayload  `json:"nat"`
	Rules         []gatewayRulePayload `json:"rules"`
}

type gatewayTunnelPayload struct {
	Address string               `json:"address"`
	Peers   []gatewayPeerPayload `json:"peers"`
}

type gatewayPeerPayload struct {
	ID                  string   `json:"id"`
	PeerName            string   `json:"peer_name"`
	PublicKey           string   `json:"public_key"`
	Endpoint            string   `json:"endpoint,omitempty"`
	AllowedIPs          []string `json:"allowed_ips"`
	PersistentKeepalive uint16   `json:"persistent_keepalive,omitempty"`
}

type gatewayNATPayload struct {
	Type              string `json:"type"`
	ShadowPrefix      string `json:"shadow_prefix,omitempty"`
	RealPrefix        string `json:"real_prefix,omitempty"`
	SourcePrefix      string `json:"source_prefix,omitempty"`
	DestinationPrefix string `json:"destination_prefix,omitempty"`
}

type gatewayRulePayload struct {
	ID                string `json:"id"`
	Priority          uint32 `json:"priority"`
	SourcePrefix      string `json:"source_prefix"`
	DestinationPrefix string `json:"destination_prefix"`
	Protocol          string `json:"protocol"`
	PortFrom          uint16 `json:"port_from,omitempty"`
	PortTo            uint16 `json:"port_to,omitempty"`
	Action            string `json:"action"`
}

type storedPolicyRule struct {
	ID       string `json:"id"`
	Priority uint32 `json:"priority"`
	Action   string `json:"action"`
}

type orderedGatewayRule struct {
	policyID       int64
	policyPriority int
	rulePriority   uint32
	ruleID         string
	source         string
	destination    string
	protocol       string
	portFrom       uint16
	portTo         uint16
	action         string
}

func (r *GatewayConfigurationSource) Current(ctx context.Context, nodeCode, appliedHash string) (configsync.Snapshot, bool, error) {
	if ctx == nil || strings.TrimSpace(nodeCode) == "" {
		return configsync.Snapshot{}, false, errors.New("request context and node code are required")
	}
	if err := r.reconcileClientAddress(ctx, nodeCode); err != nil {
		return configsync.Snapshot{}, false, fmt.Errorf("reconcile Client Overlay address before configuration: %w", err)
	}
	s := r.db.Engine().NewSession().Context(ctx)
	defer s.Close()
	payload, nodeID, err := compileNodeSnapshot(s, nodeCode)
	if errors.Is(err, configsync.ErrClientAddressNotAssigned) {
		slog.Info("HUB is withholding Client tunnel configuration because automatic Overlay address assignment is pending", "node_id", nodeCode, "reason", err)
		return configsync.Snapshot{}, false, nil
	}
	if errors.Is(err, configsync.ErrClientConfigurationNotReady) {
		slog.Info("HUB is withholding Client tunnel configuration because no policy-authorized Gateway configuration is available", "node_id", nodeCode, "reason", err)
		return configsync.Snapshot{}, false, nil
	}
	if err != nil {
		return configsync.Snapshot{}, false, err
	}
	hash, err := configpayload.Hash(payload)
	if err != nil {
		return configsync.Snapshot{}, false, fmt.Errorf("hash compiled node configuration: %w", err)
	}
	snapshot := configsync.Snapshot{Revision: hash, Payload: payload}
	if strings.EqualFold(hash, appliedHash) {
		return snapshot, false, nil
	}
	snapshot.TaskID, err = r.ensureTask(ctx, nodeCode, nodeID, hash, appliedHash)
	if err != nil {
		return configsync.Snapshot{}, false, err
	}
	return snapshot, true, nil
}

func compileNodeSnapshot(s *xorm.Session, nodeCode string) ([]byte, int64, error) {
	var nodeType string
	found, err := s.SQL(`SELECT node_type FROM public.t_ztna_node WHERE node_code=? AND node_status='active'`, nodeCode).Get(&nodeType)
	if err != nil {
		return nil, 0, fmt.Errorf("resolve configuration node type: %w", err)
	}
	if !found {
		return nil, 0, fmt.Errorf("%w: active node was not found", configsync.ErrTargetUnavailable)
	}
	switch nodeType {
	case "gateway":
		return compileGatewaySnapshot(s, nodeCode)
	case "client_agent":
		return compileClientSnapshot(s, nodeCode)
	default:
		return nil, 0, fmt.Errorf("%w: node type %q does not receive configuration", configsync.ErrTargetUnavailable, nodeType)
	}
}

func (r *GatewayConfigurationSource) AcknowledgeCurrentConfig(ctx context.Context, nodeCode string) error {
	repository, err := NewAccessChangeRepository(r.db, r.instanceID)
	if err != nil {
		return err
	}
	return repository.AcknowledgeCurrentConfig(ctx, nodeCode)
}

func (r *GatewayConfigurationSource) RecordAccessChangeFailure(ctx context.Context, nodeCode, message string) error {
	repository, err := NewAccessChangeRepository(r.db, r.instanceID)
	if err != nil {
		return err
	}
	return repository.RecordFailure(ctx, nodeCode, message)
}

// compileGatewaySnapshot reads all configuration inputs in one SQL statement
// so one snapshot cannot mix rows from different PostgreSQL statement
// snapshots. A Gateway's associated site CIDRs are forwarding destinations on
// its local network, not routes through the WireGuard interface.
func compileGatewaySnapshot(s *xorm.Session, nodeCode string) ([]byte, int64, error) {
	var row gatewaySnapshotRow
	found, err := s.SQL(`
		SELECT node.id AS node_id, node.node_code,
		       (SELECT host(lease.address) || '/' || masklen(lease.address)::text
		          FROM public.t_ztna_address_lease AS lease
		         WHERE lease.wireguard_pubkey=node.public_key
		           AND lease.node_type='Gateway'
		           AND lease.status IN ('active', 'reserved')
	         ORDER BY lease.id DESC LIMIT 1) AS tunnel_address,
	       COALESCE((
			SELECT jsonb_agg(jsonb_build_object(
				'id', peer.id,
				'peer_name', peer_node.node_name,
				'public_key', peer.public_key,
				'endpoint', COALESCE(NULLIF(peer.endpoint, ''), NULLIF(peer_node.public_endpoint, ''), ''),
				'tunnel_ip', peer.tunnel_ip::text,
				'allowed_ips', peer.allowed_ips,
				'persistent_keepalive', peer.persistent_keepalive
			) ORDER BY peer.id)
			FROM public.t_ztna_tunnel_peer AS membership
			JOIN public.t_ztna_tunnel AS tunnel
			  ON tunnel.id=membership.tunnel_id AND tunnel.status='active'
			JOIN public.t_ztna_tunnel_peer AS peer
			  ON peer.tunnel_id=tunnel.id AND peer.status='active' AND peer.node_id<>node.id
			JOIN public.t_ztna_node AS peer_node
			  ON peer_node.id=peer.node_id AND peer_node.node_status='active'
			LEFT JOIN LATERAL (
				SELECT device.id, device.status, device.owner_user_id,
				       EXISTS (
						SELECT 1 FROM public.t_ztna_device_credential AS credential
						WHERE credential.device_id=device.id AND credential.public_key=peer_node.public_key
						  AND credential.status='active' AND credential.not_before <= CURRENT_TIMESTAMP
						  AND (credential.expires_at IS NULL OR credential.expires_at > CURRENT_TIMESTAMP)
				       ) AS has_active_credential,
				       COUNT(*) OVER () AS device_match_count
				FROM public.t_ztna_device AS device
				WHERE device.id=peer.device_id OR device.node_id=peer_node.id OR EXISTS (
					SELECT 1 FROM public.t_ztna_device_credential AS credential
					WHERE credential.device_id=device.id AND credential.public_key=peer_node.public_key
				)
				ORDER BY CASE WHEN device.id=peer.device_id THEN 0 WHEN device.node_id=peer_node.id THEN 1 ELSE 2 END,
				         device.id
				LIMIT 1
			) AS linked_device ON TRUE
			LEFT JOIN public.t_iam_user AS device_owner ON device_owner.id=linked_device.owner_user_id
			WHERE membership.node_id=node.id AND membership.status='active'
			  AND (
				peer_node.node_type <> 'client_agent'
				OR (linked_device.id IS NOT NULL AND linked_device.device_match_count=1
				    AND linked_device.status='active'
				    AND linked_device.has_active_credential
				    AND linked_device.owner_user_id IS NOT NULL
				    AND device_owner.account_type='employee' AND device_owner.user_status='active'
				    AND EXISTS (
					SELECT 1 FROM public.t_ztna_gateway_group_binding AS binding
					JOIN public.t_iam_user_group AS membership ON membership.group_id=binding.group_id
					JOIN public.t_iam_group AS employee_group ON employee_group.id=membership.group_id AND employee_group.status='active'
					WHERE binding.gateway_node_id=node.id AND membership.user_id=linked_device.owner_user_id
					UNION
					SELECT 1 FROM public.t_ztna_access_policy AS policy
					JOIN public.t_ztna_policy_subject AS subject ON subject.policy_id=policy.id AND subject.subject_type='group'
					JOIN public.t_iam_user_group AS membership ON membership.group_id=subject.subject_id AND membership.user_id=linked_device.owner_user_id
					JOIN public.t_iam_group AS employee_group ON employee_group.id=membership.group_id AND employee_group.status='active'
					JOIN public.t_ztna_policy_resource AS policy_resource ON policy_resource.policy_id=policy.id
					JOIN public.t_ztna_protected_resource AS resource ON resource.id=policy_resource.protected_resource_id AND resource.status='active'
					JOIN public.t_ztna_network AS network ON network.id=resource.network_id
					WHERE policy.status='active' AND network.gateway_node_id=node.id
				))
			  )
		), '[]'::jsonb) AS peers_json,
	       COALESCE((
			SELECT jsonb_agg(jsonb_build_object(
				'id', 'client-device-' || authorized.device_id,
				'peer_name', authorized.peer_name,
				'public_key', authorized.public_key,
				'endpoint', '',
				'tunnel_ip', authorized.tunnel_ip,
				'allowed_ips', jsonb_build_array(authorized.tunnel_ip),
				'persistent_keepalive', 0
			) ORDER BY authorized.device_id)
			FROM (
				SELECT DISTINCT device.id AS device_id, client_node.node_name AS peer_name, client_node.public_key,
				       host(lease.address) || '/' || masklen(lease.address)::text AS tunnel_ip
				FROM (
					SELECT binding.group_id, binding.gateway_node_id FROM public.t_ztna_gateway_group_binding AS binding
					UNION
					SELECT subject.subject_id AS group_id, network.gateway_node_id AS gateway_node_id
					FROM public.t_ztna_access_policy AS policy
					JOIN public.t_ztna_policy_subject AS subject ON subject.policy_id=policy.id AND subject.subject_type='group'
					JOIN public.t_ztna_policy_resource AS policy_resource ON policy_resource.policy_id=policy.id
					JOIN public.t_ztna_protected_resource AS resource ON resource.id=policy_resource.protected_resource_id AND resource.status='active'
					JOIN public.t_ztna_network AS network ON network.id=resource.network_id
						AND network.network_type='site' AND network.status='active' AND network.is_del=FALSE
					WHERE policy.status='active' AND network.gateway_node_id IS NOT NULL
				) AS admission
				JOIN public.t_iam_group AS employee_group
				  ON employee_group.id=admission.group_id AND employee_group.status='active'
				JOIN public.t_iam_user_group AS membership ON membership.group_id=employee_group.id
				JOIN public.t_iam_user AS owner
				  ON owner.id=membership.user_id AND owner.account_type='employee' AND owner.user_status='active'
				JOIN public.t_ztna_device AS device
				  ON device.owner_user_id=owner.id AND device.status='active'
				JOIN public.t_ztna_node AS client_node
				  ON client_node.id=device.node_id AND client_node.node_type='client_agent'
				 AND client_node.node_status='active' AND client_node.public_key IS NOT NULL
				JOIN public.t_ztna_device_credential AS credential
				  ON credential.device_id=device.id AND credential.public_key=client_node.public_key
				 AND credential.credential_type='x25519' AND credential.status='active'
				 AND credential.not_before<=CURRENT_TIMESTAMP
				 AND (credential.expires_at IS NULL OR credential.expires_at>CURRENT_TIMESTAMP)
				JOIN public.t_ztna_address_lease AS lease
				  ON lease.wireguard_pubkey=client_node.public_key AND lease.node_type='Peer' AND lease.status='active'
				WHERE admission.gateway_node_id=node.id
			) AS authorized
		), '[]'::jsonb) AS authorized_clients_json,
	       COALESCE((
			SELECT jsonb_agg(network.cidr::text ORDER BY network.id)
			FROM public.t_ztna_network AS network
			WHERE network.gateway_node_id=node.id AND network.network_type='site'
			  AND network.status='active' AND network.is_del=FALSE
		), '[]'::jsonb) AS routes_json,
	       COALESCE((
			SELECT jsonb_agg(jsonb_build_object(
				'nat_type', mapping.nat_type,
				'real_cidr', mapping.real_cidr::text,
				'virtual_cidr', mapping.virtual_cidr::text
			) ORDER BY mapping.id)
			FROM public.t_ztna_nat_mapping AS mapping
			JOIN public.t_ztna_network AS network
			  ON network.id=mapping.network_id AND network.gateway_node_id=node.id
			WHERE mapping.enabled=TRUE AND network.network_type='site'
			  AND network.status='active' AND network.is_del=FALSE
		), '[]'::jsonb) AS nat_json,
	       (SELECT overlay.network_cidr::text
	          FROM public.t_ztna_overlay_network AS overlay WHERE overlay.id=1) AS overlay_cidr
		FROM public.t_ztna_node AS node
		WHERE node.node_code=? AND node.node_type='gateway' AND node.node_status='active'
	`, nodeCode).Get(&row)
	if err != nil {
		return nil, 0, fmt.Errorf("read Gateway business configuration: %w", err)
	}
	if !found {
		return nil, 0, fmt.Errorf("%w: active Gateway was not found", configsync.ErrTargetUnavailable)
	}
	rules, err := compileGatewayRules(s, row.NodeID)
	if err != nil {
		return nil, 0, fmt.Errorf("%w: compile Gateway ACL rules: %v", configsync.ErrInvalidConfiguration, err)
	}
	if row.TunnelAddress == nil || strings.TrimSpace(*row.TunnelAddress) == "" {
		return nil, 0, fmt.Errorf("%w: Gateway has no active IPAM tunnel-address lease", configsync.ErrInvalidConfiguration)
	}
	address, err := canonicalPrefix(*row.TunnelAddress)
	if err != nil {
		return nil, 0, fmt.Errorf("%w: invalid Gateway IPAM tunnel address: %v", configsync.ErrInvalidConfiguration, err)
	}

	payload := gatewaySnapshotPayload{
		SchemaVersion: 1,
		Tunnel:        gatewayTunnelPayload{Address: address, Peers: make([]gatewayPeerPayload, 0)},
		Routes:        make([]string, 0),
		NAT:           make([]gatewayNATPayload, 0),
		Rules:         rules,
	}
	var peerRows []struct {
		ID                  int64    `json:"id"`
		PeerName            string   `json:"peer_name"`
		PublicKey           string   `json:"public_key"`
		Endpoint            string   `json:"endpoint"`
		TunnelIP            string   `json:"tunnel_ip"`
		AllowedIPs          []string `json:"allowed_ips"`
		PersistentKeepalive int      `json:"persistent_keepalive"`
	}
	if err := json.Unmarshal(row.PeersJSON, &peerRows); err != nil {
		return nil, 0, fmt.Errorf("%w: decode Gateway tunnel peers: %v", configsync.ErrInvalidConfiguration, err)
	}
	for _, peer := range peerRows {
		if peer.ID <= 0 || strings.TrimSpace(peer.PublicKey) == "" || peer.PersistentKeepalive < 0 || peer.PersistentKeepalive > 65535 {
			return nil, 0, fmt.Errorf("%w: Gateway tunnel peer has invalid identity or keepalive", configsync.ErrInvalidConfiguration)
		}
		peerTunnelIP, err := canonicalPrefix(peer.TunnelIP)
		if err != nil {
			return nil, 0, fmt.Errorf("%w: Gateway peer %d has invalid tunnel IP: %v", configsync.ErrInvalidConfiguration, peer.ID, err)
		}
		allowed := make([]string, 0, len(peer.AllowedIPs)+1)
		seen := make(map[string]struct{}, len(peer.AllowedIPs)+1)
		for _, raw := range append(peer.AllowedIPs, peer.TunnelIP) {
			prefix, err := canonicalPrefix(raw)
			if err != nil {
				return nil, 0, fmt.Errorf("%w: Gateway peer %d has invalid AllowedIPs prefix %q: %v", configsync.ErrInvalidConfiguration, peer.ID, raw, err)
			}
			if _, ok := seen[prefix]; !ok {
				allowed = append(allowed, prefix)
				seen[prefix] = struct{}{}
			}
		}
		if len(allowed) == 0 {
			allowed = append(allowed, peerTunnelIP)
		}
		sort.Strings(allowed)
		payload.Tunnel.Peers = append(payload.Tunnel.Peers, gatewayPeerPayload{
			ID: fmt.Sprintf("peer-%d", peer.ID), PeerName: strings.TrimSpace(peer.PeerName), PublicKey: strings.TrimSpace(peer.PublicKey),
			Endpoint: strings.TrimSpace(peer.Endpoint), AllowedIPs: allowed,
			PersistentKeepalive: uint16(peer.PersistentKeepalive),
		})
	}
	var authorizedClients []struct {
		ID                  string   `json:"id"`
		PeerName            string   `json:"peer_name"`
		PublicKey           string   `json:"public_key"`
		Endpoint            string   `json:"endpoint"`
		TunnelIP            string   `json:"tunnel_ip"`
		AllowedIPs          []string `json:"allowed_ips"`
		PersistentKeepalive int      `json:"persistent_keepalive"`
	}
	if err := json.Unmarshal(row.AuthorizedClientsJSON, &authorizedClients); err != nil {
		return nil, 0, fmt.Errorf("%w: decode Gateway security-group authorized Clients: %v", configsync.ErrInvalidConfiguration, err)
	}
	knownPublicKeys := make(map[string]struct{}, len(peerRows)+len(authorizedClients))
	for _, peer := range peerRows {
		knownPublicKeys[strings.TrimSpace(peer.PublicKey)] = struct{}{}
	}
	for _, peer := range authorizedClients {
		publicKey := strings.TrimSpace(peer.PublicKey)
		if publicKey == "" {
			return nil, 0, fmt.Errorf("%w: authorized Client has no WireGuard public key", configsync.ErrInvalidConfiguration)
		}
		if _, exists := knownPublicKeys[publicKey]; exists {
			continue
		}
		knownPublicKeys[publicKey] = struct{}{}
		peerTunnelIP, err := canonicalPrefix(peer.TunnelIP)
		if err != nil {
			return nil, 0, fmt.Errorf("%w: authorized Client %s has invalid tunnel IP: %v", configsync.ErrInvalidConfiguration, peer.ID, err)
		}
		payload.Tunnel.Peers = append(payload.Tunnel.Peers, gatewayPeerPayload{
			ID: peer.ID, PeerName: strings.TrimSpace(peer.PeerName), PublicKey: publicKey,
			AllowedIPs: []string{peerTunnelIP},
		})
	}
	sort.Slice(payload.Tunnel.Peers, func(i, j int) bool { return payload.Tunnel.Peers[i].ID < payload.Tunnel.Peers[j].ID })

	var routes []string
	if err := json.Unmarshal(row.RoutesJSON, &routes); err != nil {
		return nil, 0, fmt.Errorf("%w: decode Gateway associated networks: %v", configsync.ErrInvalidConfiguration, err)
	}
	for _, route := range routes {
		prefix, err := canonicalPrefix(route)
		if err != nil {
			return nil, 0, fmt.Errorf("%w: invalid associated network route %q: %v", configsync.ErrInvalidConfiguration, route, err)
		}
		payload.Routes = append(payload.Routes, prefix)
	}
	sort.Strings(payload.Routes)
	if len(payload.Routes) > 1 {
		uniqueRoutes := payload.Routes[:1]
		for _, route := range payload.Routes[1:] {
			if route != uniqueRoutes[len(uniqueRoutes)-1] {
				uniqueRoutes = append(uniqueRoutes, route)
			}
		}
		payload.Routes = uniqueRoutes
	}

	// For every active network associated with this Gateway, Masquerade client
	// Overlay addresses as traffic leaves the Gateway for the private site. This
	// lets the site return traffic without requiring a route back to the Overlay
	// range. The current IPAM Overlay range is IPv4-only, so IPv6 sites cannot be
	// safely published until IPv6 Overlay addressing is implemented.
	if len(payload.Routes) > 0 {
		if row.OverlayCIDR == nil || strings.TrimSpace(*row.OverlayCIDR) == "" {
			return nil, 0, fmt.Errorf("%w: automatic site SNAT requires the global Overlay network", configsync.ErrInvalidConfiguration)
		}
		overlay, err := canonicalPrefix(*row.OverlayCIDR)
		if err != nil {
			return nil, 0, fmt.Errorf("%w: invalid global Overlay network: %v", configsync.ErrInvalidConfiguration, err)
		}
		for _, route := range payload.Routes {
			sitePrefix, _ := netip.ParsePrefix(route)
			if !sitePrefix.Addr().Is4() {
				return nil, 0, fmt.Errorf("%w: automatic site SNAT currently requires IPv4 Overlay and IPv4 associated networks", configsync.ErrInvalidConfiguration)
			}
			payload.NAT = append(payload.NAT, gatewayNATPayload{
				Type: "masquerade", SourcePrefix: overlay,
				DestinationPrefix: sitePrefix.Masked().String(),
			})
		}
	}

	var natRows []struct {
		Type        string  `json:"nat_type"`
		RealCIDR    string  `json:"real_cidr"`
		VirtualCIDR *string `json:"virtual_cidr"`
	}
	if err := json.Unmarshal(row.NATJSON, &natRows); err != nil {
		return nil, 0, fmt.Errorf("%w: decode Gateway NAT mappings: %v", configsync.ErrInvalidConfiguration, err)
	}
	for _, mapping := range natRows {
		real, err := canonicalPrefix(mapping.RealCIDR)
		if err != nil {
			return nil, 0, fmt.Errorf("%w: invalid NAT real network %q: %v", configsync.ErrInvalidConfiguration, mapping.RealCIDR, err)
		}
		switch mapping.Type {
		case "1:1_Bidirectional", "DNAT_VirtualIP":
			if mapping.VirtualCIDR == nil {
				return nil, 0, fmt.Errorf("%w: NAT mapping for %s requires a virtual CIDR", configsync.ErrInvalidConfiguration, real)
			}
			virtual, err := canonicalPrefix(*mapping.VirtualCIDR)
			if err != nil {
				return nil, 0, fmt.Errorf("%w: invalid NAT virtual network %q: %v", configsync.ErrInvalidConfiguration, *mapping.VirtualCIDR, err)
			}
			realPrefix, _ := netip.ParsePrefix(real)
			virtualPrefix, _ := netip.ParsePrefix(virtual)
			if realPrefix.Addr().Is4() != virtualPrefix.Addr().Is4() || realPrefix.Bits() != virtualPrefix.Bits() {
				return nil, 0, fmt.Errorf("%w: NAT real and virtual prefixes must have matching address families and prefix lengths", configsync.ErrInvalidConfiguration)
			}
			payload.NAT = append(payload.NAT, gatewayNATPayload{Type: "dnat", ShadowPrefix: virtual, RealPrefix: real})
		case "SNAT_Masquerade":
			// Legacy mapping rows used to request per-site SNAT. SNAT is now
			// generated from every active Gateway-associated site above, so this
			// record adds no second or duplicate rule.
		default:
			return nil, 0, fmt.Errorf("%w: unsupported NAT type %q", configsync.ErrInvalidConfiguration, mapping.Type)
		}
	}

	encoded, err := json.Marshal(payload)
	if err != nil {
		return nil, 0, fmt.Errorf("encode compiled Gateway configuration: %w", err)
	}
	if len(encoded) == 0 || len(encoded) > maxGatewayConfigPayload {
		return nil, 0, fmt.Errorf("%w: compiled Gateway configuration exceeds %d bytes", configsync.ErrInvalidConfiguration, maxGatewayConfigPayload)
	}
	return encoded, row.NodeID, nil
}

func canonicalPrefix(raw string) (string, error) {
	prefix, err := netip.ParsePrefix(strings.TrimSpace(raw))
	if err != nil || !prefix.IsValid() || prefix.Addr().Is4In6() || prefix.Addr().IsUnspecified() || prefix.Addr().IsMulticast() {
		return "", errors.New("must be a valid unicast IP prefix")
	}
	return prefix.Masked().String(), nil
}

func compileGatewayRules(s *xorm.Session, gatewayNodeID int64) ([]gatewayRulePayload, error) {
	if s == nil {
		return nil, errors.New("database session is required")
	}
	if gatewayNodeID <= 0 {
		return nil, errors.New("Gateway node ID must be positive")
	}
	var policies []struct {
		ID       int64  `xorm:"'id'"`
		Priority int    `xorm:"'priority'"`
		Rules    []byte `xorm:"'rules_json'"`
	}
	if err := s.SQL(`SELECT id, priority, rules_json FROM public.t_ztna_access_policy
		WHERE status='active' ORDER BY priority, id`).Find(&policies); err != nil {
		return nil, fmt.Errorf("load active access policies: %w", err)
	}
	ordered := make([]orderedGatewayRule, 0)
	for _, policy := range policies {
		if policy.ID <= 0 || policy.Priority < 0 {
			return nil, errors.New("active policy has an invalid identity or priority")
		}
		var subjects []struct {
			Type string `xorm:"'subject_type'"`
			ID   int64  `xorm:"'subject_id'"`
		}
		if err := s.SQL(`SELECT subject_type, subject_id FROM public.t_ztna_policy_subject
			WHERE policy_id=? ORDER BY subject_type, subject_id`, policy.ID).Find(&subjects); err != nil {
			return nil, fmt.Errorf("load subjects for active policy %d: %w", policy.ID, err)
		}
		var resourceRows []struct {
			ID int64 `xorm:"'protected_resource_id'"`
		}
		if err := s.SQL(`SELECT protected_resource_id FROM public.t_ztna_policy_resource
			WHERE policy_id=? ORDER BY protected_resource_id`, policy.ID).Find(&resourceRows); err != nil {
			return nil, fmt.Errorf("load resources for active policy %d: %w", policy.ID, err)
		}
		resourceIDs := make([]int64, 0, len(resourceRows))
		for _, resource := range resourceRows {
			resourceIDs = append(resourceIDs, resource.ID)
		}
		if len(subjects) == 0 || len(resourceIDs) == 0 {
			continue
		}
		sources := make(map[string]struct{})
		for _, subject := range subjects {
			if subject.Type != "group" || subject.ID <= 0 {
				return nil, fmt.Errorf("active policy %d contains unsupported subject type %q", policy.ID, subject.Type)
			}
			var addressRows []struct {
				Prefix string `xorm:"'prefix'"`
			}
			if err := s.SQL(`SELECT DISTINCT host(lease.address) || '/' || masklen(lease.address)::text AS prefix
				FROM public.t_iam_group AS employee_group
				JOIN public.t_iam_user_group AS membership ON membership.group_id=employee_group.id
				JOIN public.t_iam_user AS owner ON owner.id=membership.user_id
				  AND owner.account_type='employee' AND owner.user_status='active'
				JOIN public.t_ztna_device AS device ON device.owner_user_id=owner.id AND device.status='active'
				JOIN public.t_ztna_node AS client ON client.id=device.node_id
				  AND client.node_type='client_agent' AND client.node_status='active' AND client.public_key IS NOT NULL
				JOIN public.t_ztna_device_credential AS credential ON credential.device_id=device.id
				  AND credential.credential_type='x25519' AND credential.public_key=client.public_key
				  AND credential.status='active' AND credential.not_before<=CURRENT_TIMESTAMP
				  AND (credential.expires_at IS NULL OR credential.expires_at>CURRENT_TIMESTAMP)
				JOIN public.t_ztna_address_lease AS lease ON lease.wireguard_pubkey=client.public_key
				  AND lease.node_type='Peer' AND lease.status='active'
				WHERE employee_group.id=? AND employee_group.status='active'
				ORDER BY prefix`, subject.ID).Find(&addressRows); err != nil {
				return nil, fmt.Errorf("resolve active Client addresses for policy %d group %d: %w", policy.ID, subject.ID, err)
			}
			for _, address := range addressRows {
				prefix, err := canonicalACLPrefix(address.Prefix)
				if err != nil {
					return nil, fmt.Errorf("policy %d group %d has an invalid Client address", policy.ID, subject.ID)
				}
				sources[prefix] = struct{}{}
			}
		}
		if len(sources) == 0 {
			continue
		}
		var rules []storedPolicyRule
		if err := json.Unmarshal(policy.Rules, &rules); err != nil {
			return nil, fmt.Errorf("decode current rules for policy %d: %w", policy.ID, err)
		}
		if len(rules) != 1 {
			return nil, fmt.Errorf("active policy %d must have exactly one authorization outcome", policy.ID)
		}
		for _, resourceID := range resourceIDs {
			resource, found, err := loadACLResource(s, resourceID)
			if err != nil {
				return nil, fmt.Errorf("load policy %d resource %d: %w", policy.ID, resourceID, err)
			}
			if !found || resource.Status != "active" {
				continue
			}
			if resource.GatewayNodeID != gatewayNodeID {
				continue
			}
			destination, err := resolveACLResourcePrefix(resource)
			if err != nil {
				return nil, fmt.Errorf("policy %d resource %d: %w", policy.ID, resourceID, err)
			}
			protocol, portFrom, portTo, err := aclResourceTransport(resource)
			if err != nil {
				return nil, fmt.Errorf("policy %d resource %d: %w", policy.ID, resourceID, err)
			}
			for _, rule := range rules {
				if rule.ID == "" || len(rule.ID) > 64 || (rule.Action != "allow" && rule.Action != "deny") {
					return nil, fmt.Errorf("policy %d contains an invalid authorization rule", policy.ID)
				}
				for source := range sources {
					ordered = append(ordered, orderedGatewayRule{
						policyID: policy.ID, policyPriority: policy.Priority, rulePriority: rule.Priority, ruleID: rule.ID,
						source: source, destination: destination, protocol: protocol, portFrom: portFrom, portTo: portTo, action: rule.Action,
					})
				}
			}
		}
	}
	if len(ordered) > 4096 {
		return nil, errors.New("Gateway ACL exceeds the 4096-rule limit")
	}
	sort.SliceStable(ordered, func(i, j int) bool {
		left, right := ordered[i], ordered[j]
		if left.policyPriority != right.policyPriority {
			return left.policyPriority < right.policyPriority
		}
		if left.policyID != right.policyID {
			return left.policyID < right.policyID
		}
		if left.rulePriority != right.rulePriority {
			return left.rulePriority < right.rulePriority
		}
		if left.ruleID != right.ruleID {
			return left.ruleID < right.ruleID
		}
		if left.source != right.source {
			return left.source < right.source
		}
		return left.destination < right.destination
	})

	compiled := make([]gatewayRulePayload, 0, len(ordered))
	seenIDs := make(map[string]struct{}, len(ordered))
	for index, entry := range ordered {
		source, err := canonicalACLPrefix(entry.source)
		if err != nil {
			return nil, fmt.Errorf("policy %d rule %q has an invalid source prefix", entry.policyID, entry.ruleID)
		}
		destination, err := canonicalACLPrefix(entry.destination)
		if err != nil {
			return nil, fmt.Errorf("policy %d rule %q has an invalid destination prefix", entry.policyID, entry.ruleID)
		}
		sourcePrefix, _ := netip.ParsePrefix(source)
		destinationPrefix, _ := netip.ParsePrefix(destination)
		if sourcePrefix.Addr().Is4() != destinationPrefix.Addr().Is4() {
			return nil, fmt.Errorf("policy %d rule %q mixes IPv4 and IPv6 prefixes", entry.policyID, entry.ruleID)
		}
		compiledID := fmt.Sprintf("policy-%d-%s-%d", entry.policyID, entry.ruleID, index+1)
		if _, exists := seenIDs[compiledID]; exists {
			return nil, fmt.Errorf("compiled ACL rule ID %q is duplicated", compiledID)
		}
		seenIDs[compiledID] = struct{}{}
		compiled = append(compiled, gatewayRulePayload{
			ID: compiledID, Priority: uint32(index + 1), SourcePrefix: source,
			DestinationPrefix: destination, Protocol: entry.protocol,
			PortFrom: entry.portFrom, PortTo: entry.portTo, Action: entry.action,
		})
	}
	return compiled, nil
}

type aclResource struct {
	ID             int64   `xorm:"'id'"`
	Address        *string `xorm:"'address'"`
	NetworkCIDR    string  `xorm:"'network_cidr'"`
	GatewayNodeID  int64   `xorm:"'gateway_node_id'"`
	Protocol       *string `xorm:"'protocol'"`
	PortStart      *int    `xorm:"'port_start'"`
	PortEnd        *int    `xorm:"'port_end'"`
	Status         string  `xorm:"'status'"`
	NATRealCIDR    *string `xorm:"'real_cidr'"`
	NATVirtualCIDR *string `xorm:"'virtual_cidr'"`
}

func loadACLResource(s *xorm.Session, resourceID int64) (aclResource, bool, error) {
	var resource aclResource
	found, err := s.SQL(`SELECT resource.id, resource.address, network.cidr::text AS network_cidr,
		network.gateway_node_id AS gateway_node_id,
		resource.protocol, resource.port_start, resource.port_end, resource.status,
		mapping.real_cidr::text AS real_cidr, mapping.virtual_cidr::text AS virtual_cidr
		FROM public.t_ztna_protected_resource AS resource
		JOIN public.t_ztna_network AS network ON network.id=resource.network_id
		LEFT JOIN public.t_ztna_nat_mapping AS mapping ON mapping.network_id=network.id AND mapping.enabled=TRUE
		WHERE resource.id=? AND network.network_type='site' AND network.status='active' AND network.is_del=FALSE`, resourceID).Get(&resource)
	return resource, found, err
}

func resolveACLResourcePrefix(resource aclResource) (string, error) {
	var target netip.Prefix
	if resource.Address == nil || strings.TrimSpace(*resource.Address) == "" {
		return "", errors.New("has no resource service address")
	} else if prefix, err := netip.ParsePrefix(strings.TrimSpace(*resource.Address)); err == nil && !prefix.Addr().Is4In6() {
		target = prefix.Masked()
	} else if address, err := netip.ParseAddr(strings.TrimSpace(*resource.Address)); err == nil && !address.Is4In6() {
		bits := 128
		if address.Is4() {
			bits = 32
		}
		target = netip.PrefixFrom(address, bits)
	} else {
		return "", errors.New("address must be an IP address or CIDR")
	}
	real, err := netip.ParsePrefix(strings.TrimSpace(resource.NetworkCIDR))
	if err != nil || real.Addr().Is4In6() {
		return "", errors.New("belongs to a site with an invalid real CIDR")
	}
	real = real.Masked()
	if prefixWithin(target, real) {
		return target.String(), nil
	}
	if resource.NATVirtualCIDR == nil || resource.NATRealCIDR == nil || strings.TrimSpace(*resource.NATVirtualCIDR) == "" || strings.TrimSpace(*resource.NATRealCIDR) == "" {
		if prefixesOverlap(target, real) {
			return "", errors.New("partially overlaps its site CIDR and cannot be authorized safely")
		}
		return "", errors.New("address is outside the site CIDR and no active NAT mapping covers it")
	}
	virtual, err := netip.ParsePrefix(*resource.NATVirtualCIDR)
	if err != nil {
		return "", errors.New("has an invalid NAT shadow CIDR")
	}
	realMapping, err := netip.ParsePrefix(*resource.NATRealCIDR)
	if err != nil {
		return "", errors.New("has an invalid NAT real CIDR")
	}
	virtual = virtual.Masked()
	realMapping = realMapping.Masked()
	if prefixWithin(target, realMapping) {
		return target.String(), nil
	}
	if !prefixWithin(target, virtual) {
		if prefixesOverlap(target, virtual) || prefixesOverlap(target, realMapping) {
			return "", errors.New("partially overlaps a NAT mapping and cannot be translated safely")
		}
		return "", errors.New("address is outside the site's real and NAT shadow CIDRs")
	}
	if target.Addr().Is4() != realMapping.Addr().Is4() || virtual.Addr().Is4() != realMapping.Addr().Is4() || virtual.Bits() != realMapping.Bits() {
		return "", errors.New("NAT shadow and real CIDRs are incompatible")
	}
	virtualBase, targetBase, realBase := addrBigInt(virtual.Addr()), addrBigInt(target.Addr()), addrBigInt(realMapping.Addr())
	translated := new(big.Int).Add(realBase, new(big.Int).Sub(targetBase, virtualBase))
	address, ok := bigIntAddr(translated, target.Addr().Is4())
	if !ok {
		return "", errors.New("translated resource address overflows its address family")
	}
	translatedPrefix := netip.PrefixFrom(address, target.Bits()).Masked()
	if !prefixWithin(translatedPrefix, realMapping) {
		return "", errors.New("translated resource prefix exceeds the NAT real CIDR")
	}
	return translatedPrefix.String(), nil
}

func aclResourceTransport(resource aclResource) (string, uint16, uint16, error) {
	if resource.Protocol == nil {
		return "", 0, 0, errors.New("must define a transport protocol")
	}
	protocol := strings.ToLower(strings.TrimSpace(*resource.Protocol))
	switch protocol {
	case "http", "https":
		protocol = "tcp"
	case "tcp", "udp":
	default:
		return "", 0, 0, errors.New("uses an unsupported transport protocol")
	}
	if resource.PortStart == nil || resource.PortEnd == nil {
		return "", 0, 0, errors.New("has no configured service port range")
	}
	if *resource.PortStart < 1 || *resource.PortEnd < *resource.PortStart || *resource.PortEnd > 65535 {
		return "", 0, 0, errors.New("has an invalid service port range")
	}
	return protocol, uint16(*resource.PortStart), uint16(*resource.PortEnd), nil
}

func prefixWithin(child, parent netip.Prefix) bool {
	return child.IsValid() && parent.IsValid() && child.Addr().Is4() == parent.Addr().Is4() && child.Bits() >= parent.Bits() && parent.Contains(child.Addr())
}

func prefixesOverlap(left, right netip.Prefix) bool {
	return left.IsValid() && right.IsValid() && left.Addr().Is4() == right.Addr().Is4() && (left.Contains(right.Addr()) || right.Contains(left.Addr()))
}

func addrBigInt(address netip.Addr) *big.Int {
	bytes := address.AsSlice()
	return new(big.Int).SetBytes(bytes)
}

func bigIntAddr(value *big.Int, ipv4 bool) (netip.Addr, bool) {
	if value.Sign() < 0 {
		return netip.Addr{}, false
	}
	length := 16
	if ipv4 {
		length = 4
	}
	bytes := value.Bytes()
	if len(bytes) > length {
		return netip.Addr{}, false
	}
	buffer := make([]byte, length)
	copy(buffer[length-len(bytes):], bytes)
	if ipv4 {
		return netip.AddrFrom4([4]byte(buffer)), true
	}
	return netip.AddrFrom16([16]byte(buffer)), true
}

func canonicalACLPrefix(raw string) (string, error) {
	prefix, err := netip.ParsePrefix(strings.TrimSpace(raw))
	if err != nil || !prefix.IsValid() || prefix.Addr().Is4In6() {
		return "", errors.New("must be a valid IP network prefix")
	}
	return prefix.Masked().String(), nil
}

func (r *GatewayConfigurationSource) ensureTask(ctx context.Context, nodeCode string, nodeID int64, configHash, appliedHash string) (string, error) {
	var taskID string
	err := r.db.WithSaveTransaction(ctx, func(tx *database.DBTransaction) error {
		node, err := lockActiveNode(tx, nodeCode)
		if err != nil {
			return err
		}
		if node.ID != nodeID {
			return errors.New("node target changed while compiling configuration")
		}
		currentPayload, _, err := compileNodeSnapshot(tx.Session, nodeCode)
		if err != nil {
			return err
		}
		currentHash, err := configpayload.Hash(currentPayload)
		if err != nil {
			return fmt.Errorf("hash node configuration in task transaction: %w", err)
		}
		if !strings.EqualFold(currentHash, configHash) {
			return errors.New("node business configuration changed while creating its delivery task")
		}
		var existing model.NodeConfigTask
		found, err := tx.SQL(`
			SELECT * FROM public.t_ztna_node_config_task
			WHERE node_id=? AND task_type='apply_config' AND target_config_hash=?
			ORDER BY id DESC LIMIT 1 FOR UPDATE
		`, nodeID, configHash).Get(&existing)
		if err != nil {
			return fmt.Errorf("lock Gateway configuration delivery task: %w", err)
		}
		if found && existing.Status != "cancelled" && ((existing.Status != "applied" && existing.Status != "verified") || strings.EqualFold(appliedHash, configHash)) {
			taskID = existing.TaskID
			return nil
		}
		generatedID, err := newGatewayConfigTaskID()
		if err != nil {
			return fmt.Errorf("generate Gateway configuration task ID: %w", err)
		}
		hash := configHash
		task := model.NodeConfigTask{
			TaskID: generatedID, NodeID: nodeID, TargetConfigHash: &hash,
			TaskType: "apply_config", Status: "pending",
		}
		task.CreateNew(r.actor)
		if _, err := tx.Save(&task); err != nil {
			return fmt.Errorf("create Gateway configuration delivery task: %w", err)
		}
		status := task.Status
		detail, err := json.Marshal(map[string]string{"target_config_hash": configHash, "source": "business_state"})
		if err != nil {
			return fmt.Errorf("encode Gateway configuration task event: %w", err)
		}
		event := model.NodeConfigTaskEvent{
			TaskID: task.TaskID, NodeID: nodeID, EventType: "task.created",
			ToStatus: &status, DetailJSON: detail, OccurredAt: time.Now().UTC(),
		}
		event.CreateNew(r.actor)
		if _, err := tx.Save(&event); err != nil {
			return fmt.Errorf("record Gateway configuration task creation: %w", err)
		}
		taskID = task.TaskID
		return nil
	})
	if err != nil {
		return "", err
	}
	return taskID, nil
}

func (r *GatewayConfigurationSource) MarkSent(ctx context.Context, nodeCode string, snapshot configsync.Snapshot) error {
	return r.transitionTask(ctx, nodeCode, snapshot.TaskID, snapshot.Revision, "sent", "", "", false)
}

func (r *GatewayConfigurationSource) MarkApplied(ctx context.Context, nodeCode, configHash string) error {
	if ctx == nil || nodeCode == "" || configHash == "" {
		return errors.New("context, Gateway node code, and applied configuration hash are required")
	}
	return r.db.WithSaveTransaction(ctx, func(tx *database.DBTransaction) error {
		node, err := lockActiveNode(tx, nodeCode)
		if err != nil {
			return err
		}
		payload, _, err := compileNodeSnapshot(tx.Session, nodeCode)
		if err != nil {
			return err
		}
		currentHash, err := configpayload.Hash(payload)
		if err != nil {
			return fmt.Errorf("hash current node business configuration: %w", err)
		}
		if !strings.EqualFold(currentHash, configHash) {
			return nil
		}
		var task model.NodeConfigTask
		found, err := tx.SQL(`
			SELECT * FROM public.t_ztna_node_config_task
			WHERE node_id=? AND task_type='apply_config' AND target_config_hash=?
			ORDER BY id DESC LIMIT 1 FOR UPDATE
		`, node.ID, currentHash).Get(&task)
		if err != nil {
			return fmt.Errorf("lock applied Gateway configuration task: %w", err)
		}
		if !found || task.Status == "applied" || task.Status == "verified" || task.Status == "cancelled" {
			return nil
		}
		now := time.Now().UTC()
		task.FinishedAt = &now
		task.LastErrorCode = nil
		task.LastErrorMessage = nil
		actor := nodeAuditActor(node.NodeType, node.NodeCode)
		if actor == "" {
			return errors.New("unsupported node configuration audit principal")
		}
		return r.saveTaskStatus(tx, &task, "applied", "", "", "config.applied", map[string]string{"config_hash": currentHash}, actor)
	})
}

func (r *GatewayConfigurationSource) Acknowledge(ctx context.Context, nodeCode, taskID, configHash string, success bool, errorCode, errorMessage string) error {
	if taskID == "" {
		return nil
	}
	if len(errorCode) > 128 || len(errorMessage) > 512 || strings.ContainsAny(errorCode+errorMessage, "\r\n\x00") {
		return errors.New("Gateway configuration receipt contains invalid error details")
	}
	status := "failed"
	eventType := "config.failed"
	if success {
		status, eventType = "applied", "config.applied"
		errorCode, errorMessage = "", ""
	}
	return r.transitionTask(ctx, nodeCode, taskID, configHash, status, errorCode, errorMessage, true, eventType)
}

func (r *GatewayConfigurationSource) transitionTask(ctx context.Context, nodeCode, taskID, configHash, nextStatus, errorCode, errorMessage string, receipt bool, eventTypes ...string) error {
	if ctx == nil || nodeCode == "" || configHash == "" || (taskID == "" && nextStatus != "sent") {
		return errors.New("context, Gateway identity, task ID, and configuration hash are required")
	}
	return r.db.WithSaveTransaction(ctx, func(tx *database.DBTransaction) error {
		node, err := lockActiveNode(tx, nodeCode)
		if err != nil {
			return err
		}
		actor := r.actor
		if receipt {
			actor = nodeAuditActor(node.NodeType, node.NodeCode)
			if actor == "" {
				return errors.New("unsupported node configuration audit principal")
			}
		}
		payload, _, err := compileNodeSnapshot(tx.Session, nodeCode)
		if err != nil {
			return err
		}
		currentHash, err := configpayload.Hash(payload)
		if err != nil {
			return fmt.Errorf("hash current node business configuration: %w", err)
		}
		if !strings.EqualFold(currentHash, configHash) {
			return errors.New("Gateway configuration receipt is stale")
		}
		if nextStatus == "sent" && (node.LastConfigHash == nil || !strings.EqualFold(*node.LastConfigHash, configHash)) {
			expectedHash := configHash
			node.LastConfigHash = &expectedHash
			node.Update(r.actor)
			node.Cols("last_config_hash")
			if _, err := tx.Save(&node); err != nil {
				return fmt.Errorf("save Gateway desired configuration hash: %w", err)
			}
		}
		if taskID == "" {
			return nil
		}
		var task model.NodeConfigTask
		found, err := tx.SQL(`
			SELECT * FROM public.t_ztna_node_config_task
			WHERE task_id=? AND node_id=? AND task_type='apply_config' AND target_config_hash=?
			FOR UPDATE
		`, taskID, node.ID, configHash).Get(&task)
		if err != nil {
			return fmt.Errorf("lock Gateway configuration task: %w", err)
		}
		if !found {
			if receipt {
				return errors.New("Gateway configuration receipt does not match a current task")
			}
			return nil
		}
		eventType := "task.sent"
		if len(eventTypes) > 0 {
			eventType = eventTypes[0]
		}
		if nextStatus == "sent" {
			switch task.Status {
			case "pending", "queued", "failed":
			default:
				return nil
			}
			if task.Status == "failed" {
				task.RetryCount++
			}
			if task.StartedAt == nil {
				now := time.Now().UTC()
				task.StartedAt = &now
			}
			task.FinishedAt = nil
			task.NextRetryAt = nil
			task.LastErrorCode = nil
			task.LastErrorMessage = nil
		} else {
			if task.Status == nextStatus {
				return nil
			}
			switch task.Status {
			case "pending", "queued", "sent", "received", "applying", "failed":
			default:
				return nil
			}
			now := time.Now().UTC()
			task.FinishedAt = &now
			if nextStatus == "failed" {
				task.LastErrorCode = optionalString(errorCode)
				task.LastErrorMessage = optionalString(errorMessage)
			}
		}
		return r.saveTaskStatus(tx, &task, nextStatus, errorCode, errorMessage, eventType, map[string]string{
			"config_hash": configHash,
			"error_code":  errorCode,
		}, actor)
	})
}

func (r *GatewayConfigurationSource) saveTaskStatus(tx *database.DBTransaction, task *model.NodeConfigTask, status, errorCode, errorMessage, eventType string, detail any, actor string) error {
	before := task.Status
	task.Status = status
	task.Update(actor)
	task.Cols("status", "retry_count", "next_retry_at", "last_error_code", "last_error_message", "started_at", "finished_at")
	if _, err := tx.Save(task); err != nil {
		return fmt.Errorf("save Gateway configuration task status: %w", err)
	}
	detailJSON, err := json.Marshal(detail)
	if err != nil {
		return fmt.Errorf("encode Gateway configuration task event: %w", err)
	}
	fromStatus, toStatus := before, status
	event := model.NodeConfigTaskEvent{
		TaskID: task.TaskID, NodeID: task.NodeID, EventType: eventType,
		FromStatus: &fromStatus, ToStatus: &toStatus, DetailJSON: detailJSON,
		OccurredAt: time.Now().UTC(),
	}
	event.CreateNew(actor)
	if _, err := tx.Save(&event); err != nil {
		return fmt.Errorf("save Gateway configuration task event: %w", err)
	}
	return nil
}

func lockActiveNode(tx *database.DBTransaction, nodeCode string) (model.Node, error) {
	var node model.Node
	found, err := tx.SQL(`
		SELECT id, node_code, node_type, node_status, last_config_hash
		FROM public.t_ztna_node
		WHERE node_code=? AND node_type IN ('gateway','client_agent') AND node_status='active'
		FOR UPDATE
	`, nodeCode).Get(&node)
	if err != nil {
		return model.Node{}, fmt.Errorf("lock node configuration target: %w", err)
	}
	if !found {
		return model.Node{}, errors.New("node configuration target is not active")
	}
	return node, nil
}

func newGatewayConfigTaskID() (string, error) {
	var value [16]byte
	if _, err := rand.Read(value[:]); err != nil {
		return "", err
	}
	return "cfg-" + hex.EncodeToString(value[:]), nil
}

var _ configsync.Source = (*GatewayConfigurationSource)(nil)
