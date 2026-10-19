package ipam

import (
	"errors"
	"fmt"
	"strings"
	"time"

	servicecontext "velotrust/management/internal/context"
	iamModel "velotrust/management/internal/model/iam"
	model "velotrust/management/internal/model/ipam"
	"velotrust/shared/repository/database"
	"xorm.io/xorm"
)

const (
	userTable           = "t_iam_user"
	userRoleTable       = "t_iam_user_role"
	roleTable           = "t_iam_role"
	rolePermissionTable = "t_iam_role_permission"
	permissionTable     = "t_iam_permission"
	resourceTable       = "t_iam_resource"
)

type Repository struct{ db *database.Database }

func New(db *database.Database) (*Repository, error) {
	if db == nil || db.Engine() == nil || db.Driver() != "postgres" {
		return nil, errors.New("IPAM repository requires a PostgreSQL database")
	}
	return &Repository{db: db}, nil
}

func (r *Repository) ListNetworks(ctx *servicecontext.Context, activeOnly bool) ([]model.Network, error) {
	return r.listNetworks(ctx, activeOnly, "")
}

func (r *Repository) ListActiveSiteNetworks(ctx *servicecontext.Context) ([]model.Network, error) {
	return r.listNetworks(ctx, true, "site")
}

func (r *Repository) ListSelectableSiteNetworks(ctx *servicecontext.Context) ([]model.Network, error) {
	return r.ListActiveSiteNetworks(ctx)
}

func (r *Repository) listNetworks(ctx *servicecontext.Context, activeOnly bool, networkType string) ([]model.Network, error) {
	query := "SELECT id, network_name, network_code, network_type, cidr::text AS cidr, gateway_node_id, region, environment, description, status, is_del, created_at, updated_at, version_no FROM " + model.TableNetwork
	conditions := []string{"is_del=FALSE"}
	args := make([]interface{}, 0, 2)
	if activeOnly {
		conditions = append(conditions, "status='active'")
	}
	if networkType != "" {
		conditions = append(conditions, "network_type=?")
		args = append(args, networkType)
	}
	if len(conditions) > 0 {
		query += " WHERE " + strings.Join(conditions, " AND ")
	}
	query += " ORDER BY network_name, id"
	s := r.db.Engine().NewSession().Context(ctx)
	defer s.Close()
	var networks []model.Network
	if err := s.SQL(query, args...).Find(&networks); err != nil {
		return nil, fmt.Errorf("list IPAM networks: %w", err)
	}
	return networks, nil
}

func (r *Repository) ResolveActiveNetworks(ctx *servicecontext.Context, ids []int64) ([]model.Network, error) {
	return r.resolveActiveNetworks(ctx, ids, "")
}

func (r *Repository) ResolveActiveSiteNetworks(ctx *servicecontext.Context, ids []int64) ([]model.Network, error) {
	return r.resolveActiveNetworks(ctx, ids, "site")
}

func (r *Repository) resolveActiveNetworks(ctx *servicecontext.Context, ids []int64, networkType string) ([]model.Network, error) {
	if len(ids) == 0 || len(ids) > 32 {
		return nil, errors.New("select between 1 and 32 routed networks")
	}
	unique := make(map[int64]struct{}, len(ids))
	for _, id := range ids {
		if id <= 0 {
			return nil, errors.New("IPAM network IDs must be positive")
		}
		if _, ok := unique[id]; ok {
			return nil, errors.New("duplicate IPAM network IDs are not allowed")
		}
		unique[id] = struct{}{}
	}
	s := r.db.Engine().NewSession().Context(ctx)
	defer s.Close()
	var networks []model.Network
	pls, args := placeholders(ids)
	query := "SELECT id, network_name, network_code, network_type, cidr::text AS cidr, gateway_node_id, region, environment, description, status, is_del, created_at, updated_at, version_no FROM " + model.TableNetwork + " WHERE status='active' AND is_del=FALSE AND id IN (" + pls + ")"
	if networkType != "" {
		query += " AND network_type=?"
		args = append(args, networkType)
	}
	query += " ORDER BY id"
	if err := s.SQL(query, args...).Find(&networks); err != nil {
		return nil, fmt.Errorf("resolve selected active routed networks: %w", err)
	}
	if len(networks) != len(ids) {
		return nil, errors.New("one or more selected networks are unavailable, disabled, or are not network-site networks")
	}
	return networks, nil
}

func placeholders(ids []int64) (string, []interface{}) {
	parts := make([]string, len(ids))
	args := make([]interface{}, len(ids))
	for i, id := range ids {
		parts[i] = "?"
		args[i] = id
	}
	return strings.Join(parts, ","), args
}

func (r *Repository) CreateNetwork(ctx *servicecontext.Context, actorID int64, network *model.Network, audit *model.AuditEvent) error {
	if actorID <= 0 || network == nil || audit == nil {
		return errors.New("IPAM network and audit event are required")
	}
	return r.db.WithSaveTransaction(ctx, func(tx *database.DBTransaction) error {
		s := tx.Session
		if err := lockIPAMCatalog(s); err != nil {
			return err
		}
		if err := requirePermission(s, actorID, "ipam.network.write"); err != nil {
			return err
		}
		if err := assertBusinessNetworkCompatible(s, network.CIDR); err != nil {
			return err
		}
		network.Status = model.StatusActive
		network.CreateNew(ctx.UserName())
		if _, err := tx.Save(network); err != nil {
			return fmt.Errorf("create IPAM network: %w", err)
		}
		audit.TargetID = fmt.Sprintf("%d", network.ID)
		audit.TargetName = network.NetworkName
		if err := writeAudit(ctx, tx, audit); err != nil {
			return fmt.Errorf("write IPAM network audit: %w", err)
		}
		return nil
	})
}

