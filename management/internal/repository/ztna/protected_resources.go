package ztna

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/netip"
	"strings"

	servicecontext "velotrust/management/internal/context"
	model "velotrust/management/internal/model/ztna"
	"velotrust/shared/repository/database"
)

func (r *Repository) ListPolicyGroups(ctx *servicecontext.Context) ([]model.PolicyGroupOption, error) {
	if ctx == nil {
		return nil, errors.New("service context is required")
	}
	var groups []model.PolicyGroupOption
	s := r.db.Engine().NewSession().Context(ctx)
	defer s.Close()
	err := s.SQL(`SELECT group_row.id, group_row.group_name AS name, group_row.group_code AS code,
		group_row.status, COUNT(employee.id) AS member_count
		FROM public.t_iam_group AS group_row
		LEFT JOIN public.t_iam_user_group AS membership ON membership.group_id=group_row.id
		LEFT JOIN public.t_iam_user AS employee ON employee.id=membership.user_id
			AND employee.account_type='employee' AND employee.user_status='active'
		GROUP BY group_row.id ORDER BY group_row.sort_order, group_row.group_name, group_row.id`).Find(&groups)
	if err != nil {
		return nil, fmt.Errorf("list security groups for access policies: %w", err)
	}
	return groups, nil
}

func (r *Repository) ListProtectedResources(ctx *servicecontext.Context) ([]model.ProtectedResourceView, error) {
	if ctx == nil {
		return nil, errors.New("service context is required")
	}
	var rows []protectedResourceRow
	s := r.db.Engine().NewSession().Context(ctx)
	defer s.Close()
	if err := s.SQL(protectedResourceSelect + ` GROUP BY resource.id, network.id, gateway.id
		ORDER BY resource.resource_name, resource.id`).Find(&rows); err != nil {
		return nil, fmt.Errorf("list enterprise protected resources: %w", err)
	}
	return protectedResourceViews(rows)
}

func (r *Repository) SaveProtectedResource(ctx *servicecontext.Context, resource *model.ProtectedResource, expectedVersion int64) error {
	if ctx == nil || strings.TrimSpace(ctx.UserName()) == "" || resource == nil {
		return errors.New("authenticated service context and protected resource are required")
	}
	if resource.NetworkID == nil || *resource.NetworkID <= 0 {
		return errors.New("a protected resource must be associated with a network site")
	}
	return r.db.WithSaveTransaction(ctx, func(tx *database.DBTransaction) error {
		var network struct {
			ID            int64   `xorm:"'id'"`
			NetworkType   string  `xorm:"'network_type'"`
			GatewayNodeID *int64  `xorm:"'gateway_node_id'"`
			CIDR          string  `xorm:"'cidr'"`
			VirtualCIDR   *string `xorm:"'virtual_cidr'"`
		}
		found, err := tx.SQL(`SELECT network.id, network.network_type, network.gateway_node_id,
			network.cidr::text AS cidr, mapping.virtual_cidr::text AS virtual_cidr
			FROM public.t_ztna_network AS network
			LEFT JOIN public.t_ztna_nat_mapping AS mapping ON mapping.network_id=network.id AND mapping.enabled=TRUE
			WHERE network.id=? AND network.status='active' AND network.is_del=FALSE FOR KEY SHARE OF network`, *resource.NetworkID).Get(&network)
		if err != nil {
			return fmt.Errorf("validate protected resource network site: %w", err)
		}
		if !found || network.NetworkType != "site" || network.GatewayNodeID == nil || *network.GatewayNodeID <= 0 {
			return errors.New("protected resource must belong to an active site assigned to a Gateway")
		}
		if resource.Address == nil || strings.TrimSpace(*resource.Address) == "" {
			return errors.New("protected resource requires an IP address or CIDR")
		}
		address, err := netip.ParsePrefix(strings.TrimSpace(*resource.Address))
		if err != nil || address.Addr().Is4In6() {
			return errors.New("protected resource address must be a valid IP prefix")
		}
		address = address.Masked()
		realNetwork, err := netip.ParsePrefix(network.CIDR)
		if err != nil {
			return fmt.Errorf("network site has an invalid CIDR: %w", err)
		}
		within := prefixWithinManagement(address, realNetwork.Masked())
		if !within && network.VirtualCIDR != nil {
			virtualNetwork, parseErr := netip.ParsePrefix(*network.VirtualCIDR)
			within = parseErr == nil && prefixWithinManagement(address, virtualNetwork.Masked())
		}
		if !within {
			return errors.New("protected resource address must be contained in the selected site CIDR or its NAT shadow CIDR")
		}
		var gateway struct {
			ID int64 `xorm:"'id'"`
		}
		found, err = tx.SQL(`SELECT id FROM public.t_ztna_node
			WHERE id=? AND node_type='gateway' AND node_status='active' FOR KEY SHARE`, *network.GatewayNodeID).Get(&gateway)
		if err != nil {
			return fmt.Errorf("validate protected resource Gateway: %w", err)
		}
		if !found {
			return errors.New("protected resource network site has no active Gateway")
		}
		if resource.ID == 0 {
			if expectedVersion != 0 {
				return errors.New("new protected resource must not specify a version")
			}
			resource.CreateNew(ctx.UserName())
			if _, err := tx.Save(resource); err != nil {
				return fmt.Errorf("insert enterprise protected resource: %w", err)
			}
		} else {
			if expectedVersion <= 0 {
				return errors.New("protected resource version is required for update")
			}
			var current model.ProtectedResource
			found, err := tx.Session.SQL("SELECT * FROM "+model.TableProtectedResource+" WHERE id=? FOR UPDATE", resource.ID).Get(&current)
			if err != nil {
				return fmt.Errorf("lock protected resource for update: %w", err)
			}
			if !found || current.VersionNo != expectedVersion {
				return errors.New("protected resource was not found or changed; refresh and try again")
			}
			current.ResourceName = resource.ResourceName
			current.ResourceCode = resource.ResourceCode
			current.ResourceType = resource.ResourceType
			current.NetworkID = resource.NetworkID
			current.Host = resource.Host
			current.Address = resource.Address
			current.PortStart = resource.PortStart
			current.PortEnd = resource.PortEnd
			current.Protocol = resource.Protocol
			current.AccessMode = resource.AccessMode
			current.Category = resource.Category
			current.TagsJSON = append(current.TagsJSON[:0], resource.TagsJSON...)
			current.Description = resource.Description
			current.Status = resource.Status
			current.Update(ctx.UserName())
			current.Cols("resource_name", "resource_code", "resource_type", "network_id", "host", "address", "port_start", "port_end", "protocol", "access_mode", "category", "tags_json", "description", "status")
			if _, err := tx.Save(&current); err != nil {
				return fmt.Errorf("update enterprise protected resource: %w", err)
			}
			*resource = current
		}
		if err := queuePoliciesUsingResource(ctx, tx, resource.ID); err != nil {
			return err
		}
		return nil
	})
}

