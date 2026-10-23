package node

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	servicecontext "velotrust/management/internal/context"
	enrollmentModel "velotrust/management/internal/model/enrollment"
	iamModel "velotrust/management/internal/model/iam"
	model "velotrust/management/internal/model/ztna"
	accesschanges "velotrust/management/internal/repository/ztna"
	appErrors "velotrust/pkg/errors"
	"velotrust/shared/repository/database"
	"xorm.io/xorm"
)

const (
	iamUserTable           = "t_iam_user"
	iamUserRoleTable       = "t_iam_user_role"
	iamRoleTable           = "t_iam_role"
	iamRolePermissionTable = "t_iam_role_permission"
	iamPermissionTable     = "t_iam_permission"
	iamResourceTable       = "t_iam_resource"
)

type Repository struct{ db *database.Database }

func New(db *database.Database) (*Repository, error) {
	if db == nil || db.Engine() == nil || db.Driver() != "postgres" {
		return nil, errors.New("virtual-node repository requires a PostgreSQL database")
	}
	return &Repository{db: db}, nil
}

type nodeRow struct {
	ID                 int64        `xorm:"'id'"`
	VersionNo          int64        `xorm:"'version_no'"`
	NodeName           string       `xorm:"'node_name'"`
	NodeCode           string       `xorm:"'node_code'"`
	NodeType           string       `xorm:"'node_type'"`
	NodeStatus         string       `xorm:"'node_status'"`
	ConnectionStatus   string       `xorm:"'connection_status'"`
	AgentVersion       *string      `xorm:"'agent_version'"`
	ControlEndpoint    *string      `xorm:"'control_endpoint'"`
	PublicEndpoint     *string      `xorm:"'public_endpoint'"`
	PrivateEndpoint    *string      `xorm:"'private_endpoint'"`
	IPAMAddress        *string      `xorm:"'ipam_address'"`
	PublicKey          *string      `xorm:"'public_key'"`
	LastHeartbeatAt    sql.NullTime `xorm:"'last_heartbeat_at'"`
	LastConfigHash     *string      `xorm:"'last_config_hash'"`
	ActualConfigHash   *string      `xorm:"'actual_config_hash'"`
	CapabilitiesJSON   []byte       `xorm:"'capabilities_json'"`
	HealthStatus       string       `xorm:"'health_status'"`
	GatewayType        *string      `xorm:"'gateway_type'"`
	GatewayNetworkName *string      `xorm:"'gateway_network_name'"`
	RequireApproval    bool         `xorm:"'require_approval'"`
	RegisteredAt       *time.Time   `xorm:"'registered_at'"`
	CreatedAt          time.Time    `xorm:"'created_at'"`
	NetworksJSON       []byte       `xorm:"'networks_json'"`
	PeersJSON          []byte       `xorm:"'peers_json'"`
	GatewayGroupsJSON  []byte       `xorm:"'gateway_groups_json'"`
	DeviceID           *int64       `xorm:"'device_id'"`
	DeviceName         *string      `xorm:"'device_name'"`
	DeviceType         *string      `xorm:"'device_type'"`
	DevicePlatform     *string      `xorm:"'device_platform'"`
	DeviceStatus       *string      `xorm:"'device_status'"`
	OwnerUserID        *int64       `xorm:"'owner_user_id'"`
	OwnerUsername      *string      `xorm:"'owner_username'"`
}

