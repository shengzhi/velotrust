package postgres

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
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
	db    *database.Database
	actor string
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
	return &GatewayConfigurationSource{db: db, actor: auditActor(instanceID)}, nil
}

type gatewaySnapshotRow struct {
	NodeID        int64   `xorm:"'node_id'"`
	NodeCode      string  `xorm:"'node_code'"`
	TunnelAddress *string `xorm:"'tunnel_address'"`
	PeersJSON     []byte  `xorm:"'peers_json'"`
	RoutesJSON    []byte  `xorm:"'routes_json'"`
	NATJSON       []byte  `xorm:"'nat_json'"`
	OverlayCIDR   *string `xorm:"'overlay_cidr'"`
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

// gatewayRulePayload intentionally stays empty until access-policy rules have
// a stable, documented compiler input. An empty ruleset is default deny in the
// Gateway dataplane.
type gatewayRulePayload struct{}

func (r *GatewayConfigurationSource) Current(ctx context.Context, nodeCode, appliedHash string) (configsync.Snapshot, bool, error) {
	if ctx == nil || strings.TrimSpace(nodeCode) == "" {
		return configsync.Snapshot{}, false, errors.New("request context and Gateway node code are required")
	}
	s := r.db.Engine().NewSession().Context(ctx)
	defer s.Close()
	payload, nodeID, err := compileGatewaySnapshot(s, nodeCode)
	if err != nil {
		return configsync.Snapshot{}, false, err
	}
	hash, err := configpayload.Hash(payload)
	if err != nil {
		return configsync.Snapshot{}, false, fmt.Errorf("hash compiled Gateway configuration: %w", err)
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
			WHERE membership.node_id=node.id AND membership.status='active'
		), '[]'::jsonb) AS peers_json,
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
		Rules:         make([]gatewayRulePayload, 0),
	}
	var peerRows []struct {
		ID                  int64    `json:"id"`
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
			ID: fmt.Sprintf("peer-%d", peer.ID), PublicKey: strings.TrimSpace(peer.PublicKey),
			Endpoint: strings.TrimSpace(peer.Endpoint), AllowedIPs: allowed,
			PersistentKeepalive: uint16(peer.PersistentKeepalive),
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
			payload.NAT = append(payload.NAT, gatewayNATPayload{Type: "dnat", ShadowPrefix: virtual, RealPrefix: real})
		case "SNAT_Masquerade":
			if row.OverlayCIDR == nil || strings.TrimSpace(*row.OverlayCIDR) == "" {
				return nil, 0, fmt.Errorf("%w: SNAT masquerade requires the global overlay network", configsync.ErrInvalidConfiguration)
			}
			overlay, err := canonicalPrefix(*row.OverlayCIDR)
			if err != nil {
				return nil, 0, fmt.Errorf("%w: invalid global overlay network: %v", configsync.ErrInvalidConfiguration, err)
			}
			payload.NAT = append(payload.NAT, gatewayNATPayload{Type: "masquerade", SourcePrefix: overlay, DestinationPrefix: real})
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

func (r *GatewayConfigurationSource) ensureTask(ctx context.Context, nodeCode string, nodeID int64, configHash, appliedHash string) (string, error) {
	var taskID string
	err := r.db.WithSaveTransaction(ctx, func(tx *database.DBTransaction) error {
		node, err := lockActiveGateway(tx, nodeCode)
		if err != nil {
			return err
		}
		if node.ID != nodeID {
			return errors.New("Gateway target changed while compiling configuration")
		}
		currentPayload, _, err := compileGatewaySnapshot(tx.Session, nodeCode)
		if err != nil {
			return err
		}
		currentHash, err := configpayload.Hash(currentPayload)
		if err != nil {
			return fmt.Errorf("hash Gateway configuration in task transaction: %w", err)
		}
		if !strings.EqualFold(currentHash, configHash) {
			return errors.New("Gateway business configuration changed while creating its delivery task")
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
		node, err := lockActiveGateway(tx, nodeCode)
		if err != nil {
			return err
		}
		payload, _, err := compileGatewaySnapshot(tx.Session, nodeCode)
		if err != nil {
			return err
		}
		currentHash, err := configpayload.Hash(payload)
		if err != nil {
			return fmt.Errorf("hash current Gateway business configuration: %w", err)
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
		return r.saveTaskStatus(tx, &task, "applied", "", "", "config.applied", map[string]string{"config_hash": currentHash})
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
		node, err := lockActiveGateway(tx, nodeCode)
		if err != nil {
			return err
		}
		payload, _, err := compileGatewaySnapshot(tx.Session, nodeCode)
		if err != nil {
			return err
		}
		currentHash, err := configpayload.Hash(payload)
		if err != nil {
			return fmt.Errorf("hash current Gateway business configuration: %w", err)
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
		})
	})
}

func (r *GatewayConfigurationSource) saveTaskStatus(tx *database.DBTransaction, task *model.NodeConfigTask, status, errorCode, errorMessage, eventType string, detail any) error {
	before := task.Status
	task.Status = status
	task.Update(r.actor)
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
	event.CreateNew(r.actor)
	if _, err := tx.Save(&event); err != nil {
		return fmt.Errorf("save Gateway configuration task event: %w", err)
	}
	return nil
}

func lockActiveGateway(tx *database.DBTransaction, nodeCode string) (model.Node, error) {
	var node model.Node
	found, err := tx.SQL(`
		SELECT id, node_code, node_type, node_status, last_config_hash
		FROM public.t_ztna_node
		WHERE node_code=? AND node_type='gateway' AND node_status='active'
		FOR UPDATE
	`, nodeCode).Get(&node)
	if err != nil {
		return model.Node{}, fmt.Errorf("lock Gateway configuration target: %w", err)
	}
	if !found {
		return model.Node{}, errors.New("Gateway configuration target is not active")
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
