package enrollment

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	servicecontext "velotrust/management/internal/context"
	model "velotrust/management/internal/model/enrollment"
	iamModel "velotrust/management/internal/model/iam"
	ipamModel "velotrust/management/internal/model/ipam"
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
	if db == nil || db.Engine() == nil {
		return nil, errors.New("enrollment database is required")
	}
	if db.Driver() != "postgres" {
		return nil, fmt.Errorf("enrollment repository requires PostgreSQL, got %q", db.Driver())
	}
	return &Repository{db: db}, nil
}

func (r *Repository) ListCredentials(ctx *servicecontext.Context) ([]model.Credential, error) {
	s := r.db.Engine().NewSession().Context(ctx)
	defer s.Close()
	type credentialRow struct {
		ID                int64      `xorm:"'id'"`
		NodeID            int64      `xorm:"'node_id'"`
		GatewayID         string     `xorm:"'gateway_id'"`
		GatewayName       string     `xorm:"'gateway_name'"`
		GatewayType       string     `xorm:"'gateway_type'"`
		NetworkName       string     `xorm:"'network_name'"`
		AllowedSubnetsRaw []byte     `xorm:"'allowed_subnets'"`
		RequireApproval   bool       `xorm:"'require_approval'"`
		Status            string     `xorm:"'status'"`
		MaxUses           int        `xorm:"'max_uses'"`
		UsedCount         int        `xorm:"'used_count'"`
		ExpiresAt         time.Time  `xorm:"'expires_at'"`
		Notes             *string    `xorm:"'notes'"`
		CreatedBy         string     `xorm:"'created_by'"`
		CreatedByName     string     `xorm:"'created_by_name'"`
		CreatedAt         time.Time  `xorm:"'created_at'"`
		UpdatedAt         time.Time  `xorm:"'updated_at'"`
		UpdatedBy         string     `xorm:"'updated_by'"`
		VersionNo         int64      `xorm:"'version_no'"`
		UsedAt            *time.Time `xorm:"'used_at'"`
		UsedIP            *string    `xorm:"'used_ip'"`
		RevokedAt         *time.Time `xorm:"'revoked_at'"`
		RevocationReason  *string    `xorm:"'revocation_reason'"`
	}
	query := "SELECT c.id, c.node_id, g.node_code AS gateway_id, g.node_name AS gateway_name, g.gateway_type, g.gateway_network_name AS network_name, " +
		"COALESCE(routes.allowed_subnets, '[]'::jsonb) AS allowed_subnets, g.require_approval, " +
		"CASE WHEN c.status='pending' AND c.expires_at<=CURRENT_TIMESTAMP THEN 'expired' ELSE c.status END AS status, " +
		"c.max_uses, c.used_count, c.expires_at, c.notes, c.created_by, COALESCE(u.display_name,c.created_by) AS created_by_name, c.created_at, c.updated_at, c.updated_by, c.version_no, c.used_at, " +
		"c.used_ip::text AS used_ip, c.revoked_at, c.revocation_reason FROM " + model.TableCredential + " c " +
		"JOIN " + model.TableGateway + " g ON g.id=c.node_id AND g.node_type='gateway' " +
		"LEFT JOIN " + iamUserTable + " u ON u.username=c.created_by " +
		"LEFT JOIN LATERAL (SELECT jsonb_agg(net.cidr::text ORDER BY net.network_name,net.id) AS allowed_subnets " +
		"FROM t_ztna_network net WHERE net.gateway_node_id=g.id " +
		"AND net.status='active' AND net.is_del=FALSE) routes ON TRUE " +
		"ORDER BY c.created_at DESC, c.id DESC"
	var rows []credentialRow
	if err := s.SQL(query).Find(&rows); err != nil {
		return nil, fmt.Errorf("list gateway registration credentials: %w", err)
	}
	credentials := make([]model.Credential, len(rows))
	for index, row := range rows {
		var subnets []string
		if err := json.Unmarshal(row.AllowedSubnetsRaw, &subnets); err != nil {
			return nil, fmt.Errorf("decode credential gateway subnets: %w", err)
		}
		credential := model.Credential{
			ID: row.ID, NodeID: row.NodeID, GatewayID: row.GatewayID, GatewayName: row.GatewayName,
			GatewayType: row.GatewayType, NetworkName: row.NetworkName, AllowedSubnets: subnets,
			RequireApproval: row.RequireApproval, Status: row.Status, MaxUses: row.MaxUses, UsedCount: row.UsedCount,
			ExpiresAt: row.ExpiresAt, Notes: row.Notes, CreatedByName: row.CreatedByName,
			UsedAt: row.UsedAt, UsedIP: row.UsedIP, RevokedAt: row.RevokedAt, RevocationReason: row.RevocationReason,
		}
		credential.VersionNo, credential.CreatedAt, credential.UpdatedAt = row.VersionNo, row.CreatedAt, row.UpdatedAt
		credential.CreatedBy, credential.UpdatedBy = row.CreatedBy, row.UpdatedBy
		credentials[index] = credential
	}
	return credentials, nil
}