const nodeViewQuery = `
SELECT n.id, n.version_no, n.node_name, n.node_code, n.node_type, n.node_status,
       CASE WHEN n.node_status <> 'active' THEN 'offline'
            WHEN n.last_heartbeat_at IS NULL THEN 'unknown'
            WHEN n.last_heartbeat_at >= CURRENT_TIMESTAMP - INTERVAL '45 seconds'
             AND n.last_heartbeat_at <= CURRENT_TIMESTAMP + INTERVAL '5 seconds' THEN 'online'
            ELSE 'offline' END AS connection_status,
       n.agent_version, n.control_endpoint, n.public_endpoint, n.private_endpoint,
       node_ipam.ipam_address, n.public_key, n.last_heartbeat_at, n.last_config_hash, n.actual_config_hash,
       n.capabilities_json, n.health_status, n.gateway_type, n.gateway_network_name,
       n.require_approval, n.registered_at, n.created_at,
       COALESCE((
           SELECT json_agg(json_build_object(
               'id', net.id, 'network_name', net.network_name,
               'network_code', net.network_code, 'network_type', net.network_type,
               'cidr', net.cidr::text, 'status', net.status
           ) ORDER BY net.network_name, net.id)
           FROM t_ztna_network net
           WHERE net.gateway_node_id = n.id AND net.is_del = FALSE
       ), '[]'::json) AS networks_json,
       COALESCE((
           SELECT json_agg(json_build_object(
               'id', peer.id, 'tunnel_id', peer.tunnel_id,
               'tunnel_code', tunnel.tunnel_code, 'tunnel_ip', peer.tunnel_ip::text,
               'allowed_ips', peer.allowed_ips, 'endpoint', peer.endpoint,
               'status', peer.status, 'last_handshake_at', peer.last_handshake_at
           ) ORDER BY peer.id)
           FROM t_ztna_tunnel_peer peer
           JOIN t_ztna_tunnel tunnel ON tunnel.id = peer.tunnel_id
           WHERE peer.node_id = n.id
       ), '[]'::json) AS peers_json,
       COALESCE((
           SELECT json_agg(json_build_object(
               'id', security_group.id, 'name', security_group.group_name,
               'code', security_group.group_code, 'status', security_group.status
           ) ORDER BY security_group.group_name, security_group.id)
           FROM public.t_ztna_gateway_group_binding AS binding
           JOIN public.t_iam_group AS security_group ON security_group.id=binding.group_id
           WHERE binding.gateway_node_id=n.id
       ), '[]'::json) AS gateway_groups_json,
       linked_device.device_id, linked_device.device_name, linked_device.device_type,
       linked_device.device_platform, linked_device.device_status,
       linked_device.owner_user_id, linked_device.owner_username
FROM t_ztna_node n
LEFT JOIN LATERAL (
    SELECT host(lease.address) AS ipam_address
    FROM t_ztna_address_lease lease
    WHERE lease.status IN ('active', 'reserved')
      AND (
          (n.public_key IS NOT NULL AND lease.wireguard_pubkey = n.public_key)
          OR (
              lease.target_name = n.node_name
              AND (lease.wireguard_pubkey IS NULL OR lease.wireguard_pubkey = '')
              AND CASE n.node_type
                  WHEN 'gateway' THEN lease.node_type = 'Gateway'
                  WHEN 'client_agent' THEN lease.node_type = 'Peer'
                  ELSE FALSE
              END
          )
      )
    ORDER BY CASE WHEN n.public_key IS NOT NULL AND lease.wireguard_pubkey = n.public_key THEN 0 ELSE 1 END,
             lease.allocated_at DESC, lease.id DESC
    LIMIT 1
) node_ipam ON TRUE
LEFT JOIN LATERAL (
    SELECT device.id AS device_id, device.device_name, device.device_type,
           device.platform AS device_platform, device.status AS device_status,
           device.owner_user_id, owner.username AS owner_username
    FROM t_ztna_device_credential credential
    JOIN t_ztna_device device ON device.id=credential.device_id
    LEFT JOIN t_iam_user owner ON owner.id=device.owner_user_id
    WHERE credential.public_key=n.public_key
    ORDER BY CASE WHEN credential.status='active' THEN 0 ELSE 1 END,
             credential.key_version DESC, credential.id DESC
    LIMIT 1
) linked_device ON TRUE`

func (r *Repository) ListNodes(ctx *servicecontext.Context) ([]model.VirtualNodeView, error) {
	if ctx == nil {
		return nil, errors.New("service context is required")
	}
	return r.loadNodes(ctx, 0)
}

func (r *Repository) GetNode(ctx *servicecontext.Context, id int64) (*model.VirtualNodeView, error) {
	if ctx == nil || id <= 0 {
		return nil, errors.New("service context and positive node ID are required")
	}
	items, err := r.loadNodes(ctx, id)
	if err != nil {
		return nil, err
	}
	if len(items) == 0 {
		return nil, nil
	}
	return &items[0], nil
}

func (r *Repository) GetAccessChangeStatus(ctx *servicecontext.Context, subjectType string, subjectID int64) (*model.AccessChangeStatusView, error) {
	if ctx == nil || subjectID <= 0 {
		return nil, errors.New("service context and positive subject ID are required")
	}
	s := r.db.Engine().NewSession().Context(ctx)
	defer s.Close()
	return accesschanges.AccessChangeStatus(ctx, s, subjectType, subjectID)
}