func (r *Repository) SetNetworkStatus(ctx *servicecontext.Context, actorID, networkID int64, status string, expectedVersion int64, audit *model.AuditEvent) error {
	if actorID <= 0 || networkID <= 0 || expectedVersion <= 0 || (status != model.StatusActive && status != model.StatusDisabled) || audit == nil {
		return errors.New("valid IPAM network status update and audit event are required")
	}
	return r.db.WithSaveTransaction(ctx, func(tx *database.DBTransaction) error {
		s := tx.Session
		if err := lockIPAMCatalog(s); err != nil {
			return err
		}
		if err := requirePermission(s, actorID, "ipam.network.write"); err != nil {
			return err
		}
		var network model.Network
		found, err := s.SQL("SELECT id, network_name, network_code, network_type, cidr::text AS cidr, gateway_node_id, status, is_del, version_no FROM "+model.TableNetwork+" WHERE id=? AND is_del=FALSE FOR UPDATE", networkID).Get(&network)
		if err != nil {
			return fmt.Errorf("lock IPAM network for update: %w", err)
		}
		if !found || network.VersionNo != expectedVersion {
			return errors.New("IPAM network was not found or has changed; refresh and try again")
		}
		if network.NetworkType == "site" {
			return errors.New("network-site network status must be managed from the Sites service")
		}
		if status == model.StatusDisabled && network.GatewayNodeID != nil {
			return errors.New("network is assigned to a gateway; unbind the gateway before disabling it")
		}
		if status == model.StatusActive {
			if err := assertBusinessNetworkCompatible(s, network.CIDR); err != nil {
				return err
			}
		}
		if network.Status == status {
			return nil
		}
		network.Status = status
		network.Update(ctx.UserName())
		network.Cols("status")
		if _, err := tx.Save(&network); err != nil {
			return fmt.Errorf("update IPAM network status: %w", err)
		}
		audit.TargetID = fmt.Sprintf("%d", network.ID)
		audit.TargetName = network.NetworkName
		audit.ChangesJSON = []byte(fmt.Sprintf(`{"status":"%s"}`, status))
		if err := writeAudit(ctx, tx, audit); err != nil {
			return fmt.Errorf("write IPAM network audit: %w", err)
		}
		return nil
	})
}

func requirePermission(s *xorm.Session, actorID int64, permissionCode string) error {
	if actorID <= 0 {
		return errors.New("an authenticated operator is required")
	}
	var allowed struct {
		Count int64 `xorm:"'count'"`
	}
	query := "SELECT COUNT(*) AS count FROM " + userTable + " u WHERE u.id=? AND u.user_status='active' AND EXISTS (" +
		"SELECT 1 FROM " + userRoleTable + " ur JOIN " + roleTable + " r ON r.id=ur.role_id " +
		"JOIN " + rolePermissionTable + " rp ON rp.role_id=r.id JOIN " + permissionTable + " p ON p.id=rp.permission_id " +
		"JOIN " + resourceTable + " res ON res.id=p.resource_id WHERE ur.user_id=u.id AND r.status='active' " +
		"AND p.status='active' AND res.status='active' AND p.permission_code=?)"
	if _, err := s.SQL(query, actorID, permissionCode).Get(&allowed); err != nil {
		return fmt.Errorf("check IPAM permission: %w", err)
	}
	if allowed.Count < 1 {
		if permissionCode == "ztna.site.write" {
			return errors.New("network-site write permission is required")
		}
		return errors.New("IPAM network write permission is required")
	}
	return nil
}

func writeAudit(ctx *servicecontext.Context, tx *database.DBTransaction, event *model.AuditEvent) error {
	if strings.TrimSpace(event.EventID) == "" || event.ActorID <= 0 || strings.TrimSpace(event.Action) == "" {
		return errors.New("IPAM audit event is incomplete")
	}
	if len(event.ChangesJSON) == 0 {
		event.ChangesJSON = []byte(`{}`)
	}
	if event.OccurredAt.IsZero() {
		event.OccurredAt = time.Now().UTC()
	}
	targetType := event.TargetType
	if targetType == "" {
		targetType = "ztna_network"
	}
	actor := ctx.UserName()
	if actor == "" {
		actor = fmt.Sprintf("%d", event.ActorID)
	}
	audit := &iamModel.AuditLog{
		EventID: event.EventID, UserID: &event.ActorID, UserName: actor, Action: event.Action,
		TargetType: targetType, TargetID: &event.TargetID, TargetName: &event.TargetName,
		Result: "success", ChangesJSON: event.ChangesJSON, OccurredAt: event.OccurredAt,
	}
	audit.CreateNew(ctx.UserName())
	_, err := tx.Save(audit)
	return err
}