func prefixWithinManagement(child, parent netip.Prefix) bool {
	return child.IsValid() && parent.IsValid() && child.Addr().Is4() == parent.Addr().Is4() && child.Bits() >= parent.Bits() && parent.Contains(child.Addr())
}

func (r *Repository) DeleteProtectedResource(ctx *servicecontext.Context, id, expectedVersion int64) error {
	if ctx == nil || strings.TrimSpace(ctx.UserName()) == "" || id <= 0 || expectedVersion <= 0 {
		return errors.New("authenticated service context, resource ID, and version are required")
	}
	return r.db.WithSaveTransaction(ctx, func(tx *database.DBTransaction) error {
		var resource model.ProtectedResource
		found, err := tx.Session.SQL("SELECT * FROM "+model.TableProtectedResource+" WHERE id=? FOR UPDATE", id).Get(&resource)
		if err != nil {
			return fmt.Errorf("lock protected resource for deletion: %w", err)
		}
		if !found || resource.VersionNo != expectedVersion {
			return errors.New("protected resource was not found or changed; refresh and try again")
		}
		var used bool
		if _, err := tx.SQL(`SELECT EXISTS (SELECT 1 FROM public.t_ztna_policy_resource WHERE protected_resource_id=?)`, id).Get(&used); err != nil {
			return fmt.Errorf("check protected resource policy references: %w", err)
		}
		if used {
			return errors.New("remove this resource from all access policies before deleting it")
		}
		resource.HardDelete()
		if _, err := tx.Save(&resource); err != nil {
			return fmt.Errorf("delete enterprise protected resource: %w", err)
		}
		return nil
	})
}

