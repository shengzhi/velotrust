package ipam

import (
	"errors"
	"fmt"
	"math/bits"
	"net/netip"
	"strings"
	"time"

	servicecontext "velotrust/management/internal/context"
	model "velotrust/management/internal/model/ipam"
	"velotrust/shared/repository/database"
	"xorm.io/xorm"
)

const ipamCatalogLock int64 = 0x4950414d

func (r *Repository) GetOverlayNetwork(ctx *servicecontext.Context) (*model.OverlayNetwork, error) {
	s := r.db.Engine().NewSession().Context(ctx)
	defer s.Close()
	var network model.OverlayNetwork
	found, err := s.SQL("SELECT id, network_cidr::text AS network_cidr, created_at, updated_at, version_no FROM t_ztna_overlay_network WHERE id=1").Get(&network)
	if err != nil {
		return nil, fmt.Errorf("load global Overlay network: %w", err)
	}
	if !found {
		return nil, nil
	}
	return &network, nil
}

func (r *Repository) SaveOverlayNetwork(ctx *servicecontext.Context, actorID int64, cidr string, expectedVersion int64, audit *model.AuditEvent) (*model.OverlayNetwork, error) {
	if actorID <= 0 || audit == nil || expectedVersion < 0 {
		return nil, errors.New("global Overlay network configuration and audit event are required")
	}
	prefix, err := netip.ParsePrefix(cidr)
	if err != nil || !prefix.Addr().Is4() || prefix.Bits() < 8 || prefix.Bits() > 16 || prefix.Addr().As4()[0] != 10 {
		return nil, errors.New("global Overlay CIDR must be a private 10.0.0.0/8 prefix between /8 and /16")
	}
	cidr = prefix.Masked().String()
	var saved model.OverlayNetwork
	err = r.db.WithSaveTransaction(ctx, func(tx *database.DBTransaction) error {
		s := tx.Session
		if err := lockIPAMCatalog(s); err != nil {
			return err
		}
		if err := requirePermission(s, actorID, "ipam.network.write"); err != nil {
			return err
		}
		var current model.OverlayNetwork
		found, err := s.SQL("SELECT id, network_cidr::text AS network_cidr, created_by, created_at, updated_at, version_no FROM t_ztna_overlay_network WHERE id=1 FOR UPDATE").Get(&current)
		if err != nil {
			return fmt.Errorf("lock global Overlay network: %w", err)
		}
		if !found {
			if expectedVersion != 0 {
				return errors.New("global Overlay network is not initialized; refresh and try again")
			}
			if err := assertOverlayCIDRCompatible(s, cidr); err != nil {
				return err
			}
			saved = model.OverlayNetwork{ID: 1, NetworkCIDR: cidr}
			saved.CreateNew(ctx.UserName())
			if _, err := tx.Save(&saved); err != nil {
				return fmt.Errorf("initialize global Overlay network: %w", err)
			}
		} else {
			if expectedVersion == 0 || expectedVersion != current.VersionNo {
				return errors.New("global Overlay network changed; refresh and try again")
			}
			if current.NetworkCIDR == cidr {
				saved = current
				return nil
			}
			return errors.New("global Overlay network is immutable after initialization; its CIDR cannot be changed")
		}
		audit.TargetType = "ztna_overlay_network"
		audit.TargetID = "1"
		audit.TargetName = "Global Overlay"
		audit.ChangesJSON = []byte(fmt.Sprintf(`{"cidr":"%s","version_no":%d}`, cidr, saved.VersionNo))
		if err := writeAudit(ctx, tx, audit); err != nil {
			return fmt.Errorf("write global Overlay network audit: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &saved, nil
}

func assertOverlayCIDRCompatible(s *xorm.Session, cidr string) error {
	candidate, err := netip.ParsePrefix(cidr)
	if err != nil || !candidate.Addr().Is4() || candidate.Bits() < 8 || candidate.Bits() > 16 || candidate.Addr().As4()[0] != 10 {
		return errors.New("global Overlay CIDR must be a private 10.0.0.0/8 prefix between /8 and /16")
	}
	type row struct {
		Name string `xorm:"'name'"`
		CIDR string `xorm:"'cidr'"`
	}
	checks := []struct {
		query string
		args  []interface{}
		pool  bool
	}{
		{"SELECT network_name AS name, cidr::text AS cidr FROM t_ztna_network WHERE status='active' AND is_del=FALSE", nil, false},
		{"SELECT pool_name AS name, network_cidr::text AS cidr FROM t_ztna_address_pool", nil, true},
		{"SELECT network_name || ' NAT 影子网段' AS name, virtual_cidr::text AS cidr FROM t_ztna_nat_mapping WHERE enabled=TRUE AND virtual_cidr IS NOT NULL", nil, false},
	}
	for _, check := range checks {
		var rows []row
		if err := s.SQL(check.query, check.args...).Find(&rows); err != nil {
			return fmt.Errorf("check global Overlay CIDR compatibility: %w", err)
		}
		for _, existing := range rows {
			prefix, parseErr := netip.ParsePrefix(existing.CIDR)
			if parseErr != nil {
				return fmt.Errorf("stored IPAM range %q has invalid CIDR", existing.Name)
			}
			prefix = prefix.Masked()
			if check.pool {
				if !candidate.Contains(prefix.Addr()) || candidate.Bits() > prefix.Bits() {
					return fmt.Errorf("global Overlay CIDR %s must contain existing address pool %s (%s)", candidate, existing.Name, prefix)
				}
				continue
			}
			if prefixesOverlap(candidate, prefix) {
				return fmt.Errorf("global Overlay CIDR %s overlaps %s (%s)", candidate, existing.Name, prefix)
			}
		}
	}
	return nil
}

func (r *Repository) ListPools(ctx *servicecontext.Context) ([]model.Pool, error) {
	query := "SELECT p.id, p.pool_name, p.network_cidr::text AS network_cidr, p.pool_type, p.allocation_mode, p.idp_group, p.client_group_id, group_row.group_name AS client_group_name, p.gateway_binding, p.color, p.description, p.status, " +
		"COUNT(l.id) FILTER (WHERE l.status IN ('active','reserved')) AS used, p.created_at, p.updated_at, p.version_no " +
		"FROM t_ztna_address_pool p LEFT JOIN t_ztna_address_lease l ON l.pool_id=p.id " +
		"LEFT JOIN t_iam_group group_row ON group_row.id=p.client_group_id GROUP BY p.id, group_row.group_name ORDER BY p.pool_name, p.id"
	s := r.db.Engine().NewSession().Context(ctx)
	defer s.Close()
	type poolRow struct {
		ID              int64     `xorm:"'id'"`
		PoolName        string    `xorm:"'pool_name'"`
		NetworkCIDR     string    `xorm:"'network_cidr'"`
		Type            string    `xorm:"'pool_type'"`
		AllocationMode  string    `xorm:"'allocation_mode'"`
		IDPGroup        *string   `xorm:"'idp_group'"`
		ClientGroupID   *int64    `xorm:"'client_group_id'"`
		ClientGroupName string    `xorm:"'client_group_name'"`
		GatewayBinding  *string   `xorm:"'gateway_binding'"`
		Color           string    `xorm:"'color'"`
		Description     *string   `xorm:"'description'"`
		Status          string    `xorm:"'status'"`
		Used            int64     `xorm:"'used'"`
		CreatedAt       time.Time `xorm:"'created_at'"`
		UpdatedAt       time.Time `xorm:"'updated_at'"`
		VersionNo       int64     `xorm:"'version_no'"`
	}
	var rows []poolRow
	if err := s.SQL(query).Find(&rows); err != nil {
		return nil, fmt.Errorf("list overlay address pools: %w", err)
	}
	pools := make([]model.Pool, len(rows))
	for i, row := range rows {
		pools[i] = model.Pool{
			ID: row.ID, PoolName: row.PoolName, NetworkCIDR: row.NetworkCIDR, Type: row.Type,
			AllocationMode: row.AllocationMode, IDPGroup: row.IDPGroup, ClientGroupID: row.ClientGroupID,
			ClientGroupName: row.ClientGroupName, GatewayBinding: row.GatewayBinding,
			Color: row.Color, Description: row.Description, Status: row.Status, Used: row.Used,
		}
		pools[i].VersionNo, pools[i].CreatedAt, pools[i].UpdatedAt = row.VersionNo, row.CreatedAt, row.UpdatedAt
	}
	return pools, nil
}

func (r *Repository) CreatePool(ctx *servicecontext.Context, actorID int64, pool *model.Pool, audit *model.AuditEvent) error {
	if actorID <= 0 || pool == nil || audit == nil {
		return errors.New("IPAM pool and audit event are required")
	}
	return r.db.WithSaveTransaction(ctx, func(tx *database.DBTransaction) error {
		s := tx.Session
		if err := lockIPAMCatalog(s); err != nil {
			return err
		}
		if err := requirePermission(s, actorID, "ipam.network.write"); err != nil {
			return err
		}
		if err := assertPoolCIDRAvailable(s, pool.NetworkCIDR, 0); err != nil {
			return err
		}
		if err := validatePoolClientGroup(s, pool, 0); err != nil {
			return err
		}
		pool.Status = model.StatusActive
		pool.CreateNew(ctx.UserName())
		if _, err := tx.Save(pool); err != nil {
			return fmt.Errorf("create overlay address pool: %w", err)
		}
		audit.TargetType = "ztna_address_pool"
		audit.TargetID = fmt.Sprintf("%d", pool.ID)
		audit.TargetName = pool.PoolName
		if err := writeAudit(ctx, tx, audit); err != nil {
			return fmt.Errorf("write address pool audit: %w", err)
		}
		return nil
	})
}

func (r *Repository) UpdatePool(ctx *servicecontext.Context, actorID int64, pool *model.Pool, audit *model.AuditEvent) error {
	if actorID <= 0 || pool == nil || pool.ID <= 0 || pool.VersionNo <= 0 || audit == nil {
		return errors.New("IPAM pool update and audit event are required")
	}
	return r.db.WithSaveTransaction(ctx, func(tx *database.DBTransaction) error {
		s := tx.Session
		if err := lockIPAMCatalog(s); err != nil {
			return err
		}
		if err := requirePermission(s, actorID, "ipam.network.write"); err != nil {
			return err
		}
		var current model.Pool
		found, err := s.SQL("SELECT id, pool_name, network_cidr::text AS network_cidr, version_no FROM t_ztna_address_pool WHERE id=? FOR UPDATE", pool.ID).Get(&current)
		if err != nil {
			return fmt.Errorf("lock overlay address pool: %w", err)
		}
		if !found || current.VersionNo != pool.VersionNo {
			return errors.New("IPAM pool was not found or changed; refresh and try again")
		}
		if current.NetworkCIDR != pool.NetworkCIDR {
			var leases struct {
				Count int64 `xorm:"'count'"`
			}
			if _, err := s.SQL("SELECT COUNT(*) AS count FROM t_ztna_address_lease WHERE pool_id=?", pool.ID).Get(&leases); err != nil {
				return fmt.Errorf("check pool address history before changing its CIDR: %w", err)
			}
			if leases.Count > 0 {
				return errors.New("an address pool CIDR cannot change after address leases have been allocated")
			}
			if err := assertPoolCIDRAvailable(s, pool.NetworkCIDR, pool.ID); err != nil {
				return err
			}
		}
		if err := validatePoolClientGroup(s, pool, pool.ID); err != nil {
			return err
		}
		pool.Update(ctx.UserName())
		pool.Cols("pool_name", "network_cidr", "pool_type", "allocation_mode", "idp_group", "client_group_id", "gateway_binding", "color", "description")
		if _, err := tx.Save(pool); err != nil {
			return fmt.Errorf("update overlay address pool: %w", err)
		}
		audit.TargetType = "ztna_address_pool"
		audit.TargetID = fmt.Sprintf("%d", pool.ID)
		audit.TargetName = pool.PoolName
		if err := writeAudit(ctx, tx, audit); err != nil {
			return fmt.Errorf("write address pool audit: %w", err)
		}
		return nil
	})
}

func validatePoolClientGroup(session *xorm.Session, pool *model.Pool, currentPoolID int64) error {
	if pool == nil || pool.ClientGroupID == nil {
		return nil
	}
	groupID := *pool.ClientGroupID
	if groupID <= 0 || pool.Type != "User-Device" || pool.AllocationMode != "dynamic" {
		return errors.New("Client automatic address assignment requires an active employee group and a dynamic User-Device pool")
	}
	var group struct {
		ID int64 `xorm:"'id'"`
	}
	found, err := session.SQL("SELECT id FROM t_iam_group WHERE id=? AND status='active' FOR SHARE", groupID).Get(&group)
	if err != nil {
		return fmt.Errorf("validate active employee group for address pool: %w", err)
	}
	if !found {
		return errors.New("selected employee group is missing or disabled")
	}
	var other struct {
		PoolName string `xorm:"'pool_name'"`
	}
	found, err = session.SQL("SELECT pool_name FROM t_ztna_address_pool WHERE client_group_id=? AND id<>? LIMIT 1", groupID, currentPoolID).Get(&other)
	if err != nil {
		return fmt.Errorf("check employee group address pool uniqueness: %w", err)
	}
	if found {
		return fmt.Errorf("employee group is already associated with address pool %q", other.PoolName)
	}
	return nil
}

func (r *Repository) SetPoolStatus(ctx *servicecontext.Context, actorID, poolID int64, status string, expectedVersion int64, audit *model.AuditEvent) error {
	if actorID <= 0 || poolID <= 0 || expectedVersion <= 0 || (status != model.StatusActive && status != model.StatusDisabled) || audit == nil {
		return errors.New("valid IPAM pool status update and audit event are required")
	}
	return r.db.WithSaveTransaction(ctx, func(tx *database.DBTransaction) error {
		s := tx.Session
		if err := lockIPAMCatalog(s); err != nil {
			return err
		}
		if err := requirePermission(s, actorID, "ipam.network.write"); err != nil {
			return err
		}
		var pool model.Pool
		found, err := s.SQL("SELECT id, pool_name, status, version_no FROM t_ztna_address_pool WHERE id=? FOR UPDATE", poolID).Get(&pool)
		if err != nil {
			return fmt.Errorf("lock address pool for status update: %w", err)
		}
		if !found || pool.VersionNo != expectedVersion {
			return errors.New("IPAM pool was not found or changed; refresh and try again")
		}
		if pool.Status == status {
			return nil
		}
		pool.Status = status
		pool.Update(ctx.UserName())
		pool.Cols("status")
		if _, err := tx.Save(&pool); err != nil {
			return fmt.Errorf("update address pool status: %w", err)
		}
		audit.TargetType = "ztna_address_pool"
		audit.TargetID, audit.TargetName = fmt.Sprintf("%d", poolID), pool.PoolName
		audit.ChangesJSON = []byte(fmt.Sprintf(`{"status":"%s"}`, status))
		if err := writeAudit(ctx, tx, audit); err != nil {
			return fmt.Errorf("write address pool status audit: %w", err)
		}
		return nil
	})
}

func (r *Repository) ListLeases(ctx *servicecontext.Context) ([]model.Lease, error) {
	query := "SELECT l.id, l.pool_id, host(l.address) AS address, l.node_type, l.target_name, COALESCE(l.endpoint,'') AS endpoint, l.allocation_mode, l.idp_group, COALESCE(l.wireguard_pubkey,'') AS wireguard_pubkey, CASE l.status WHEN 'active' THEN 'Active' WHEN 'reserved' THEN 'Reserved' ELSE 'Released' END AS status, p.pool_name, l.allocated_at, l.released_at FROM t_ztna_address_lease l JOIN t_ztna_address_pool p ON p.id=l.pool_id ORDER BY l.allocated_at DESC, l.id DESC LIMIT 5000"
	s := r.db.Engine().NewSession().Context(ctx)
	defer s.Close()
	type leaseRow struct {
		ID              int64      `xorm:"'id'"`
		PoolID          int64      `xorm:"'pool_id'"`
		VirtualIP       string     `xorm:"'address'"`
		NodeType        string     `xorm:"'node_type'"`
		TargetName      string     `xorm:"'target_name'"`
		Endpoint        string     `xorm:"'endpoint'"`
		AllocationMode  string     `xorm:"'allocation_mode'"`
		IDPGroup        *string    `xorm:"'idp_group'"`
		WireGuardPubKey string     `xorm:"'wireguard_pubkey'"`
		Status          string     `xorm:"'status'"`
		PoolName        string     `xorm:"'pool_name'"`
		AllocatedAt     time.Time  `xorm:"'allocated_at'"`
		ReleasedAt      *time.Time `xorm:"'released_at'"`
	}
	var rows []leaseRow
	if err := s.SQL(query).Find(&rows); err != nil {
		return nil, fmt.Errorf("list IP address leases: %w", err)
	}
	leases := make([]model.Lease, len(rows))
	for i, row := range rows {
		leases[i] = model.Lease{ID: row.ID, PoolID: row.PoolID, VirtualIP: row.VirtualIP, NodeType: row.NodeType,
			TargetName: row.TargetName, Endpoint: row.Endpoint, AllocationMode: row.AllocationMode, IDPGroup: row.IDPGroup,
			WireGuardPubKey: row.WireGuardPubKey, Status: row.Status, PoolName: row.PoolName, AllocatedAt: row.AllocatedAt, ReleasedAt: row.ReleasedAt}
	}
	return leases, nil
}

func (r *Repository) AllocateAddress(ctx *servicecontext.Context, actorID int64, lease *model.Lease, audit *model.AuditEvent) error {
	if actorID <= 0 || lease == nil || audit == nil || lease.PoolID <= 0 || lease.TargetName == "" {
		return errors.New("IP address allocation and audit event are required")
	}
	return r.db.WithSaveTransaction(ctx, func(tx *database.DBTransaction) error {
		s := tx.Session
		if err := requirePermission(s, actorID, "ipam.network.write"); err != nil {
			return err
		}
		var pool struct {
			ID             int64  `xorm:"'id'"`
			PoolName       string `xorm:"'pool_name'"`
			NetworkCIDR    string `xorm:"'network_cidr'"`
			AllocationMode string `xorm:"'allocation_mode'"`
			Status         string `xorm:"'status'"`
		}
		found, err := s.SQL("SELECT id, pool_name, network_cidr::text AS network_cidr, allocation_mode, status FROM t_ztna_address_pool WHERE id=? FOR UPDATE", lease.PoolID).Get(&pool)
		if err != nil {
			return fmt.Errorf("lock IPAM pool for allocation: %w", err)
		}
		if !found || pool.Status != model.StatusActive {
			return errors.New("selected IPAM pool is unavailable or disabled")
		}
		prefix, err := parseAllocatablePool(pool.NetworkCIDR)
		if err != nil {
			return err
		}
		var existingTarget struct {
			ID int64 `xorm:"'id'"`
		}
		found, err = s.SQL("SELECT id FROM t_ztna_address_lease WHERE pool_id=? AND target_name=? AND status IN ('active','reserved')", lease.PoolID, lease.TargetName).Get(&existingTarget)
		if err != nil {
			return fmt.Errorf("check existing IP lease for target: %w", err)
		}
		if found {
			return errors.New("this target already has an active IP lease in the selected pool")
		}
		var usedRows []struct {
			Address string `xorm:"'address'"`
		}
		if err := s.SQL("SELECT host(address) AS address FROM t_ztna_address_lease WHERE pool_id=? AND status IN ('active','reserved')", lease.PoolID).Find(&usedRows); err != nil {
			return fmt.Errorf("load occupied addresses: %w", err)
		}
		used := make(map[netip.Addr]struct{}, len(usedRows))
		for _, row := range usedRows {
			address, parseErr := netip.ParseAddr(row.Address)
			if parseErr == nil {
				used[address] = struct{}{}
			}
		}
		var address netip.Addr
		if lease.VirtualIP != "" {
			address, err = netip.ParseAddr(lease.VirtualIP)
			if err != nil || !address.Is4() || !prefix.Contains(address) {
				return errors.New("requested IP address must be an IPv4 host inside the selected pool")
			}
			if _, occupied := used[address]; occupied {
				return errors.New("requested IP address is already allocated")
			}
			bytes := prefix.Addr().As4()
			network := uint64(bytes[0])<<24 | uint64(bytes[1])<<16 | uint64(bytes[2])<<8 | uint64(bytes[3])
			last := network + (uint64(1) << uint(32-prefix.Bits())) - 1
			if address == prefix.Addr() || address == netip.AddrFrom4([4]byte{byte(last >> 24), byte(last >> 16), byte(last >> 8), byte(last)}) {
				return errors.New("requested IP address cannot be the network or broadcast address")
			}
		} else {
			address, err = firstFreeAddress(prefix, used)
			if err != nil {
				return err
			}
		}
		now := time.Now().UTC()
		lease.VirtualIP = address.String()
		lease.PoolName = pool.PoolName
		lease.AllocatedAt = now
		lease.Status = "active"
		if lease.WireGuardPubKey != "" {
			pubKey := lease.WireGuardPubKey
			lease.WireGuardPubKeyRaw = &pubKey
		}
		lease.CreateNew(ctx.UserName())
		if _, err := tx.Save(lease); err != nil {
			return fmt.Errorf("allocate overlay address: %w", err)
		}
		lease.Status = "Active"
		audit.TargetType = "ztna_address_lease"
		audit.TargetID, audit.TargetName = fmt.Sprintf("%d", lease.ID), lease.TargetName
		audit.ChangesJSON = []byte(fmt.Sprintf(`{"pool_id":%d,"address":"%s","node_type":"%s"}`, lease.PoolID, lease.VirtualIP, lease.NodeType))
		if err := writeAudit(ctx, tx, audit); err != nil {
			return fmt.Errorf("write IP address allocation audit: %w", err)
		}
		return nil
	})
}

func (r *Repository) ReleaseLease(ctx *servicecontext.Context, actorID, leaseID int64, audit *model.AuditEvent) error {
	if actorID <= 0 || leaseID <= 0 || audit == nil {
		return errors.New("lease release and audit event are required")
	}
	return r.db.WithSaveTransaction(ctx, func(tx *database.DBTransaction) error {
		s := tx.Session
		if err := requirePermission(s, actorID, "ipam.network.write"); err != nil {
			return err
		}
		var lease model.Lease
		found, err := s.SQL("SELECT id, pool_id, host(address) AS address, node_type, target_name, COALESCE(endpoint,'') AS endpoint, allocation_mode, idp_group, wireguard_pubkey, status, allocated_at, released_at, created_at, updated_at, version_no, created_by, updated_by FROM t_ztna_address_lease WHERE id=? FOR UPDATE", leaseID).Get(&lease)
		if err != nil {
			return fmt.Errorf("lock IP address lease for release: %w", err)
		}
		if !found {
			return errors.New("IP address lease was not found")
		}
		if lease.Status == "released" {
			return nil
		}
		now := time.Now().UTC()
		lease.Status, lease.ReleasedAt = "released", &now
		lease.Update(ctx.UserName())
		lease.Cols("status", "released_at")
		lease.AddSaveCondition("status IN ('active','reserved')")
		if _, err := tx.Save(&lease); err != nil {
			return fmt.Errorf("release IP address lease: %w", err)
		}
		audit.TargetType = "ztna_address_lease"
		audit.TargetID, audit.TargetName = fmt.Sprintf("%d", leaseID), lease.TargetName
		audit.ChangesJSON = []byte(fmt.Sprintf(`{"address":"%s","status":"released"}`, lease.VirtualIP))
		if err := writeAudit(ctx, tx, audit); err != nil {
			return fmt.Errorf("write IP address release audit: %w", err)
		}
		return nil
	})
}

func (r *Repository) CheckPrefix(ctx *servicecontext.Context, cidr string) (model.Conflict, error) {
	prefix, err := netip.ParsePrefix(strings.TrimSpace(cidr))
	if err != nil || prefix.Bits() == 0 {
		return model.Conflict{IsValid: false, Message: "无效 CIDR；请输入带前缀长度的非默认网段。"}, nil
	}
	prefix = prefix.Masked()
	type candidate struct{ Name, CIDR string }
	var existing []candidate
	queries := []string{
		"SELECT network_name AS name, cidr::text AS cidr FROM t_ztna_network WHERE status='active' AND is_del=FALSE",
		"SELECT pool_name AS name, network_cidr::text AS cidr FROM t_ztna_address_pool WHERE status='active'",
		"SELECT network_name || ' NAT 影子网段' AS name, virtual_cidr::text AS cidr FROM t_ztna_nat_mapping WHERE enabled=TRUE AND virtual_cidr IS NOT NULL",
	}
	s := r.db.Engine().NewSession().Context(ctx)
	defer s.Close()
	for _, query := range queries {
		var rows []candidate
		if err := s.SQL(query).Find(&rows); err != nil {
			return model.Conflict{}, fmt.Errorf("read IPAM prefixes for conflict check: %w", err)
		}
		existing = append(existing, rows...)
	}
	for _, item := range existing {
		other, parseErr := netip.ParsePrefix(item.CIDR)
		if parseErr == nil && prefixesOverlap(prefix, other.Masked()) {
			return model.Conflict{IsValid: true, HasConflict: true, ConflictTarget: item.Name, Message: fmt.Sprintf("网段 %s 与已登记网络 %s (%s) 重叠；请更换网段或先规划 NAT 映射。", prefix, item.Name, other.Masked())}, nil
		}
	}
	configuredOverlay, err := r.GetOverlayNetwork(ctx)
	if err != nil {
		return model.Conflict{}, err
	}
	var global netip.Prefix
	if configuredOverlay != nil {
		global, _ = netip.ParsePrefix(configuredOverlay.NetworkCIDR)
	} else {
		pools, poolErr := r.ListPools(ctx)
		if poolErr != nil {
			return model.Conflict{}, poolErr
		}
		for _, pool := range pools {
			if pool.Status != model.StatusActive {
				continue
			}
			p, parseErr := netip.ParsePrefix(pool.NetworkCIDR)
			if parseErr == nil {
				global = coveringPrefix(global, p.Masked())
			}
		}
	}
	within := global.IsValid() && global.Contains(prefix.Addr()) && global.Bits() <= prefix.Bits()
	message := fmt.Sprintf("网段 %s 未与当前 IPAM 目录重叠。", prefix)
	if within {
		message = fmt.Sprintf("网段 %s 未与已登记网络重叠，且位于 Overlay 地址池覆盖范围 %s 内。", prefix, global)
	} else if global.IsValid() {
		message = fmt.Sprintf("网段 %s 未与已登记网络重叠；当前 Overlay 池覆盖范围约为 %s，新增网关路由时请确认节点均可达。", prefix, global)
	}
	return model.Conflict{IsValid: true, Message: message, IsWithinOverlay: within}, nil
}

func (r *Repository) SaveSiteNATMapping(ctx *servicecontext.Context, actorID int64, mapping *model.NATMapping, audit *model.AuditEvent) error {
	return r.saveNATMapping(ctx, actorID, mapping, audit)
}

func (r *Repository) saveNATMapping(ctx *servicecontext.Context, actorID int64, mapping *model.NATMapping, audit *model.AuditEvent) error {
	if actorID <= 0 || mapping == nil || mapping.NetworkID <= 0 || audit == nil {
		return errors.New("IPAM NAT mapping and audit event are required")
	}
	if mapping.NATType != "DNAT_VirtualIP" {
		return errors.New("only DNAT_VirtualIP is supported for optional shadow mappings; site SNAT is generated automatically")
	}
	return r.db.WithSaveTransaction(ctx, func(tx *database.DBTransaction) error {
		s := tx.Session
		if err := lockIPAMCatalog(s); err != nil {
			return err
		}
		if err := requirePermission(s, actorID, "ztna.site.write"); err != nil {
			return err
		}
		var network struct {
			ID          int64  `xorm:"'id'"`
			NetworkName string `xorm:"'network_name'"`
			CIDR        string `xorm:"'cidr'"`
			Status      string `xorm:"'status'"`
			NetworkType string `xorm:"'network_type'"`
			IsDeleted   bool   `xorm:"'is_del'"`
		}
		query := "SELECT id, network_name, cidr::text AS cidr, status, network_type, is_del FROM t_ztna_network WHERE id=? FOR UPDATE"
		found, err := s.SQL(query, mapping.NetworkID).Get(&network)
		if err != nil {
			return fmt.Errorf("load business network for NAT mapping: %w", err)
		}
		if !found || network.Status != model.StatusActive || network.IsDeleted {
			return errors.New("NAT mapping requires an active IPAM business network")
		}
		if network.NetworkType != "site" {
			return errors.New("site NAT mapping requires an active network-site network")
		}
		mapping.NetworkName = network.NetworkName
		mapping.RealCIDR = network.CIDR
		if mapping.Enabled && mapping.VirtualCIDR != nil {
			virtual, parseErr := netip.ParsePrefix(*mapping.VirtualCIDR)
			if parseErr != nil || virtual.Bits() == 0 {
				return errors.New("NAT virtual CIDR must be a valid non-default network prefix")
			}
			*mapping.VirtualCIDR = virtual.Masked().String()
			realPrefix, _ := netip.ParsePrefix(network.CIDR)
			if virtual.Addr().Is4() != realPrefix.Addr().Is4() || virtual.Bits() != realPrefix.Bits() {
				return errors.New("prefix DNAT requires matching address families and prefix lengths")
			}
			if err := assertNATShadowAvailable(s, *mapping.VirtualCIDR, mapping.ID); err != nil {
				return err
			}
		}
		if mapping.ID == 0 {
			mapping.CreateNew(ctx.UserName())
			if _, err := tx.Save(mapping); err != nil {
				return fmt.Errorf("create IPAM NAT mapping: %w", err)
			}
		} else {
			if mapping.VersionNo <= 0 {
				return errors.New("NAT mapping version is required for update")
			}
			var current struct {
				VersionNo int64 `xorm:"'version_no'"`
				NetworkID int64 `xorm:"'network_id'"`
			}
			found, err = s.SQL("SELECT version_no, network_id FROM t_ztna_nat_mapping WHERE id=? FOR UPDATE", mapping.ID).Get(&current)
			if err != nil {
				return fmt.Errorf("lock IPAM NAT mapping: %w", err)
			}
			if !found || current.VersionNo != mapping.VersionNo || current.NetworkID != mapping.NetworkID {
				return errors.New("NAT mapping was not found or changed; refresh and try again")
			}
			mapping.Update(ctx.UserName())
			mapping.Cols("network_name", "real_cidr", "virtual_cidr", "nat_type", "enabled", "description")
			if _, err := tx.Save(mapping); err != nil {
				return fmt.Errorf("update IPAM NAT mapping: %w", err)
			}
		}
		audit.TargetType = "ztna_nat_mapping"
		audit.TargetID, audit.TargetName = fmt.Sprintf("%d", mapping.ID), mapping.NetworkName
		if err := writeAudit(ctx, tx, audit); err != nil {
			return fmt.Errorf("write NAT mapping audit: %w", err)
		}
		return nil
	})
}

func assertBusinessNetworkCompatible(s *xorm.Session, cidr string) error {
	candidate, err := netip.ParsePrefix(cidr)
	if err != nil {
		return errors.New("business network CIDR is invalid")
	}
	var overlay struct {
		CIDR string `xorm:"'cidr'"`
	}
	configured, err := s.SQL("SELECT network_cidr::text AS cidr FROM t_ztna_overlay_network WHERE id=1").Get(&overlay)
	if err != nil {
		return fmt.Errorf("load global Overlay network for business network: %w", err)
	}
	if !configured {
		return errors.New("initialize the global Overlay network before creating a business network")
	}
	global, err := netip.ParsePrefix(overlay.CIDR)
	if err != nil {
		return fmt.Errorf("configured global Overlay network %q is invalid: %w", overlay.CIDR, err)
	}
	if candidate.Addr().Is4() != global.Addr().Is4() {
		return fmt.Errorf("business network %s must use the same address family as global Overlay network %s", candidate.Masked(), global.Masked())
	}
	queries := []struct {
		query string
		args  []interface{}
	}{
		{"SELECT pool_name AS name, network_cidr::text AS cidr FROM t_ztna_address_pool WHERE status='active'", nil},
		{"SELECT network_name || ' NAT 影子网段' AS name, virtual_cidr::text AS cidr FROM t_ztna_nat_mapping WHERE enabled=TRUE AND virtual_cidr IS NOT NULL", nil},
		{"SELECT '全局 Overlay 网络' AS name, network_cidr::text AS cidr FROM t_ztna_overlay_network WHERE id=1", nil},
	}
	for _, item := range queries {
		type row struct{ Name, CIDR string }
		var rows []row
		if err := s.SQL(item.query, item.args...).Find(&rows); err != nil {
			return fmt.Errorf("check business network against IPAM ranges: %w", err)
		}
		for _, existing := range rows {
			prefix, parseErr := netip.ParsePrefix(existing.CIDR)
			if parseErr == nil && prefixesOverlap(candidate.Masked(), prefix.Masked()) {
				return fmt.Errorf("business network %s overlaps %s (%s)", candidate.Masked(), existing.Name, prefix.Masked())
			}
		}
	}
	return nil
}

// EnsureBusinessNetworkCompatible validates a network-site prefix while
// holding the same catalog lock used by IPAM network and pool mutations.
func EnsureBusinessNetworkCompatible(s *xorm.Session, cidr string) error {
	if err := lockIPAMCatalog(s); err != nil {
		return err
	}
	return assertBusinessNetworkCompatible(s, cidr)
}

func assertNATShadowAvailable(s *xorm.Session, cidr string, mappingID int64) error {
	candidate, err := netip.ParsePrefix(cidr)
	if err != nil {
		return errors.New("NAT virtual CIDR is invalid")
	}
	queries := []struct {
		query string
		args  []interface{}
	}{
		{"SELECT network_name AS name, cidr::text AS cidr FROM t_ztna_network WHERE status='active' AND is_del=FALSE", nil},
		{"SELECT pool_name AS name, network_cidr::text AS cidr FROM t_ztna_address_pool WHERE status='active'", nil},
		{"SELECT network_name || ' NAT 影子网段' AS name, virtual_cidr::text AS cidr FROM t_ztna_nat_mapping WHERE enabled=TRUE AND virtual_cidr IS NOT NULL AND id<>?", []interface{}{mappingID}},
		{"SELECT '全局 Overlay 网络' AS name, network_cidr::text AS cidr FROM t_ztna_overlay_network WHERE id=1", nil},
	}
	for _, item := range queries {
		type row struct{ Name, CIDR string }
		var rows []row
		if err := s.SQL(item.query, item.args...).Find(&rows); err != nil {
			return fmt.Errorf("check NAT virtual CIDR availability: %w", err)
		}
		for _, existing := range rows {
			prefix, parseErr := netip.ParsePrefix(existing.CIDR)
			if parseErr == nil && prefixesOverlap(candidate.Masked(), prefix.Masked()) {
				return fmt.Errorf("NAT virtual network %s overlaps %s (%s)", candidate.Masked(), existing.Name, prefix.Masked())
			}
		}
	}
	return nil
}

func lockIPAMCatalog(s *xorm.Session) error {
	if _, err := s.Exec("SELECT pg_advisory_xact_lock(?)", ipamCatalogLock); err != nil {
		return fmt.Errorf("lock IPAM catalog for range validation: %w", err)
	}
	return nil
}

func assertPoolCIDRAvailable(s *xorm.Session, cidr string, exceptPoolID int64) error {
	candidate, err := netip.ParsePrefix(cidr)
	if err != nil {
		return errors.New("address pool CIDR is invalid")
	}
	var overlay struct {
		CIDR string `xorm:"'cidr'"`
	}
	configured, err := s.SQL("SELECT network_cidr::text AS cidr FROM t_ztna_overlay_network WHERE id=1").Get(&overlay)
	if err != nil {
		return fmt.Errorf("load global Overlay network for address pool: %w", err)
	}
	if !configured {
		return errors.New("initialize the global Overlay network before creating an address pool")
	}
	global, err := netip.ParsePrefix(overlay.CIDR)
	if err != nil || !global.Contains(candidate.Masked().Addr()) || global.Bits() > candidate.Bits() {
		return fmt.Errorf("address pool %s must be contained within the configured global Overlay network %s", candidate.Masked(), overlay.CIDR)
	}
	queries := []struct {
		query string
		args  []interface{}
	}{
		{"SELECT network_name AS name, cidr::text AS cidr FROM t_ztna_network WHERE status='active' AND is_del=FALSE", nil},
		{"SELECT pool_name AS name, network_cidr::text AS cidr FROM t_ztna_address_pool WHERE status='active' AND id<>?", []interface{}{exceptPoolID}},
		{"SELECT network_name || ' NAT 影子网段' AS name, virtual_cidr::text AS cidr FROM t_ztna_nat_mapping WHERE enabled=TRUE AND virtual_cidr IS NOT NULL", nil},
	}
	for _, item := range queries {
		type row struct{ Name, CIDR string }
		var rows []row
		if len(item.args) == 0 {
			if err := s.SQL(item.query).Find(&rows); err != nil {
				return fmt.Errorf("check overlay pool range overlap: %w", err)
			}
		} else if err := s.SQL(item.query, item.args...).Find(&rows); err != nil {
			return fmt.Errorf("check overlay pool range overlap: %w", err)
		}
		for _, existing := range rows {
			prefix, parseErr := netip.ParsePrefix(existing.CIDR)
			if parseErr == nil && prefixesOverlap(candidate.Masked(), prefix.Masked()) {
				return fmt.Errorf("overlay pool %s overlaps existing IPAM range %s (%s)", candidate.Masked(), existing.Name, prefix.Masked())
			}
		}
	}
	if candidate.Addr().Is4In6() {
		return errors.New("IPv4-mapped IPv6 is not supported in overlay pools")
	}
	if !candidate.Addr().Is4() || candidate.Bits() < 16 || candidate.Bits() > 30 {
		return errors.New("overlay pools must be IPv4 prefixes between /16 and /30")
	}
	return nil
}

func parseAllocatablePool(cidr string) (netip.Prefix, error) {
	prefix, err := netip.ParsePrefix(cidr)
	if err != nil || !prefix.Addr().Is4() || prefix.Bits() < 16 || prefix.Bits() > 30 {
		return netip.Prefix{}, errors.New("selected pool must be an IPv4 prefix between /16 and /30")
	}
	return prefix.Masked(), nil
}

func firstFreeAddress(prefix netip.Prefix, used map[netip.Addr]struct{}) (netip.Addr, error) {
	bytes := prefix.Addr().As4()
	network := uint64(bytes[0])<<24 | uint64(bytes[1])<<16 | uint64(bytes[2])<<8 | uint64(bytes[3])
	hostCount := uint64(1) << uint(32-prefix.Bits())
	first, last := network+1, network+hostCount-2
	for candidate := first; candidate <= last; candidate++ {
		address := netip.AddrFrom4([4]byte{byte(candidate >> 24), byte(candidate >> 16), byte(candidate >> 8), byte(candidate)})
		if _, occupied := used[address]; !occupied {
			return address, nil
		}
	}
	return netip.Addr{}, errors.New("selected IPAM pool has no free host addresses")
}

func prefixesOverlap(a, b netip.Prefix) bool {
	return a.Addr().Is4() == b.Addr().Is4() && (a.Contains(b.Addr()) || b.Contains(a.Addr()))
}

func coveringPrefix(current, next netip.Prefix) netip.Prefix {
	if !current.IsValid() {
		return next
	}
	if current.Addr().Is4() != next.Addr().Is4() {
		return current
	}
	if !current.Addr().Is4() {
		return current
	}
	first, second := current.Addr().As4(), next.Addr().As4()
	left := uint32(first[0])<<24 | uint32(first[1])<<16 | uint32(first[2])<<8 | uint32(first[3])
	right := uint32(second[0])<<24 | uint32(second[1])<<16 | uint32(second[2])<<8 | uint32(second[3])
	common := 32
	if diff := left ^ right; diff != 0 {
		common = bits.LeadingZeros32(diff)
	}
	if current.Bits() < common {
		common = current.Bits()
	}
	if next.Bits() < common {
		common = next.Bits()
	}
	return netip.PrefixFrom(current.Addr(), common).Masked()
}
