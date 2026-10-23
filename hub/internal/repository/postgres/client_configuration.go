package postgres

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"sort"
	"strings"

	"velotrust/hub/internal/configsync"
	"xorm.io/xorm"
)

// Keep this in sync with client/internal/config.maxTunnelPeers. If an
// employee is authorized for more Gateways than the Client can install, an
// empty peer set is safer than leaving its previous tunnel configuration
// active or silently selecting only part of the authorization result.
const maxClientGatewayPeers = 64

type clientSnapshotPayload struct {
	SchemaVersion int                 `json:"schema_version"`
	Tunnel        clientTunnelPayload `json:"tunnel"`
}

type clientTunnelPayload struct {
	Address string              `json:"address"`
	MTU     int                 `json:"mtu"`
	Peers   []clientPeerPayload `json:"peers"`
}

type clientPeerPayload struct {
	ID                  string   `json:"id"`
	PeerName            string   `json:"peer_name"`
	PublicKey           string   `json:"public_key"`
	Endpoint            string   `json:"endpoint"`
	AllowedIPs          []string `json:"allowed_ips"`
	PersistentKeepalive uint16   `json:"persistent_keepalive,omitempty"`
}

// compileClientSnapshot derives Gateway admission from active group bindings
// and group-to-resource access policies. Each peer receives routes from its
// associated site; the Gateway firewall applies the resource-specific ACL.
func compileClientSnapshot(s *xorm.Session, nodeCode string) ([]byte, int64, error) {
	var target struct {
		NodeID            int64   `xorm:"'node_id'"`
		OwnerID           int64   `xorm:"'owner_id'"`
		Active            bool    `xorm:"'active'"`
		TunnelAddress     *string `xorm:"'tunnel_address'"`
		BoundGatewayCount int64   `xorm:"'bound_gateway_count'"`
		GatewaysJSON      []byte  `xorm:"'gateways_json'"`
	}
	found, err := s.SQL(`SELECT node.id AS node_id,
		owner.id AS owner_id,
		(node.node_status='active' AND device.status='active' AND owner.user_status='active'
		 AND owner.account_type='employee' AND credential.status='active'
		 AND credential.not_before<=CURRENT_TIMESTAMP
		 AND (credential.expires_at IS NULL OR credential.expires_at>CURRENT_TIMESTAMP)) AS active,
		client_lease.address AS tunnel_address,
		COALESCE((
			SELECT jsonb_agg(jsonb_build_object(
				'id', 'gateway-' || candidate.id,
				'peer_name', candidate.peer_name,
				'public_key', candidate.public_key,
				'endpoint', candidate.endpoint,
				'allowed_ips', candidate.allowed_ips || jsonb_build_array(host(candidate.tunnel_address) || '/' || masklen(candidate.tunnel_address)::text),
				'persistent_keepalive', 25
			) ORDER BY candidate.id)
			FROM LATERAL (
				SELECT DISTINCT gateway.id, gateway.node_name AS peer_name, gateway.public_key,
					COALESCE(NULLIF(gateway.public_endpoint, ''), NULLIF(gateway_lease.endpoint, '')) AS endpoint,
					gateway_lease.address AS tunnel_address,
					COALESCE((
						SELECT jsonb_agg(destination.cidr ORDER BY destination.cidr)
						FROM (
							SELECT network.cidr::text AS cidr
							FROM public.t_ztna_network AS network
							WHERE network.gateway_node_id=gateway.id AND network.network_type='site'
							  AND network.status='active' AND network.is_del=FALSE
							  AND NOT EXISTS (
								SELECT 1 FROM public.t_ztna_nat_mapping AS mapping
								WHERE mapping.network_id=network.id AND mapping.enabled=TRUE
								  AND mapping.virtual_cidr IS NOT NULL
							  )
							UNION
							SELECT mapping.virtual_cidr::text AS cidr
							FROM public.t_ztna_nat_mapping AS mapping
							JOIN public.t_ztna_network AS network ON network.id=mapping.network_id
							WHERE network.gateway_node_id=gateway.id AND network.network_type='site'
							  AND network.status='active' AND network.is_del=FALSE
							  AND mapping.enabled=TRUE AND mapping.virtual_cidr IS NOT NULL
						) AS destination
					), '[]'::jsonb) AS allowed_ips
				FROM (
					SELECT binding.group_id, binding.gateway_node_id
					FROM public.t_ztna_gateway_group_binding AS binding
					UNION
					SELECT subject.subject_id AS group_id,
						network.gateway_node_id AS gateway_node_id
					FROM public.t_ztna_access_policy AS policy
					JOIN public.t_ztna_policy_subject AS subject ON subject.policy_id=policy.id AND subject.subject_type='group'
					JOIN public.t_ztna_policy_resource AS policy_resource ON policy_resource.policy_id=policy.id
					JOIN public.t_ztna_protected_resource AS resource ON resource.id=policy_resource.protected_resource_id AND resource.status='active'
					JOIN public.t_ztna_network AS network ON network.id=resource.network_id
						AND network.network_type='site' AND network.status='active' AND network.is_del=FALSE
					WHERE policy.status='active' AND network.gateway_node_id IS NOT NULL
				) AS admission
				JOIN public.t_iam_user_group AS membership ON membership.group_id=admission.group_id AND membership.user_id=owner.id
				JOIN public.t_iam_group AS employee_group ON employee_group.id=membership.group_id AND employee_group.status='active'
				JOIN public.t_ztna_node AS gateway ON gateway.id=admission.gateway_node_id
				  AND gateway.node_type='gateway' AND gateway.node_status='active'
				JOIN public.t_ztna_address_lease AS gateway_lease ON gateway_lease.wireguard_pubkey=gateway.public_key
				  AND gateway_lease.node_type='Gateway' AND gateway_lease.status IN ('active','reserved')
				WHERE gateway.public_key IS NOT NULL
				  AND COALESCE(NULLIF(gateway.public_endpoint, ''), NULLIF(gateway_lease.endpoint, '')) IS NOT NULL
			) AS candidate
		), '[]'::jsonb) AS gateways_json,
		(SELECT COUNT(DISTINCT admission.gateway_node_id)
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
		  JOIN public.t_iam_user_group AS membership ON membership.group_id=admission.group_id AND membership.user_id=owner.id
		  JOIN public.t_iam_group AS employee_group ON employee_group.id=membership.group_id AND employee_group.status='active'
		) AS bound_gateway_count
		FROM public.t_ztna_node node
		JOIN public.t_ztna_device device ON device.node_id=node.id
		JOIN public.t_iam_user owner ON owner.id=device.owner_user_id
		JOIN public.t_ztna_device_credential credential ON credential.device_id=device.id
		  AND credential.public_key=node.public_key AND credential.credential_type='x25519'
		LEFT JOIN LATERAL (
			SELECT host(lease.address) || '/' || masklen(lease.address)::text AS address
			FROM public.t_ztna_address_lease lease
			WHERE lease.wireguard_pubkey=node.public_key AND lease.node_type='Peer' AND lease.status='active'
			ORDER BY lease.id DESC LIMIT 1
		) AS client_lease ON TRUE
		WHERE node.node_code=? AND node.node_type='client_agent'
		ORDER BY (credential.status='active' AND credential.not_before<=CURRENT_TIMESTAMP
		  AND (credential.expires_at IS NULL OR credential.expires_at>CURRENT_TIMESTAMP)) DESC,
		  credential.key_version DESC, credential.id DESC LIMIT 1`, nodeCode).Get(&target)
	if err != nil {
		return nil, 0, fmt.Errorf("read Client configuration target: %w", err)
	}
	if !found || target.NodeID <= 0 || !target.Active {
		return nil, 0, fmt.Errorf("%w: active Client was not found", configsync.ErrTargetUnavailable)
	}
	if target.TunnelAddress == nil || strings.TrimSpace(*target.TunnelAddress) == "" {
		return nil, target.NodeID, configsync.ErrClientAddressNotAssigned
	}
	var peers []clientPeerPayload
	if err := json.Unmarshal(target.GatewaysJSON, &peers); err != nil {
		return nil, target.NodeID, fmt.Errorf("decode Client Gateway security-group bindings: %w", err)
	}
	claimedPrefixes := make(map[string]string)
	for i := range peers {
		allowed := make([]string, 0, len(peers[i].AllowedIPs))
		seen := make(map[string]struct{}, len(peers[i].AllowedIPs))
		for _, raw := range peers[i].AllowedIPs {
			prefix, err := canonicalPrefix(raw)
			if err != nil {
				return nil, target.NodeID, fmt.Errorf("%w: Gateway peer %q has invalid route prefix %q: %v", configsync.ErrInvalidConfiguration, peers[i].ID, raw, err)
			}
			if _, exists := seen[prefix]; exists {
				continue
			}
			if owner, exists := claimedPrefixes[prefix]; exists && owner != peers[i].ID {
				return nil, target.NodeID, fmt.Errorf("%w: Gateway peers %q and %q claim the same Client route %s", configsync.ErrInvalidConfiguration, owner, peers[i].ID, prefix)
			}
			seen[prefix] = struct{}{}
			claimedPrefixes[prefix] = peers[i].ID
			allowed = append(allowed, prefix)
		}
		if len(allowed) > 256 {
			return nil, target.NodeID, fmt.Errorf("%w: Gateway peer %q has %d routes; the Client limit is 256 per peer", configsync.ErrInvalidConfiguration, peers[i].ID, len(allowed))
		}
		sort.Strings(allowed)
		peers[i].AllowedIPs = allowed
	}
	if len(peers) > maxClientGatewayPeers {
		slog.Error("Client is authorized for more Gateways than its tunnel configuration supports; withholding all Gateway peers",
			"node_id", nodeCode, "authorized_gateway_count", len(peers), "maximum_gateway_count", maxClientGatewayPeers)
		peers = make([]clientPeerPayload, 0)
	}
	slog.Debug("compiled Client Gateway Peer configuration", "node_id", nodeCode,
		"employee_user_id", target.OwnerID, "security_group_bound_gateway_count", target.BoundGatewayCount,
		"eligible_peer_count", len(peers))
	payload := clientSnapshotPayload{SchemaVersion: 1, Tunnel: clientTunnelPayload{
		Address: strings.TrimSpace(*target.TunnelAddress), MTU: 1380, Peers: peers,
	}}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return nil, target.NodeID, fmt.Errorf("encode Client tunnel configuration: %w", err)
	}
	return encoded, target.NodeID, nil
}