func (r *Repository) ListGatewayTunnelOptions(ctx *servicecontext.Context) ([]model.GatewayTunnelOption, error) {
	if ctx == nil {
		return nil, errors.New("service context is required")
	}
	s := r.db.Engine().NewSession().Context(ctx)
	defer s.Close()
	items := make([]model.GatewayTunnelOption, 0)
	if err := s.SQL(`SELECT tunnel.id, tunnel.tunnel_code AS code, COALESCE(network.network_name, '') AS network_name
		FROM public.t_ztna_tunnel tunnel
		LEFT JOIN public.t_ztna_network network ON network.id=tunnel.network_id
		WHERE tunnel.status='active'
		ORDER BY tunnel.tunnel_code, tunnel.id`).Find(&items); err != nil {
		return nil, fmt.Errorf("list active logical tunnels for Gateway assignment: %w", err)
	}
	return items, nil
}

func (r *Repository) CreateLogicalTunnel(ctx *servicecontext.Context, actorID int64, code, description string) (*model.GatewayTunnelOption, error) {
	if ctx == nil || actorID <= 0 || strings.TrimSpace(code) == "" {
		return nil, errors.New("authenticated operator and logical tunnel code are required")
	}
	var result *model.GatewayTunnelOption
	err := r.db.WithSaveTransaction(ctx, func(tx *database.DBTransaction) error {
		if err := requirePermission(tx.Session, actorID, "ztna.tunnel.write"); err != nil {
			return err
		}
		var existing struct {
			ID int64 `xorm:"'id'"`
		}
		found, err := tx.SQL("SELECT id FROM public."+model.TableTunnel+" WHERE tunnel_code=?", code).Get(&existing)
		if err != nil {
			return fmt.Errorf("check logical tunnel code: %w", err)
		}
		if found {
			return appErrors.NewClientError("ZTNA_NODE_INVALID_ARGUMENT", map[string]string{"tunnel_code": "隧道编码已存在"})
		}
		var descriptionValue *string
		if description != "" {
			descriptionValue = &description
		}
		tunnel := model.Tunnel{TunnelCode: code, TunnelType: "wireguard", Status: "active", Description: descriptionValue}
		tunnel.CreateNew(ctx.UserName())
		if _, err := tx.Save(&tunnel); err != nil {
			return fmt.Errorf("create logical tunnel: %w", err)
		}
		if err := saveTunnelAudit(ctx, tx, actorID, tunnel.ID, tunnel.TunnelCode, map[string]any{
			"tunnel_type": tunnel.TunnelType,
			"status":      tunnel.Status,
			"description": tunnel.Description,
		}); err != nil {
			return err
		}
		result = &model.GatewayTunnelOption{ID: tunnel.ID, Code: tunnel.TunnelCode}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (r *Repository) AssignGatewayTunnel(ctx *servicecontext.Context, actorID, nodeID, tunnelID, expectedVersion int64) error {
	if ctx == nil || actorID <= 0 || nodeID <= 0 || tunnelID <= 0 || expectedVersion <= 0 {
		return errors.New("authenticated operator, Gateway, tunnel, and expected version are required")
	}
	return r.db.WithSaveTransaction(ctx, func(tx *database.DBTransaction) error {
		s := tx.Session
		if err := requirePermission(s, actorID, "ztna.tunnel.write"); err != nil {
			return err
		}
		var gateway model.Node
		found, err := s.SQL("SELECT * FROM public."+model.TableNode+" WHERE id=? FOR UPDATE", nodeID).Get(&gateway)
		if err != nil {
			return fmt.Errorf("lock Gateway before logical tunnel assignment: %w", err)
		}
		if !found {
			return appErrors.NewResourceError("ZTNA_NODE_NOT_FOUND", "virtual node", true)
		}
		if gateway.VersionNo != expectedVersion {
			return appErrors.NewResourceError("ZTNA_NODE_VERSION_CONFLICT", "virtual node", false)
		}
		if gateway.NodeType != "gateway" || gateway.NodeStatus != "active" || gateway.RegisteredAt == nil || gateway.PublicKey == nil || strings.TrimSpace(*gateway.PublicKey) == "" {
			return appErrors.NewClientError("ZTNA_NODE_INVALID_ARGUMENT", map[string]string{
				"gateway": "Gateway 必须已注册、已启用并绑定公钥",
			})
		}
		if gateway.PublicEndpoint == nil || strings.TrimSpace(*gateway.PublicEndpoint) == "" {
			return appErrors.NewClientError("ZTNA_NODE_INVALID_ARGUMENT", map[string]string{
				"public_endpoint": "请先配置其他节点可访问的 Gateway UDP 公网端点",
			})
		}
		var tunnel struct {
			ID int64 `xorm:"'id'"`
		}
		activeTunnel, err := s.SQL("SELECT id FROM public."+model.TableTunnel+" WHERE id=? AND status='active'", tunnelID).Get(&tunnel)
		if err != nil {
			return fmt.Errorf("validate active logical tunnel: %w", err)
		}
		if !activeTunnel {
			return appErrors.NewClientError("ZTNA_NODE_INVALID_ARGUMENT", map[string]string{
				"tunnel_id": "请选择活动的逻辑隧道",
			})
		}
		var lease struct {
			Address string `xorm:"'tunnel_ip'"`
		}
		hasLease, err := s.SQL(`SELECT host(lease.address) || '/32' AS tunnel_ip
			FROM public.t_ztna_address_lease lease
			JOIN public.t_ztna_address_pool pool ON pool.id=lease.pool_id
			WHERE lease.wireguard_pubkey=? AND lease.node_type='Gateway'
			  AND lease.status IN ('active','reserved')
			  AND pool.pool_type='Gateway' AND pool.status='active'
			ORDER BY lease.id DESC LIMIT 1 FOR UPDATE OF lease`, *gateway.PublicKey).Get(&lease)
		if err != nil {
			return fmt.Errorf("load active Gateway IPAM lease: %w", err)
		}
		if !hasLease {
			return appErrors.NewClientError("ZTNA_NODE_INVALID_ARGUMENT", map[string]string{
				"gateway_ip": "请先为 Gateway 分配活动的 Gateway 类型 IPAM 地址",
			})
		}

		var peer model.TunnelPeer
		hasPeer, err := s.SQL("SELECT * FROM public."+model.TableTunnelPeer+" WHERE public_key=? FOR UPDATE", *gateway.PublicKey).Get(&peer)
		if err != nil {
			return fmt.Errorf("lock Gateway tunnel Peer: %w", err)
		}
		if hasPeer && peer.NodeID != gateway.ID {
			return appErrors.NewClientError("ZTNA_NODE_INVALID_ARGUMENT", map[string]string{
				"public_key": "Gateway 公钥已绑定到其他隧道节点",
			})
		}
		if hasPeer && peer.Status == "revoked" {
			return appErrors.NewClientError("ZTNA_NODE_INVALID_ARGUMENT", map[string]string{
				"peer": "已撤销的 Gateway 隧道 Peer 不能重新启用",
			})
		}
		if hasPeer && peer.TunnelID == tunnelID && peer.TunnelIP == lease.Address && peer.Status == "active" {
			return nil
		}
		beforeTunnelID := int64(0)
		if hasPeer {
			beforeTunnelID = peer.TunnelID
		}
		peer.TunnelID = tunnelID
		peer.NodeID = gateway.ID
		peer.DeviceID = nil
		peer.PublicKey = *gateway.PublicKey
		peer.TunnelIP = lease.Address
		peer.AllowedIPs = []byte("[]")
		peer.Endpoint = nil // HUB resolves the advertised endpoint from the node record.
		peer.PersistentKeepalive = 25
		peer.Status = "active"
		if peer.KeyVersion <= 0 {
			peer.KeyVersion = 1
		}
		if hasPeer {
			peer.Update(ctx.UserName())
			peer.Cols("tunnel_id", "node_id", "device_id", "public_key", "tunnel_ip", "allowed_ips", "endpoint", "persistent_keepalive", "status", "key_version")
			if _, err := tx.Save(&peer); err != nil {
				return fmt.Errorf("move Gateway Peer to logical tunnel: %w", err)
			}
		} else {
			peer.CreateNew(ctx.UserName())
			if _, err := tx.Save(&peer); err != nil {
				return fmt.Errorf("add Gateway Peer to logical tunnel: %w", err)
			}
		}
		if err := saveAudit(ctx, tx, actorID, &gateway, "ztna.gateway.tunnel_assignment", map[string]any{
			"tunnel_id_from": beforeTunnelID,
			"tunnel_id_to":   tunnelID,
			"tunnel_ip":      lease.Address,
		}); err != nil {
			return err
		}
		if _, err := accesschanges.CreateAccessChange(ctx, tx, "node", gateway.ID, "authorization_changed"); err != nil {
			return fmt.Errorf("queue HUB configuration refresh after Gateway tunnel assignment: %w", err)
		}
		return nil
	})
}

func saveTunnelAudit(ctx *servicecontext.Context, tx *database.DBTransaction, actorID, tunnelID int64, tunnelCode string, changes map[string]any) error {
	encoded, err := json.Marshal(changes)
	if err != nil {
		return fmt.Errorf("encode logical tunnel audit details: %w", err)
	}
	var eventID [16]byte
	if _, err := rand.Read(eventID[:]); err != nil {
		return fmt.Errorf("generate logical tunnel audit event ID: %w", err)
	}
	actor := ctx.UserName()
	if actor == "" {
		return errors.New("authenticated operator name is required for logical tunnel audit")
	}
	targetID := fmt.Sprintf("%d", tunnelID)
	audit := &iamModel.AuditLog{
		EventID: hex.EncodeToString(eventID[:]), UserID: &actorID, UserName: actor,
		Action: "ztna.tunnel.create", TargetType: "ztna_tunnel", TargetID: &targetID,
		TargetName: &tunnelCode, Result: "success", SourceIP: optionalString(ctx.ClientIP()),
		RequestID: optionalString(ctx.RequestID()), ChangesJSON: encoded, OccurredAt: time.Now().UTC(),
	}
	audit.CreateNew(actor)
	if _, err := tx.Save(audit); err != nil {
		return fmt.Errorf("write logical tunnel audit: %w", err)
	}
	return nil
}

func (r *Repository) loadNodes(ctx *servicecontext.Context, id int64) ([]model.VirtualNodeView, error) {
	s := r.db.Engine().NewSession().Context(ctx)
	defer s.Close()
	query := nodeViewQuery
	args := []any{}
	if id > 0 {
		query += " WHERE n.id = ?"
		args = append(args, id)
	}
	query += " ORDER BY n.node_name, n.id"
	var rows []nodeRow
	if err := s.SQL(query, args...).Find(&rows); err != nil {
		return nil, fmt.Errorf("list virtual nodes: %w", err)
	}
	items := make([]model.VirtualNodeView, 0, len(rows))
	for _, row := range rows {
		item, err := toNodeView(row)
		if err != nil {
			return nil, fmt.Errorf("decode virtual node %d: %w", row.ID, err)
		}
		items = append(items, item)
	}
	return items, nil
}

func toNodeView(row nodeRow) (model.VirtualNodeView, error) {
	var lastHeartbeatAt *time.Time
	if row.LastHeartbeatAt.Valid {
		utc := row.LastHeartbeatAt.Time.UTC()
		lastHeartbeatAt = &utc
	}
	item := model.VirtualNodeView{
		ID: row.ID, VersionNo: row.VersionNo, NodeName: row.NodeName, NodeCode: row.NodeCode,
		NodeType: row.NodeType, NodeStatus: row.NodeStatus, ConnectionStatus: row.ConnectionStatus, AgentVersion: row.AgentVersion,
		ControlEndpoint: row.ControlEndpoint, PublicEndpoint: row.PublicEndpoint,
		PrivateEndpoint: row.PrivateEndpoint, IPAMAddress: row.IPAMAddress, PublicKey: row.PublicKey,
		LastHeartbeatAt: lastHeartbeatAt, LastConfigHash: row.LastConfigHash,
		ActualConfigHash: row.ActualConfigHash, HealthStatus: row.HealthStatus,
		GatewayType: row.GatewayType, GatewayNetworkName: row.GatewayNetworkName,
		RequireApproval: row.RequireApproval, RegisteredAt: row.RegisteredAt,
		CreatedAt: row.CreatedAt, Networks: []model.VirtualNodeNetwork{}, Peers: []model.VirtualNodePeer{},
		GatewayGroups: []model.VirtualNodeGroup{},
		DeviceID:      row.DeviceID, DeviceName: row.DeviceName, DeviceType: row.DeviceType,
		DevicePlatform: row.DevicePlatform, DeviceStatus: row.DeviceStatus,
		OwnerUserID: row.OwnerUserID, OwnerUsername: row.OwnerUsername,
	}
	item.Capabilities = map[string]any{}
	if len(row.CapabilitiesJSON) != 0 {
		if err := json.Unmarshal(row.CapabilitiesJSON, &item.Capabilities); err != nil {
			return model.VirtualNodeView{}, fmt.Errorf("decode capabilities: %w", err)
		}
	}
	if len(row.NetworksJSON) != 0 {
		if err := json.Unmarshal(row.NetworksJSON, &item.Networks); err != nil {
			return model.VirtualNodeView{}, fmt.Errorf("decode assigned networks: %w", err)
		}
	}
	if len(row.PeersJSON) != 0 {
		if err := json.Unmarshal(row.PeersJSON, &item.Peers); err != nil {
			return model.VirtualNodeView{}, fmt.Errorf("decode tunnel peers: %w", err)
		}
	}
	if len(row.GatewayGroupsJSON) != 0 {
		if err := json.Unmarshal(row.GatewayGroupsJSON, &item.GatewayGroups); err != nil {
			return model.VirtualNodeView{}, fmt.Errorf("decode Gateway security-group bindings: %w", err)
		}
	}
	return item, nil
}

func (r *Repository) SetNodeStatus(ctx *servicecontext.Context, actorID, nodeID int64, status string, expectedVersion int64) error {
	if ctx == nil || actorID <= 0 || nodeID <= 0 || expectedVersion <= 0 {
		return errors.New("authenticated operator, node ID, and expected version are required")
	}
	if !manageableStatus(status) {
		return errors.New("virtual-node status must be active, disabled, quarantined, or revoked")
	}
	return r.db.WithSaveTransaction(ctx, func(tx *database.DBTransaction) error {
		s := tx.Session
		if err := requirePermission(s, actorID, "ztna.node.write"); err != nil {
			return err
		}
		var current model.Node
		found, err := s.SQL("SELECT * FROM "+model.TableNode+" WHERE id=? FOR UPDATE", nodeID).Get(&current)
		if err != nil {
			return fmt.Errorf("lock virtual node: %w", err)
		}
		if !found {
			return appErrors.NewResourceError("ZTNA_NODE_NOT_FOUND", "virtual node", true)
		}
		if current.VersionNo != expectedVersion {
			return appErrors.NewResourceError("ZTNA_NODE_VERSION_CONFLICT", "virtual node", false)
		}
		if current.NodeStatus == status {
			if current.NodeType == "client_agent" {
				changed, err := synchronizeLinkedDeviceStatus(ctx, tx, current.ID, status)
				if err != nil {
					return err
				}
				if changed {
					changeType := "enabled"
					if status != "active" {
						changeType = "disabled"
					}
					if _, err := accesschanges.CreateAccessChange(ctx, tx, "node", current.ID, changeType); err != nil {
						return fmt.Errorf("queue Gateway authorization refresh: %w", err)
					}
				}
			}
			return nil
		}
		if !canTransition(current.NodeStatus, status) {
			return appErrors.NewClientError("ZTNA_NODE_TRANSITION_DENIED", map[string]string{
				"node_status": fmt.Sprintf("不能将节点从 %s 变更为 %s", current.NodeStatus, status),
			})
		}
		if status == "active" && (current.PublicKey == nil || *current.PublicKey == "" || (current.NodeType == "gateway" && current.RegisteredAt == nil)) {
			return appErrors.NewClientError("ZTNA_NODE_ACTIVATION_REQUIRES_REGISTRATION", map[string]string{
				"node_status": "节点完成注册并提交公钥后才能启用",
			})
		}
		revokedCredentialCount := 0
		if current.NodeStatus == "pending" && status != "pending" {
			revokedCredentialCount, err = revokePendingCredentials(ctx, tx, current.ID)
			if err != nil {
				return err
			}
		}
		before := current.NodeStatus
		current.NodeStatus = status
		current.Update(ctx.UserName())
		current.Cols("node_status")
		if _, err := tx.Save(&current); err != nil {
			if errors.Is(err, database.ErrVersionConflict) {
				return appErrors.NewResourceError("ZTNA_NODE_VERSION_CONFLICT", "virtual node", false)
			}
			return fmt.Errorf("save virtual-node status: %w", err)
		}
		if err := saveAudit(ctx, tx, actorID, &current, "ztna.node.status", map[string]any{
			"node_status_from":                         before,
			"node_status_to":                           status,
			"pending_registration_credentials_revoked": revokedCredentialCount,
		}); err != nil {
			return err
		}
		if current.NodeType == "client_agent" {
			if _, err := synchronizeLinkedDeviceStatus(ctx, tx, current.ID, status); err != nil {
				return err
			}
		}
		changeType := "authorization_changed"
		if status == "disabled" || status == "quarantined" || status == "revoked" {
			changeType = "disabled"
		} else if status == "active" {
			changeType = "enabled"
		}
		if _, err := accesschanges.CreateAccessChange(ctx, tx, "node", current.ID, changeType); err != nil {
			return fmt.Errorf("queue Gateway authorization refresh: %w", err)
		}
		if current.NodeType == "gateway" {
			if err := accesschanges.CreateGatewayGroupClientRefreshes(ctx, tx, current.ID); err != nil {
				return fmt.Errorf("queue Client refresh after Gateway status change: %w", err)
			}
		}
		return nil
	})
}

func synchronizeLinkedDeviceStatus(ctx *servicecontext.Context, tx *database.DBTransaction, nodeID int64, nodeStatus string) (bool, error) {
	var devices []model.Device
	err := tx.Session.SQL(`
		SELECT device.* FROM public.t_ztna_device device
		WHERE device.node_id=? OR EXISTS (
			SELECT 1 FROM public.t_ztna_device_credential credential
			JOIN public.t_ztna_node node ON node.id=?
			WHERE credential.device_id=device.id AND credential.public_key=node.public_key
		)
		ORDER BY device.id FOR UPDATE
	`, nodeID, nodeID).Find(&devices)
	if err != nil {
		return false, fmt.Errorf("lock linked device status: %w", err)
	}
	if len(devices) == 0 {
		return false, nil
	}
	if len(devices) != 1 {
		return false, errors.New("client node public key matches multiple device records; resolve the device association before changing node status")
	}
	device := devices[0]
	status := nodeStatus
	if status == "pending" {
		status = "disabled"
	}
	changed := device.Status != status || device.NodeID == nil || *device.NodeID != nodeID
	if !changed {
		return false, nil
	}
	device.Status = status
	device.NodeID = &nodeID
	if status == "revoked" {
		now := time.Now().UTC()
		device.RevokedAt = &now
	} else {
		device.RevokedAt = nil
	}
	device.Update(ctx.UserName())
	device.Cols("status", "node_id", "revoked_at")
	if _, err := tx.Save(&device); err != nil {
		return false, fmt.Errorf("save linked device status: %w", err)
	}
	return true, nil
}

func (r *Repository) SetNodePublicEndpoint(ctx *servicecontext.Context, actorID, nodeID int64, endpoint string, expectedVersion int64) error {
	if ctx == nil || actorID <= 0 || nodeID <= 0 || expectedVersion <= 0 {
		return errors.New("authenticated operator, node ID, and expected version are required")
	}
	return r.db.WithSaveTransaction(ctx, func(tx *database.DBTransaction) error {
		s := tx.Session
		if err := requirePermission(s, actorID, "ztna.node.write"); err != nil {
			return err
		}
		var current model.Node
		found, err := s.SQL("SELECT * FROM "+model.TableNode+" WHERE id=? FOR UPDATE", nodeID).Get(&current)
		if err != nil {
			return fmt.Errorf("lock virtual node for public endpoint update: %w", err)
		}
		if !found {
			return appErrors.NewResourceError("ZTNA_NODE_NOT_FOUND", "virtual node", true)
		}
		if current.VersionNo != expectedVersion {
			return appErrors.NewResourceError("ZTNA_NODE_VERSION_CONFLICT", "virtual node", false)
		}
		before := ""
		if current.PublicEndpoint != nil {
			before = *current.PublicEndpoint
		}
		if before == endpoint {
			return nil
		}
		current.PublicEndpoint = optionalString(endpoint)
		current.Update(ctx.UserName())
		current.Cols("public_endpoint")
		if _, err := tx.Save(&current); err != nil {
			if errors.Is(err, database.ErrVersionConflict) {
				return appErrors.NewResourceError("ZTNA_NODE_VERSION_CONFLICT", "virtual node", false)
			}
			return fmt.Errorf("save virtual-node public endpoint: %w", err)
		}
		if err := saveAudit(ctx, tx, actorID, &current, "ztna.node.public_endpoint", map[string]any{
			"public_endpoint_from": optionalString(before),
			"public_endpoint_to":   optionalString(endpoint),
		}); err != nil {
			return err
		}
		if current.NodeType == "gateway" {
			if err := accesschanges.CreateGatewayGroupClientRefreshes(ctx, tx, current.ID); err != nil {
				return fmt.Errorf("queue Client refresh after Gateway endpoint change: %w", err)
			}
		}
		return nil
	})
}

func revokePendingCredentials(ctx *servicecontext.Context, tx *database.DBTransaction, nodeID int64) (int, error) {
	var credentials []enrollmentModel.Credential
	if err := tx.SQL("SELECT * FROM "+enrollmentModel.TableCredential+" WHERE node_id=? AND status='pending' FOR UPDATE", nodeID).Find(&credentials); err != nil {
		return 0, fmt.Errorf("lock pending node registration credentials: %w", err)
	}
	now := time.Now().UTC()
	reason := "node_lifecycle_changed"
	for i := range credentials {
		credential := &credentials[i]
		credential.Status = "revoked"
		credential.RevokedAt = &now
		credential.RevocationReason = &reason
		credential.Update(ctx.UserName())
		credential.Cols("status", "revoked_at", "revocation_reason")
		if _, err := tx.Save(credential); err != nil {
			return 0, fmt.Errorf("revoke pending node registration credential %d: %w", credential.ID, err)
		}
	}
	return len(credentials), nil
}

func manageableStatus(status string) bool {
	switch status {
	case "active", "disabled", "quarantined", "revoked":
		return true
	default:
		return false
	}
}

func canTransition(from, to string) bool {
	if to == "revoked" {
		return from != "revoked"
	}
	switch from {
	case "pending":
		return to == "active" || to == "disabled" || to == "quarantined"
	case "active":
		return to == "disabled" || to == "quarantined"
	case "disabled":
		return to == "active" || to == "quarantined"
	case "quarantined":
		return to == "active" || to == "disabled"
	default:
		return false
	}
}

func requirePermission(s *xorm.Session, actorID int64, permissionCode string) error {
	var result struct {
		Count int64 `xorm:"'count'"`
	}
	query := "SELECT COUNT(*) AS count FROM " + iamUserTable + " u WHERE u.id=? AND u.user_status='active' AND EXISTS (" +
		"SELECT 1 FROM " + iamUserRoleTable + " ur JOIN " + iamRoleTable + " r ON r.id=ur.role_id " +
		"JOIN " + iamRolePermissionTable + " rp ON rp.role_id=r.id JOIN " + iamPermissionTable + " p ON p.id=rp.permission_id " +
		"JOIN " + iamResourceTable + " res ON res.id=p.resource_id WHERE ur.user_id=u.id AND r.status='active' " +
		"AND p.status='active' AND res.status='active' AND p.permission_code=?)"
	if _, err := s.SQL(query, actorID, permissionCode).Get(&result); err != nil {
		return fmt.Errorf("check virtual-node permission: %w", err)
	}
	if result.Count == 0 {
		return errors.New("virtual-node write permission is required")
	}
	return nil
}

func saveAudit(ctx *servicecontext.Context, tx *database.DBTransaction, actorID int64, node *model.Node, action string, changeDetails map[string]any) error {
	changes, err := json.Marshal(changeDetails)
	if err != nil {
		return fmt.Errorf("encode virtual-node audit details: %w", err)
	}
	var eventID [16]byte
	if _, err := rand.Read(eventID[:]); err != nil {
		return fmt.Errorf("generate virtual-node audit event ID: %w", err)
	}
	userName := ctx.UserName()
	if userName == "" {
		return errors.New("authenticated operator name is required for virtual-node audit")
	}
	targetID, targetName := fmt.Sprintf("%d", node.ID), node.NodeName
	audit := &iamModel.AuditLog{
		EventID: hex.EncodeToString(eventID[:]), UserID: &actorID, UserName: userName,
		Action: action, TargetType: "ztna_node", TargetID: &targetID,
		TargetName: &targetName, Result: "success", SourceIP: optionalString(ctx.ClientIP()),
		RequestID: optionalString(ctx.RequestID()), ChangesJSON: changes, OccurredAt: time.Now().UTC(),
	}
	audit.CreateNew(userName)
	if _, err := tx.Save(audit); err != nil {
		return fmt.Errorf("write virtual-node audit for %s: %w", action, err)
	}
	return nil
}

func optionalString(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}

var _ Store = (*Repository)(nil)
