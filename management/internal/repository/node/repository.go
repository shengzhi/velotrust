package node

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	servicecontext "velotrust/management/internal/context"
	enrollmentModel "velotrust/management/internal/model/enrollment"
	iamModel "velotrust/management/internal/model/iam"
	model "velotrust/management/internal/model/ztna"
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
	ID                 int64      `xorm:"'id'"`
	VersionNo          int64      `xorm:"'version_no'"`
	NodeName           string     `xorm:"'node_name'"`
	NodeCode           string     `xorm:"'node_code'"`
	NodeType           string     `xorm:"'node_type'"`
	NodeStatus         string     `xorm:"'node_status'"`
	AgentVersion       *string    `xorm:"'agent_version'"`
	ControlEndpoint    *string    `xorm:"'control_endpoint'"`
	PublicEndpoint     *string    `xorm:"'public_endpoint'"`
	PrivateEndpoint    *string    `xorm:"'private_endpoint'"`
	PublicKey          *string    `xorm:"'public_key'"`
	LastHeartbeatAt    *time.Time `xorm:"'last_heartbeat_at'"`
	LastConfigHash     *string    `xorm:"'last_config_hash'"`
	ActualConfigHash   *string    `xorm:"'actual_config_hash'"`
	CapabilitiesJSON   []byte     `xorm:"'capabilities_json'"`
	HealthStatus       string     `xorm:"'health_status'"`
	GatewayType        *string    `xorm:"'gateway_type'"`
	GatewayNetworkName *string    `xorm:"'gateway_network_name'"`
	RequireApproval    bool       `xorm:"'require_approval'"`
	RegisteredAt       *time.Time `xorm:"'registered_at'"`
	CreatedAt          time.Time  `xorm:"'created_at'"`
	NetworksJSON       []byte     `xorm:"'networks_json'"`
	PeersJSON          []byte     `xorm:"'peers_json'"`
}

const nodeViewQuery = `
SELECT n.id, n.version_no, n.node_name, n.node_code, n.node_type, n.node_status,
       n.agent_version, n.control_endpoint, n.public_endpoint, n.private_endpoint,
       n.public_key, n.last_heartbeat_at, n.last_config_hash, n.actual_config_hash,
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
       ), '[]'::json) AS peers_json
FROM t_ztna_node n`

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
	item := model.VirtualNodeView{
		ID: row.ID, VersionNo: row.VersionNo, NodeName: row.NodeName, NodeCode: row.NodeCode,
		NodeType: row.NodeType, NodeStatus: row.NodeStatus, AgentVersion: row.AgentVersion,
		ControlEndpoint: row.ControlEndpoint, PublicEndpoint: row.PublicEndpoint,
		PrivateEndpoint: row.PrivateEndpoint, PublicKey: row.PublicKey,
		LastHeartbeatAt: row.LastHeartbeatAt, LastConfigHash: row.LastConfigHash,
		ActualConfigHash: row.ActualConfigHash, HealthStatus: row.HealthStatus,
		GatewayType: row.GatewayType, GatewayNetworkName: row.GatewayNetworkName,
		RequireApproval: row.RequireApproval, RegisteredAt: row.RegisteredAt,
		CreatedAt: row.CreatedAt, Networks: []model.VirtualNodeNetwork{}, Peers: []model.VirtualNodePeer{},
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
		if err := saveAudit(ctx, tx, actorID, &current, before, status, revokedCredentialCount); err != nil {
			return err
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

func saveAudit(ctx *servicecontext.Context, tx *database.DBTransaction, actorID int64, node *model.Node, before, after string, revokedCredentials int) error {
	changes, err := json.Marshal(map[string]any{
		"node_status_from":                         before,
		"node_status_to":                           after,
		"pending_registration_credentials_revoked": revokedCredentials,
	})
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
		Action: "ztna.node.status", TargetType: "ztna_node", TargetID: &targetID,
		TargetName: &targetName, Result: "success", SourceIP: optionalString(ctx.ClientIP()),
		RequestID: optionalString(ctx.RequestID()), ChangesJSON: changes, OccurredAt: time.Now().UTC(),
	}
	audit.CreateNew(userName)
	if _, err := tx.Save(audit); err != nil {
		return fmt.Errorf("write virtual-node status audit: %w", err)
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