func (r *Repository) CreateGatewayAndCredential(ctx *servicecontext.Context, actorID int64, gateway *model.Gateway, credential *model.Credential, audit *model.AuditEvent) error {
	if actorID <= 0 || gateway == nil || credential == nil || len(gateway.AllowedNetworkIDs) == 0 {
		return errors.New("gateway, registration credential, and selected IPAM networks are required")
	}
	return r.db.WithSaveTransaction(ctx, func(tx *database.DBTransaction) error {
		s := tx.Session
		if err := requirePermission(s, actorID, "gateway.registration.write"); err != nil {
			return err
		}
		networks, err := lockSelectedNetworks(s, gateway.AllowedNetworkIDs, gateway.AllowedSubnets)
		if err != nil {
			return err
		}
		if gateway.NodeType == "" {
			gateway.NodeType = "gateway"
		}
		if gateway.NodeStatus == "" {
			gateway.NodeStatus = "pending"
		}
		if gateway.GatewayNetworkName == "" {
			gateway.GatewayNetworkName = gateway.NetworkName
		}
		gateway.CreateNew(ctx.UserName())
		if _, err := tx.Save(gateway); err != nil {
			return fmt.Errorf("create pre-registered gateway: %w", err)
		}
		for i := range networks {
			network := &networks[i]
			network.GatewayNodeID = &gateway.ID
			network.Update(ctx.UserName())
			network.Cols("gateway_node_id")
			network.AddSaveCondition("gateway_node_id IS NULL")
			affected, err := tx.Save(network)
			if err != nil {
				return fmt.Errorf("bind gateway to network site %d: %w", network.ID, err)
			}
			if affected != 1 {
				return fmt.Errorf("network site %d was changed or assigned to another gateway; refresh and retry", network.ID)
			}
		}
		credential.NodeID = gateway.ID
		if err := insertCredential(ctx, tx, s, gateway.ID, credential); err != nil {
			return err
		}
		if audit != nil {
			audit.TargetID = gateway.GatewayID
			audit.TargetName = gateway.GatewayName
			if err := writeAudit(ctx, tx, audit); err != nil {
				return fmt.Errorf("write gateway enrollment audit: %w", err)
			}
		}
		return nil
	})
}

func lockSelectedNetworks(s *xorm.Session, ids []int64, expectedCIDRs []string) ([]ipamModel.Network, error) {
	if len(ids) == 0 || len(ids) != len(expectedCIDRs) {
		return nil, errors.New("selected IPAM networks and gateway routes are inconsistent")
	}
	var selected []ipamModel.Network
	pls := make([]string, len(ids))
	args := make([]interface{}, len(ids))
	for i, id := range ids {
		pls[i] = "?"
		args[i] = id
	}
	query := "SELECT id, network_name, cidr::text AS cidr, gateway_node_id, created_by, created_at, updated_by, updated_at, version_no FROM t_ztna_network WHERE network_type='site' AND status='active' AND is_del=FALSE AND id IN (" + strings.Join(pls, ",") + ") ORDER BY id FOR UPDATE"
	if err := s.SQL(query, args...).Find(&selected); err != nil {
		return nil, fmt.Errorf("lock selected IPAM networks: %w", err)
	}
	if len(selected) != len(ids) {
		return nil, errors.New("one or more selected IPAM networks were disabled during gateway creation")
	}
	expected := make(map[int64]string, len(ids))
	for i, id := range ids {
		if id <= 0 {
			return nil, errors.New("selected IPAM network ID is invalid")
		}
		expected[id] = expectedCIDRs[i]
	}
	for _, network := range selected {
		if network.GatewayNodeID != nil {
			return nil, fmt.Errorf("network site %d (%s) is already assigned to a gateway; unbind it in Sites before registering another gateway", network.ID, network.NetworkName)
		}
		if expected[network.ID] == "" || expected[network.ID] != network.CIDR {
			return nil, errors.New("selected IPAM network changed during gateway creation; retry with current network data")
		}
	}
	return selected, nil
}