const protectedResourceSelect = `SELECT resource.id, resource.version_no, resource.resource_name,
	resource.resource_code, resource.resource_type, resource.network_id,
	network.network_name, network.cidr::text AS network_cidr, network.gateway_node_id,
	gateway.node_name AS gateway_name, resource.host, resource.address, resource.port_start,
	resource.port_end, resource.protocol, resource.access_mode, resource.category,
	resource.tags_json, resource.description, resource.status,
	COUNT(DISTINCT policy_resource.policy_id) AS policy_count
	FROM public.t_ztna_protected_resource AS resource
	LEFT JOIN public.t_ztna_network AS network ON network.id=resource.network_id
	LEFT JOIN public.t_ztna_node AS gateway ON gateway.id=network.gateway_node_id
	LEFT JOIN public.t_ztna_policy_resource AS policy_resource ON policy_resource.protected_resource_id=resource.id`

type protectedResourceRow struct {
	ID            int64   `xorm:"'id'"`
	VersionNo     int64   `xorm:"'version_no'"`
	ResourceName  string  `xorm:"'resource_name'"`
	ResourceCode  string  `xorm:"'resource_code'"`
	ResourceType  string  `xorm:"'resource_type'"`
	NetworkID     *int64  `xorm:"'network_id'"`
	NetworkName   *string `xorm:"'network_name'"`
	NetworkCIDR   *string `xorm:"'network_cidr'"`
	GatewayNodeID *int64  `xorm:"'gateway_node_id'"`
	GatewayName   *string `xorm:"'gateway_name'"`
	Host          *string `xorm:"'host'"`
	Address       *string `xorm:"'address'"`
	PortStart     *int    `xorm:"'port_start'"`
	PortEnd       *int    `xorm:"'port_end'"`
	Protocol      *string `xorm:"'protocol'"`
	AccessMode    string  `xorm:"'access_mode'"`
	Category      *string `xorm:"'category'"`
	TagsJSON      []byte  `xorm:"'tags_json'"`
	Description   *string `xorm:"'description'"`
	Status        string  `xorm:"'status'"`
	PolicyCount   int64   `xorm:"'policy_count'"`
}

func protectedResourceViews(rows []protectedResourceRow) ([]model.ProtectedResourceView, error) {
	items := make([]model.ProtectedResourceView, 0, len(rows))
	for _, row := range rows {
		item := model.ProtectedResourceView{
			ID: row.ID, VersionNo: row.VersionNo, ResourceName: row.ResourceName, ResourceCode: row.ResourceCode,
			ResourceType: row.ResourceType, Host: row.Host, Address: valueOrEmpty(row.Address),
			AccessMode: row.AccessMode, Category: row.Category, Description: row.Description,
			Status: row.Status, PolicyCount: row.PolicyCount,
		}
		if row.NetworkID != nil {
			item.NetworkID = *row.NetworkID
		}
		if row.NetworkName != nil {
			item.NetworkName = *row.NetworkName
		}
		if row.NetworkCIDR != nil {
			item.NetworkCIDR = *row.NetworkCIDR
		}
		if row.GatewayNodeID != nil {
			item.GatewayNodeID = *row.GatewayNodeID
		}
		if row.GatewayName != nil {
			item.GatewayName = *row.GatewayName
		}
		if row.PortStart != nil {
			item.PortStart = *row.PortStart
		}
		if row.PortEnd != nil {
			item.PortEnd = *row.PortEnd
		}
		if row.Protocol != nil {
			item.Protocol = *row.Protocol
		}
		if len(row.TagsJSON) > 0 {
			if err := json.Unmarshal(row.TagsJSON, &item.Tags); err != nil {
				return nil, fmt.Errorf("decode tags for protected resource %d: %w", row.ID, err)
			}
		}
		if item.Tags == nil {
			item.Tags = []string{}
		}
		items = append(items, item)
	}
	return items, nil
}

func valueOrEmpty(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func queuePoliciesUsingResource(ctx *servicecontext.Context, tx *database.DBTransaction, resourceID int64) error {
	var policies []struct {
		ID int64 `xorm:"'id'"`
	}
	if err := tx.Session.SQL(`SELECT policy.id FROM public.t_ztna_access_policy AS policy
		JOIN public.t_ztna_policy_resource AS relation ON relation.policy_id=policy.id
		WHERE relation.protected_resource_id=? AND policy.status='active' ORDER BY policy.id`, resourceID).Find(&policies); err != nil {
		return fmt.Errorf("find active policies using protected resource %d: %w", resourceID, err)
	}
	for _, policy := range policies {
		if _, err := CreateAccessChange(ctx, tx, "policy", policy.ID, "authorization_changed"); err != nil {
			return fmt.Errorf("queue Gateway refresh for policy %d after resource change: %w", policy.ID, err)
		}
	}
	return nil
}
