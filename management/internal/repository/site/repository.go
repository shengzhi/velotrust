package site

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	servicecontext "velotrust/management/internal/context"
	iamModel "velotrust/management/internal/model/iam"
	ipamModel "velotrust/management/internal/model/ipam"
	model "velotrust/management/internal/model/site"
	ipamRepository "velotrust/management/internal/repository/ipam"
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
		return nil, errors.New("network-site repository requires a PostgreSQL database")
	}
	return &Repository{db: db}, nil
}

type siteRow struct {
	ID             int64     `xorm:"'id'"`
	SiteCode       string    `xorm:"'site_code'"`
	Name           string    `xorm:"'name'"`
	GatewayNodeID  *int64    `xorm:"'gateway_node_id'"`
	GatewayName    string    `xorm:"'gateway_name'"`
	Status         string    `xorm:"'status'"`
	CreatedAt      time.Time `xorm:"'created_at'"`
	VersionNo      int64     `xorm:"'version_no'"`
	LocalSubnet    string    `xorm:"'local_subnet'"`
	NodeCount      int64     `xorm:"'node_count'"`
	NATID          *int64    `xorm:"'nat_id'"`
	NATVersionNo   *int64    `xorm:"'nat_version_no'"`
	NATVirtualCIDR *string   `xorm:"'nat_virtual_cidr'"`
	NATType        *string   `xorm:"'nat_type'"`
	NATEnabled     *bool     `xorm:"'nat_enabled'"`
	NATDescription *string   `xorm:"'nat_description'"`
}

func (r *Repository) ListSites(ctx *servicecontext.Context) ([]model.PhysicalSite, error) {
	const query = `
SELECT n.id, n.network_code AS site_code, n.network_name AS name,
       n.gateway_node_id, COALESCE(g.node_name, '') AS gateway_name,
       n.status, n.created_at, n.version_no, n.cidr::text AS local_subnet,
       CASE WHEN g.id IS NULL THEN 0 ELSE 1 END AS node_count,
       nat.id AS nat_id, nat.version_no AS nat_version_no,
       nat.virtual_cidr::text AS nat_virtual_cidr, nat.nat_type,
       nat.enabled AS nat_enabled, nat.description AS nat_description
FROM t_ztna_network n
LEFT JOIN t_ztna_node g ON g.id=n.gateway_node_id AND g.node_type='gateway'
LEFT JOIN t_ztna_nat_mapping nat ON nat.network_id=n.id
WHERE n.network_type='site' AND n.is_del=FALSE
ORDER BY n.network_name, n.id`
	s := r.db.Engine().NewSession().Context(ctx)
	defer s.Close()
	var rows []siteRow
	if err := s.SQL(query).Find(&rows); err != nil {
		return nil, fmt.Errorf("list network sites: %w", err)
	}
	sites := make([]model.PhysicalSite, 0, len(rows))
	for _, row := range rows {
		item := model.PhysicalSite{
			ID: row.ID, VersionNo: row.VersionNo, SiteCode: row.SiteCode, Name: row.Name,
			GatewayNodeID: row.GatewayNodeID,
			GatewayName:   row.GatewayName, Status: row.Status, CreatedAt: row.CreatedAt,
			NetworkID: row.ID, NetworkCode: row.SiteCode, LocalSubnet: row.LocalSubnet,
			NodeCount: row.NodeCount,
		}
		if row.NATID != nil {
			version := int64(1)
			if row.NATVersionNo != nil {
				version = *row.NATVersionNo
			}
			item.NATMapping = &model.NATMapping{
				ID: *row.NATID, VersionNo: version,
				Enabled:     row.NATEnabled != nil && *row.NATEnabled,
				VirtualCIDR: row.NATVirtualCIDR, Description: row.NATDescription,
			}
			if row.NATType != nil {
				item.NATMapping.NATType = *row.NATType
			}
		}
		sites = append(sites, item)
	}
	return sites, nil
}

func (r *Repository) ListGateways(ctx *servicecontext.Context) ([]model.GatewayOption, error) {
	s := r.db.Engine().NewSession().Context(ctx)
	defer s.Close()
	var items []model.GatewayOption
	if err := s.SQL("SELECT id, node_code AS gateway_id, node_name AS gateway_name, CASE node_status WHEN 'pending' THEN 'pending_enrollment' WHEN 'active' THEN 'registered' WHEN 'disabled' THEN 'suspended' ELSE node_status END AS status FROM t_ztna_node WHERE node_type='gateway' AND node_status IN ('pending','active') ORDER BY node_name, id").Find(&items); err != nil {
		return nil, fmt.Errorf("list network-site gateway options: %w", err)
	}
	return items, nil
}