func insertCredential(ctx *servicecontext.Context, tx *database.DBTransaction, s *xorm.Session, nodeID int64, credential *model.Credential) error {
	var expired []model.Credential
	if err := selectManyForUpdate(s, model.TableCredential, &expired, "node_id=? AND status='pending' AND expires_at<=CURRENT_TIMESTAMP", nodeID); err != nil {
		return fmt.Errorf("load expired gateway registration credentials: %w", err)
	}
	for i := range expired {
		expired[i].Status = model.StatusExpired
		expired[i].Update(ctx.UserName())
		expired[i].Cols("status")
		expired[i].AddSaveCondition("status=?", model.StatusPending)
		expired[i].AddSaveCondition("expires_at<=CURRENT_TIMESTAMP")
		if _, err := tx.Save(&expired[i]); err != nil {
			return fmt.Errorf("expire old gateway registration credential: %w", err)
		}
	}
	var active struct {
		Count int64 `xorm:"'count'"`
	}
	if _, err := s.SQL("SELECT COUNT(*) AS count FROM "+model.TableCredential+" WHERE node_id=? AND status='pending' AND expires_at>CURRENT_TIMESTAMP", nodeID).Get(&active); err != nil {
		return fmt.Errorf("check active gateway registration credential: %w", err)
	}
	if active.Count > 0 {
		return errors.New("gateway already has an active registration credential")
	}
	credential.NodeID = nodeID
	credential.Status = model.StatusPending
	credential.MaxUses = 1
	credential.UsedCount = 0
	credential.CreateNew(ctx.UserName())
	if _, err := tx.Save(credential); err != nil {
		return fmt.Errorf("create gateway registration credential: %w", err)
	}
	return nil
}

func (r *Repository) RevokeCredential(ctx *servicecontext.Context, actorID, credentialID int64, reason string, audit *model.AuditEvent) error {
	if actorID <= 0 || credentialID <= 0 {
		return errors.New("actor and credential are required")
	}
	return r.db.WithSaveTransaction(ctx, func(tx *database.DBTransaction) error {
		s := tx.Session
		if err := requirePermission(s, actorID, "gateway.registration.write"); err != nil {
			return err
		}
		var credential model.Credential
		found, err := selectOneForUpdate(s, model.TableCredential, &credential, "id=? AND status IN ('pending','expired') AND used_count=0", credentialID)
		if err != nil {
			return fmt.Errorf("load gateway registration credential for revocation: %w", err)
		}
		if !found {
			return errors.New("unused active or expired gateway registration credential was not found")
		}
		now := time.Now().UTC()
		credential.Status, credential.RevokedAt, credential.RevocationReason = model.StatusRevoked, &now, &reason
		credential.Update(ctx.UserName())
		credential.Cols("status", "revoked_at", "revocation_reason")
		credential.AddSaveCondition("status IN ('pending','expired')")
		credential.AddSaveCondition("used_count=0")
		if _, err := tx.Save(&credential); err != nil {
			return fmt.Errorf("revoke gateway registration credential: %w", err)
		}
		if audit != nil {
			if err := setCredentialAuditTarget(s, audit, credentialID); err != nil {
				return err
			}
			if err := writeAudit(ctx, tx, audit); err != nil {
				return fmt.Errorf("write credential revocation audit: %w", err)
			}
		}
		return nil
	})
}

func (r *Repository) ExtendCredential(ctx *servicecontext.Context, actorID, credentialID int64, additionalHours int, audit *model.AuditEvent) error {
	if actorID <= 0 || credentialID <= 0 || additionalHours < 1 || additionalHours > 168 {
		return errors.New("actor, credential, and extension between 1 and 168 hours are required")
	}
	return r.db.WithSaveTransaction(ctx, func(tx *database.DBTransaction) error {
		s := tx.Session
		if err := requirePermission(s, actorID, "gateway.registration.write"); err != nil {
			return err
		}
		var credential model.Credential
		found, err := s.SQL("SELECT c.* FROM "+model.TableCredential+" c JOIN "+model.TableGateway+" g ON g.id=c.node_id WHERE c.id=? AND c.status IN ('pending','expired') AND c.used_count=0 AND g.node_type='gateway' AND g.node_status='pending' FOR UPDATE OF c", credentialID).Get(&credential)
		if err != nil {
			return fmt.Errorf("load gateway registration credential for extension: %w", err)
		}
		if !found {
			return errors.New("unused gateway registration credential cannot be extended")
		}
		var active struct {
			Count int64 `xorm:"'count'"`
		}
		if _, err := s.SQL("SELECT COUNT(*) AS count FROM "+model.TableCredential+" WHERE node_id=? AND id<>? AND status='pending' AND expires_at>CURRENT_TIMESTAMP", credential.NodeID, credentialID).Get(&active); err != nil {
			return fmt.Errorf("check active gateway registration credential: %w", err)
		}
		if active.Count > 0 {
			return errors.New("gateway already has an active registration credential")
		}
		now := time.Now().UTC()
		if credential.ExpiresAt.Before(now) {
			credential.ExpiresAt = now
		}
		credential.ExpiresAt = credential.ExpiresAt.Add(time.Duration(additionalHours) * time.Hour)
		credential.Status = model.StatusPending
		credential.Update(ctx.UserName())
		credential.Cols("status", "expires_at")
		credential.AddSaveCondition("status IN ('pending','expired')")
		credential.AddSaveCondition("used_count=0")
		if _, err := tx.Save(&credential); err != nil {
			return fmt.Errorf("extend gateway registration credential: %w", err)
		}
		if audit != nil {
			if err := setCredentialAuditTarget(s, audit, credentialID); err != nil {
				return err
			}
			if err := writeAudit(ctx, tx, audit); err != nil {
				return fmt.Errorf("write credential extension audit: %w", err)
			}
		}
		return nil
	})
}

func setCredentialAuditTarget(s *xorm.Session, audit *model.AuditEvent, credentialID int64) error {
	var target struct {
		GatewayID   string `xorm:"'gateway_id'"`
		GatewayName string `xorm:"'gateway_name'"`
	}
	query := "SELECT g.node_code AS gateway_id, g.node_name AS gateway_name FROM " + model.TableCredential + " c JOIN " + model.TableGateway + " g ON g.id=c.node_id WHERE c.id=?"
	found, err := s.SQL(query, credentialID).Get(&target)
	if err != nil {
		return fmt.Errorf("load gateway for credential audit: %w", err)
	}
	if !found {
		return errors.New("gateway registration credential was not found")
	}
	audit.TargetID = fmt.Sprintf("%d", credentialID)
	audit.TargetName = target.GatewayName + " (" + target.GatewayID + ")"
	return nil
}

func requirePermission(s *xorm.Session, actorID int64, permissionCode string) error {
	if actorID <= 0 {
		return errors.New("an authenticated operator is required")
	}
	var allowed struct {
		Count int64 `xorm:"'count'"`
	}
	query := "SELECT COUNT(*) AS count FROM " + iamUserTable + " u WHERE u.id=? AND u.account_type='platform' AND u.user_status='active' AND EXISTS (" +
		"SELECT 1 FROM " + iamUserRoleTable + " ur JOIN " + iamRoleTable + " r ON r.id=ur.role_id " +
		"JOIN " + iamRolePermissionTable + " rp ON rp.role_id=r.id JOIN " + iamPermissionTable + " p ON p.id=rp.permission_id " +
		"JOIN " + iamResourceTable + " res ON res.id=p.resource_id WHERE ur.user_id=u.id AND r.status='active' " +
		"AND p.status='active' AND res.status='active' AND p.permission_code=?)"
	if _, err := s.SQL(query, actorID, permissionCode).Get(&allowed); err != nil {
		return fmt.Errorf("check gateway enrollment permission: %w", err)
	}
	if allowed.Count < 1 {
		return errors.New("gateway registration write permission is required")
	}
	return nil
}

func writeAudit(ctx *servicecontext.Context, tx *database.DBTransaction, event *model.AuditEvent) error {
	if event.EventID == "" || event.Action == "" || event.TargetType == "" || event.ActorID <= 0 {
		return errors.New("gateway enrollment audit event is incomplete")
	}
	if len(event.ChangesJSON) == 0 {
		event.ChangesJSON = []byte(`{}`)
	}
	if event.OccurredAt.IsZero() {
		event.OccurredAt = time.Now().UTC()
	}
	actorName := event.ActorName
	if actorName == "" && ctx != nil {
		actorName = ctx.UserName()
	}
	if actorName == "" {
		actorName = fmt.Sprintf("%d", event.ActorID)
	}
	result := event.Result
	if result == "" {
		result = "success"
	}
	audit := &iamModel.AuditLog{
		EventID: event.EventID, UserID: &event.ActorID, UserName: actorName, Action: event.Action,
		TargetType: event.TargetType, TargetID: nullableStringPtr(event.TargetID), TargetName: nullableStringPtr(event.TargetName),
		Result: result, Reason: nullableStringPtr(event.Reason), SourceIP: nullableStringPtr(event.SourceIP),
		RequestID: nullableStringPtr(event.RequestID), ChangesJSON: event.ChangesJSON, OccurredAt: event.OccurredAt,
	}
	audit.CreateNew(ctx.UserName())
	_, err := tx.Save(audit)
	return err
}

// XORM's ForUpdate builder emits a MySQL-only clause. The enrollment
// repository requires PostgreSQL, so use PostgreSQL row-lock syntax directly.
func selectOneForUpdate(s *xorm.Session, table string, bean any, condition string, args ...any) (bool, error) {
	query := "SELECT * FROM " + table + " WHERE " + condition + " FOR UPDATE"
	return s.SQL(query, args...).Get(bean)
}

func selectManyForUpdate(s *xorm.Session, table string, beans any, condition string, args ...any) error {
	query := "SELECT * FROM " + table + " WHERE " + condition + " FOR UPDATE"
	return s.SQL(query, args...).Find(beans)
}

func nullableStringPtr(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}