func (r *Repository) SaveSite(ctx *servicecontext.Context, actorID int64, site *model.PhysicalSite) error {
	if actorID <= 0 || site == nil {
		return errors.New("network-site record and authenticated operator are required")
	}
	return r.db.WithSaveTransaction(ctx, func(tx *database.DBTransaction) error {
		s := tx.Session
		if err := requirePermission(s, actorID, "ztna.site.write"); err != nil {
			return err
		}
		if site.ID == 0 {
			if site.GatewayNodeID != nil && site.Status != model.StatusActive {
				return errors.New("only active network sites can be assigned to a gateway")
			}
			if err := validateGateway(s, site.GatewayNodeID); err != nil {
				return err
			}
			if err := ensureSitePrefix(s, site.LocalSubnet); err != nil {
				return err
			}
			network := &ipamModel.Network{
				NetworkName: site.Name, NetworkCode: site.SiteCode, NetworkType: "site",
				GatewayNodeID: site.GatewayNodeID,
				CIDR:          site.LocalSubnet, Status: site.Status,
			}
			network.CreateNew(ctx.UserName())
			if _, err := tx.Save(network); err != nil {
				return fmt.Errorf("create network-site network: %w", err)
			}
			fillSiteFromNetwork(site, network)
			if err := setGatewayName(s, site); err != nil {
				return err
			}
			if err := saveAudit(ctx, tx, actorID, "ztna.site.create", site, map[string]any{
				"network_code": network.NetworkCode, "local_subnet": network.CIDR,
				"status": network.Status, "gateway_node_id": network.GatewayNodeID,
			}); err != nil {
				return err
			}
			return nil
		}

		var network ipamModel.Network
		found, err := lockSiteNetwork(s, site.ID, &network)
		if err != nil {
			return fmt.Errorf("lock network-site network: %w", err)
		}
		if !found || network.VersionNo != site.VersionNo {
			return errors.New("network site was not found or changed; refresh and try again")
		}
		if site.Name != network.NetworkName {
			return errors.New("site name cannot be changed after creation")
		}
		if site.LocalSubnet != network.CIDR {
			return errors.New("site CIDR cannot be changed after creation")
		}
		gatewayChanged := !sameGatewayNodeID(site.GatewayNodeID, network.GatewayNodeID)
		if gatewayChanged && site.GatewayNodeID != nil && site.Status != model.StatusActive {
			return errors.New("only active network sites can be assigned to a gateway")
		}
		if site.GatewayNodeID != nil && site.Status == model.StatusActive {
			if err := validateGateway(s, site.GatewayNodeID); err != nil {
				return err
			}
		}
		network.NetworkName = site.Name
		network.CIDR = site.LocalSubnet
		network.GatewayNodeID = site.GatewayNodeID
		network.Status = site.Status
		network.Update(ctx.UserName())
		network.Cols("network_name", "cidr", "gateway_node_id", "status")
		if _, err := tx.Save(&network); err != nil {
			return fmt.Errorf("update network-site network: %w", err)
		}
		if site.Status == model.StatusDisabled {
			if err := disableSiteNAT(s, tx, ctx, network.ID); err != nil {
				return err
			}
		}
		fillSiteFromNetwork(site, &network)
		if err := setGatewayName(s, site); err != nil {
			return err
		}
		if err := saveAudit(ctx, tx, actorID, "ztna.site.update", site, map[string]any{
			"local_subnet": network.CIDR, "status": network.Status, "gateway_node_id": network.GatewayNodeID,
		}); err != nil {
			return err
		}
		return nil
	})
}

func (r *Repository) DeleteSite(ctx *servicecontext.Context, actorID, siteID, expectedVersion int64) error {
	if actorID <= 0 || siteID <= 0 || expectedVersion <= 0 {
		return errors.New("network-site ID, version, and authenticated operator are required")
	}
	return r.db.WithSaveTransaction(ctx, func(tx *database.DBTransaction) error {
		s := tx.Session
		if err := requirePermission(s, actorID, "ztna.site.write"); err != nil {
			return err
		}
		var network ipamModel.Network
		found, err := lockSiteNetwork(s, siteID, &network)
		if err != nil {
			return fmt.Errorf("lock network-site network for deletion: %w", err)
		}
		if !found || network.VersionNo != expectedVersion {
			return errors.New("network site was not found or changed; refresh and try again")
		}
		network.Status = model.StatusDisabled
		network.GatewayNodeID = nil
		network.SoftDelete(ctx.UserName())
		network.Cols("status", "gateway_node_id")
		if _, err := tx.Save(&network); err != nil {
			return fmt.Errorf("soft-delete network-site network: %w", err)
		}
		if err := disableSiteNAT(s, tx, ctx, network.ID); err != nil {
			return err
		}
		deleted := model.PhysicalSite{}
		fillSiteFromNetwork(&deleted, &network)
		if err := saveAudit(ctx, tx, actorID, "ztna.site.delete", &deleted, map[string]any{"is_del": true}); err != nil {
			return err
		}
		return nil
	})
}

func lockSiteNetwork(s *xorm.Session, id int64, network *ipamModel.Network) (bool, error) {
	return s.SQL(`SELECT id, network_name, network_code, network_type, cidr::text AS cidr,
	       gateway_node_id, status, is_del, created_by, created_at,
       updated_by, updated_at, version_no
FROM t_ztna_network WHERE id=? AND network_type='site' AND is_del=FALSE FOR UPDATE`, id).Get(network)
}

func validateGateway(s *xorm.Session, gatewayNodeID *int64) error {
	if gatewayNodeID == nil {
		return nil
	}
	var gateway struct {
		ID int64 `xorm:"'id'"`
	}
	found, err := s.SQL("SELECT id FROM t_ztna_node WHERE id=? AND node_type='gateway' AND node_status IN ('pending','active') FOR SHARE", *gatewayNodeID).Get(&gateway)
	if err != nil {
		return fmt.Errorf("validate site gateway assignment: %w", err)
	}
	if !found {
		return errors.New("selected site gateway is unavailable")
	}
	return nil
}

func ensureSitePrefix(s *xorm.Session, cidr string) error {
	return ipamRepository.EnsureBusinessNetworkCompatible(s, cidr)
}

func sameGatewayNodeID(left, right *int64) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	return *left == *right
}

func disableSiteNAT(s *xorm.Session, tx *database.DBTransaction, ctx *servicecontext.Context, networkID int64) error {
	var mapping ipamModel.NATMapping
	found, err := s.SQL("SELECT * FROM t_ztna_nat_mapping WHERE network_id=? FOR UPDATE", networkID).Get(&mapping)
	if err != nil {
		return fmt.Errorf("lock site NAT mapping during disable: %w", err)
	}
	if found && mapping.Enabled {
		mapping.Enabled = false
		mapping.Update(ctx.UserName())
		mapping.Cols("enabled")
		if _, err := tx.Save(&mapping); err != nil {
			return fmt.Errorf("disable site NAT mapping: %w", err)
		}
	}
	return nil
}

func fillSiteFromNetwork(site *model.PhysicalSite, network *ipamModel.Network) {
	site.ID = network.ID
	site.VersionNo = network.VersionNo
	site.SiteCode = network.NetworkCode
	site.Name = network.NetworkName
	site.GatewayNodeID = network.GatewayNodeID
	site.Status = network.Status
	site.CreatedAt = network.CreatedAt
	site.NetworkID = network.ID
	site.NetworkCode = network.NetworkCode
	site.LocalSubnet = network.CIDR
}

func setGatewayName(s *xorm.Session, site *model.PhysicalSite) error {
	if site.GatewayNodeID == nil {
		site.GatewayName = ""
		site.NodeCount = 0
		return nil
	}
	var gateway struct {
		Name string `xorm:"'gateway_name'"`
	}
	if _, err := s.SQL("SELECT node_name AS gateway_name FROM t_ztna_node WHERE id=? AND node_type='gateway'", *site.GatewayNodeID).Get(&gateway); err != nil {
		return fmt.Errorf("load site gateway name: %w", err)
	}
	site.GatewayName = gateway.Name
	return nil
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
		return fmt.Errorf("check network-site permission: %w", err)
	}
	if result.Count == 0 {
		return errors.New("network-site write permission is required")
	}
	return nil
}

func saveAudit(ctx *servicecontext.Context, tx *database.DBTransaction, actorID int64, action string, site *model.PhysicalSite, changes map[string]any) error {
	encoded, err := json.Marshal(changes)
	if err != nil {
		return fmt.Errorf("encode network-site audit details: %w", err)
	}
	targetID, targetName := fmt.Sprintf("%d", site.ID), site.Name
	userName := ctx.UserName()
	if userName == "" {
		userName = fmt.Sprintf("%d", actorID)
	}
	var eventID [16]byte
	if _, err := rand.Read(eventID[:]); err != nil {
		return fmt.Errorf("generate network-site audit event ID: %w", err)
	}
	audit := &iamModel.AuditLog{
		EventID: hex.EncodeToString(eventID[:]),
		UserID:  &actorID, UserName: userName, Action: action, TargetType: "ztna_network",
		TargetID: &targetID, TargetName: &targetName, Result: "success", SourceIP: siteStringPointer(ctx.ClientIP()),
		RequestID: siteStringPointer(ctx.RequestID()), ChangesJSON: encoded, OccurredAt: time.Now().UTC(),
	}
	audit.CreateNew(userName)
	if _, err := tx.Save(audit); err != nil {
		return fmt.Errorf("write network-site audit record: %w", err)
	}
	return nil
}

func siteStringPointer(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}
