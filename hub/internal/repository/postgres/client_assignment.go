package postgres

import (
	"context"
	cryptorand "crypto/rand"
	"errors"
	"fmt"
	"log/slog"
	"math/big"
	"net/netip"
	"strconv"
	"time"

	"velotrust/hub/internal/model"
	"velotrust/shared/repository/database"
)

var errClientAddressUnavailable = errors.New("no eligible Client address pool has an available address")

func (r *GatewayConfigurationSource) reconcileClientAddress(ctx context.Context, nodeCode string) error {
	var identity struct {
		NodeID    int64  `xorm:"'node_id'"`
		NodeName  string `xorm:"'node_name'"`
		NodeCode  string `xorm:"'node_code'"`
		PublicKey string `xorm:"'public_key'"`
		DeviceID  int64  `xorm:"'device_id'"`
		OwnerID   int64  `xorm:"'owner_id'"`
		AuditUser string `xorm:"'username'"`
	}
	s := r.db.Engine().NewSession().Context(ctx)
	found, err := s.SQL(`SELECT node.id AS node_id, node.node_name, node.node_code,
		node.public_key, device.id AS device_id, owner.id AS owner_id, owner.username
		FROM public.t_ztna_node node
		JOIN public.t_ztna_device device ON device.node_id=node.id AND device.status='active'
		JOIN public.t_iam_user owner ON owner.id=device.owner_user_id AND owner.user_status='active'
		WHERE node.node_code=? AND node.node_type='client_agent' AND node.node_status='active'`, nodeCode).Get(&identity)
	closeErr := s.Close()
	if err != nil {
		return fmt.Errorf("load Client identity for address reconciliation: %w", err)
	}
	if closeErr != nil {
		return fmt.Errorf("close Client identity reconciliation query: %w", closeErr)
	}
	if !found {
		return nil
	}
	if identity.NodeID <= 0 || identity.DeviceID <= 0 || identity.OwnerID <= 0 || identity.PublicKey == "" || identity.AuditUser == "" {
		return errors.New("active Client identity is incomplete for address reconciliation")
	}
	return r.db.WithSaveTransaction(ctx, func(tx *database.DBTransaction) error {
		var active struct {
			ID int64 `xorm:"'id'"`
		}
		valid, err := tx.SQL(`SELECT node.id
			FROM public.t_ztna_node node
			JOIN public.t_ztna_device device ON device.node_id=node.id
			JOIN public.t_iam_user owner ON owner.id=device.owner_user_id
			WHERE node.id=? AND device.id=? AND owner.id=? AND node.public_key=?
			AND node.node_type='client_agent' AND node.node_status='active'
			AND device.status='active' AND owner.user_status='active' AND owner.account_type='employee'
			AND EXISTS (SELECT 1 FROM public.t_ztna_device_credential credential
				WHERE credential.device_id=device.id AND credential.credential_type='x25519'
				AND credential.public_key=node.public_key AND credential.status='active'
				AND credential.not_before<=CURRENT_TIMESTAMP
				AND (credential.expires_at IS NULL OR credential.expires_at>CURRENT_TIMESTAMP))
			FOR UPDATE OF node, device, owner`, identity.NodeID, identity.DeviceID, identity.OwnerID, identity.PublicKey).Get(&active)
		if err != nil {
			return fmt.Errorf("revalidate Client identity for address reconciliation: %w", err)
		}
		if !valid {
			return nil
		}
		return ensureClientAddressLease(tx, identity.OwnerID, identity.NodeID, identity.DeviceID,
			identity.NodeCode, identity.NodeName, identity.PublicKey, identity.AuditUser)
	})
}

type clientAssignmentCandidate struct {
	GroupID   int64  `xorm:"'group_id'"`
	GroupCode string `xorm:"'group_code'"`
	PoolID    int64  `xorm:"'pool_id'"`
	PoolCIDR  string `xorm:"'pool_cidr'"`
}

// ensureClientAddressLease assigns only the Overlay IP. Gateway selection and
// Peer creation belong to the access-policy configuration path.
func ensureClientAddressLease(tx *database.DBTransaction, userID, nodeID, deviceID int64, nodeCode, nodeName, publicKey, auditUser string) error {
	if tx == nil || userID <= 0 || nodeID <= 0 || deviceID <= 0 || nodeCode == "" || publicKey == "" || auditUser == "" {
		return errors.New("Client address assignment identity is incomplete")
	}

	var candidates []clientAssignmentCandidate
	if err := tx.SQL(`SELECT DISTINCT group_row.id AS group_id, group_row.group_code,
		pool.id AS pool_id, pool.network_cidr::text AS pool_cidr
		FROM public.t_iam_user_group user_group
		JOIN public.t_iam_group group_row ON group_row.id=user_group.group_id AND group_row.status='active'
		JOIN public.t_ztna_address_pool pool ON pool.client_group_id=group_row.id
			AND pool.status='active' AND pool.pool_type='User-Device' AND pool.allocation_mode='dynamic'
		WHERE user_group.user_id=?
		ORDER BY pool.id, group_row.id`, userID).Find(&candidates); err != nil {
		return fmt.Errorf("find Client address pools for employee groups: %w", err)
	}
	if len(candidates) == 0 {
		if err := logClientAssignmentBlockers(tx, userID, nodeCode); err != nil {
			return err
		}
	}

	var lease model.AddressLease
	leaseFound, err := tx.SQL(`SELECT id, pool_id, host(address) AS address, node_type, target_name, endpoint,
		allocation_mode, idp_group, wireguard_pubkey, status, allocated_at, released_at,
		created_at, created_by, updated_at, updated_by, version_no
		FROM public.t_ztna_address_lease WHERE wireguard_pubkey=? AND node_type='Peer'
		AND status IN ('active','reserved') FOR UPDATE`, publicKey).Get(&lease)
	if err != nil {
		return fmt.Errorf("lock existing Client Overlay lease: %w", err)
	}
	if leaseFound && lease.Status == "active" && lease.TargetName == nodeCode && clientPoolContains(candidates, lease.PoolID) {
		for _, candidate := range candidates {
			if candidate.PoolID != lease.PoolID {
				continue
			}
			prefix, err := lockEligibleClientAddressPool(tx, userID, candidate)
			if errors.Is(err, errClientAddressUnavailable) {
				break
			}
			if err != nil {
				return err
			}
			address, parseErr := netip.ParseAddr(lease.Address)
			if parseErr == nil && isUsableClientPoolAddress(prefix, address) {
				slog.Debug("reused existing automatic Client Overlay address", "node_id", nodeCode,
					"tunnel_ip", lease.Address, "pool_id", lease.PoolID)
				return nil
			}
			break
		}
	}
	if leaseFound {
		if err := releaseClientAddressLease(tx, &lease, auditUser); err != nil {
			return err
		}
	}

	for _, candidate := range candidates {
		address, err := allocateClientAddress(tx, userID, candidate)
		if errors.Is(err, errClientAddressUnavailable) {
			continue
		}
		if err != nil {
			return err
		}
		groupCode := candidate.GroupCode
		lease := model.AddressLease{
			PoolID: candidate.PoolID, Address: address.String(), NodeType: "Peer",
			TargetName: nodeCode, AllocationMode: "dynamic", IDPGroup: &groupCode,
			WireGuardPublicKey: &publicKey, Status: "active", AllocatedAt: time.Now().UTC(),
		}
		lease.CreateNew(auditUser)
		if _, err := tx.Save(&lease); err != nil {
			return fmt.Errorf("save automatic Client Overlay lease: %w", err)
		}
		if err := queueGatewayPeerRefreshesForClient(tx, userID, nodeCode); err != nil {
			return err
		}
		slog.Info("automatically assigned Client Overlay address", "node_id", nodeCode,
			"tunnel_ip", netip.PrefixFrom(address, 32).String(), "pool_id", candidate.PoolID,
			"group_code", candidate.GroupCode)
		return nil
	}
	if len(candidates) > 0 {
		slog.Warn("Client automatic address assignment has no free dynamic address", "node_id", nodeCode, "candidate_count", len(candidates))
	}
	return nil
}

func queueGatewayPeerRefreshesForClient(tx *database.DBTransaction, userID int64, nodeCode string) error {
	if tx == nil || userID <= 0 || nodeCode == "" {
		return errors.New("Client identity is required to refresh Gateway peers")
	}
	var gateways []struct {
		ID int64 `xorm:"'id'"`
	}
	if err := tx.Session.SQL(`SELECT DISTINCT gateway.id AS id
		FROM public.t_iam_user_group AS membership
		JOIN public.t_iam_group AS employee_group
		  ON employee_group.id=membership.group_id AND employee_group.status='active'
		JOIN (
			SELECT binding.group_id, binding.gateway_node_id
			FROM public.t_ztna_gateway_group_binding AS binding
			UNION
			SELECT subject.subject_id AS group_id, network.gateway_node_id AS gateway_node_id
			FROM public.t_ztna_access_policy AS policy
			JOIN public.t_ztna_policy_subject AS subject ON subject.policy_id=policy.id AND subject.subject_type='group'
			JOIN public.t_ztna_policy_resource AS policy_resource ON policy_resource.policy_id=policy.id
			JOIN public.t_ztna_protected_resource AS resource ON resource.id=policy_resource.protected_resource_id AND resource.status='active'
			JOIN public.t_ztna_network AS network ON network.id=resource.network_id
			  AND network.network_type='site' AND network.status='active' AND network.is_del=FALSE
			WHERE policy.status='active' AND network.gateway_node_id IS NOT NULL
		) AS admission ON admission.group_id=employee_group.id
		JOIN public.t_ztna_node AS gateway
		  ON gateway.id=admission.gateway_node_id AND gateway.node_type='gateway' AND gateway.node_status='active'
		WHERE membership.user_id=?
		ORDER BY gateway.id`, userID).Find(&gateways); err != nil {
		return fmt.Errorf("find Gateways for newly assigned Client peer: %w", err)
	}
	for _, gateway := range gateways {
		event := model.AccessChangeEvent{
			SubjectType: "node", SubjectID: gateway.ID,
			ChangeType: "authorization_changed", Status: "pending",
		}
		event.CreateNew(nodeAuditActor("client_agent", nodeCode))
		if _, err := tx.Save(&event); err != nil {
			return fmt.Errorf("queue Gateway %d refresh after Client address allocation: %w", gateway.ID, err)
		}
		if _, err := tx.Session.Exec("SELECT pg_notify('velotrust_access_change', ?)", strconv.FormatInt(event.ID, 10)); err != nil {
			return fmt.Errorf("notify Gateway %d after Client address allocation: %w", gateway.ID, err)
		}
	}
	return nil
}

func releaseClientAddressLease(tx *database.DBTransaction, lease *model.AddressLease, auditUser string) error {
	if tx == nil || lease == nil || lease.ID <= 0 {
		return errors.New("Client address lease is required for release")
	}
	now := time.Now().UTC()
	lease.Status, lease.ReleasedAt = "released", &now
	lease.Update(auditUser)
	lease.Cols("status", "released_at")
	lease.AddSaveCondition("status IN ('active','reserved')")
	if _, err := tx.Save(lease); err != nil {
		return fmt.Errorf("release obsolete Client Overlay lease: %w", err)
	}
	return nil
}

func logClientAssignmentBlockers(tx *database.DBTransaction, userID int64, nodeCode string) error {
	var counts struct {
		ActiveGroups    int64 `xorm:"'active_groups'"`
		ConfiguredPools int64 `xorm:"'configured_pools'"`
		EligiblePools   int64 `xorm:"'eligible_pools'"`
	}
	found, err := tx.SQL(`WITH employee_groups AS (
			SELECT group_row.id
			FROM public.t_iam_user_group membership
			JOIN public.t_iam_group group_row ON group_row.id=membership.group_id AND group_row.status='active'
			WHERE membership.user_id=?
		), configured_pools AS (
			SELECT pool.id, pool.status, pool.pool_type, pool.allocation_mode
			FROM employee_groups
			JOIN public.t_ztna_address_pool pool ON pool.client_group_id=employee_groups.id
		)
		SELECT (SELECT count(*) FROM employee_groups) AS active_groups,
		       (SELECT count(*) FROM configured_pools) AS configured_pools,
		       (SELECT count(*) FROM configured_pools WHERE status='active'
		          AND pool_type='User-Device' AND allocation_mode='dynamic') AS eligible_pools`, userID).Get(&counts)
	if err != nil {
		return fmt.Errorf("diagnose missing Client address pool: %w", err)
	}
	if !found {
		return errors.New("Client address assignment diagnostics returned no result")
	}
	slog.Info("Client has no eligible automatic address pool; no Overlay lease is assigned",
		"node_id", nodeCode, "employee_user_id", userID,
		"active_employee_groups", counts.ActiveGroups,
		"groups_with_address_pool", counts.ConfiguredPools,
		"eligible_dynamic_user_device_pools", counts.EligiblePools)
	return nil
}

func clientPoolContains(candidates []clientAssignmentCandidate, poolID int64) bool {
	for _, candidate := range candidates {
		if candidate.PoolID == poolID {
			return true
		}
	}
	return false
}

func allocateClientAddress(tx *database.DBTransaction, userID int64, candidate clientAssignmentCandidate) (netip.Addr, error) {
	prefix, err := lockEligibleClientAddressPool(tx, userID, candidate)
	if err != nil {
		return netip.Addr{}, err
	}
	var occupiedRows []struct {
		Address string `xorm:"'address'"`
	}
	if err := tx.SQL(`SELECT host(address) AS address FROM public.t_ztna_address_lease
		WHERE pool_id=? AND status IN ('active','reserved')`, candidate.PoolID).Find(&occupiedRows); err != nil {
		return netip.Addr{}, fmt.Errorf("load occupied automatic Client addresses: %w", err)
	}
	occupied := make(map[netip.Addr]struct{}, len(occupiedRows))
	for _, row := range occupiedRows {
		if address, parseErr := netip.ParseAddr(row.Address); parseErr == nil {
			occupied[address] = struct{}{}
		}
	}
	return randomAvailableClientAddress(prefix, occupied)
}

func lockEligibleClientAddressPool(tx *database.DBTransaction, userID int64, candidate clientAssignmentCandidate) (netip.Prefix, error) {
	var membership struct {
		ID int64 `xorm:"'id'"`
	}
	found, err := tx.SQL(`SELECT group_row.id
		FROM public.t_iam_user_group user_group
		JOIN public.t_iam_group group_row ON group_row.id=user_group.group_id
		WHERE user_group.user_id=? AND group_row.id=? AND group_row.status='active'
		FOR SHARE OF user_group, group_row`, userID, candidate.GroupID).Get(&membership)
	if err != nil {
		return netip.Prefix{}, fmt.Errorf("revalidate Client employee-group membership: %w", err)
	}
	if !found {
		return netip.Prefix{}, errClientAddressUnavailable
	}
	var pool struct {
		CIDR    string `xorm:"'network_cidr'"`
		Status  string `xorm:"'status'"`
		Type    string `xorm:"'pool_type'"`
		Mode    string `xorm:"'allocation_mode'"`
		GroupID int64  `xorm:"'client_group_id'"`
	}
	found, err = tx.SQL(`SELECT network_cidr::text AS network_cidr, status, pool_type, allocation_mode,
		COALESCE(client_group_id, 0) AS client_group_id
		FROM public.t_ztna_address_pool WHERE id=? FOR UPDATE`, candidate.PoolID).Get(&pool)
	if err != nil {
		return netip.Prefix{}, fmt.Errorf("lock automatic Client address pool: %w", err)
	}
	if !found || pool.Status != "active" || pool.Type != "User-Device" || pool.Mode != "dynamic" || pool.GroupID != candidate.GroupID {
		return netip.Prefix{}, errClientAddressUnavailable
	}
	prefix, err := netip.ParsePrefix(pool.CIDR)
	if err != nil || !prefix.Addr().Is4() || prefix.Bits() < 16 || prefix.Bits() > 30 || pool.CIDR != candidate.PoolCIDR {
		return netip.Prefix{}, fmt.Errorf("invalid automatic Client address pool CIDR %q", pool.CIDR)
	}
	return prefix, nil
}

func isUsableClientPoolAddress(prefix netip.Prefix, address netip.Addr) bool {
	if !prefix.Contains(address) || !address.Is4() {
		return false
	}
	last := lastIPv4Address(prefix.Masked())
	return address != prefix.Masked().Addr() && address != last
}

func lastIPv4Address(prefix netip.Prefix) netip.Addr {
	base := prefix.Masked().Addr().As4()
	baseValue := uint64(base[0])<<24 | uint64(base[1])<<16 | uint64(base[2])<<8 | uint64(base[3])
	last := baseValue + (uint64(1) << uint(32-prefix.Bits())) - 1
	return netip.AddrFrom4([4]byte{byte(last >> 24), byte(last >> 16), byte(last >> 8), byte(last)})
}

func randomAvailableClientAddress(prefix netip.Prefix, occupied map[netip.Addr]struct{}) (netip.Addr, error) {
	if !prefix.IsValid() || !prefix.Addr().Is4() || prefix.Bits() < 16 || prefix.Bits() > 30 {
		return netip.Addr{}, errors.New("Client dynamic address pool must be IPv4 /16 through /30")
	}
	base := prefix.Masked().Addr().As4()
	baseValue := uint64(base[0])<<24 | uint64(base[1])<<16 | uint64(base[2])<<8 | uint64(base[3])
	hostCount := uint64(1) << uint(32-prefix.Bits())
	usableCount := hostCount - 2
	free := make([]netip.Addr, 0, int(usableCount))
	for hostOffset := uint64(0); hostOffset < usableCount; hostOffset++ {
		value := baseValue + 1 + hostOffset
		address := netip.AddrFrom4([4]byte{byte(value >> 24), byte(value >> 16), byte(value >> 8), byte(value)})
		if _, exists := occupied[address]; !exists {
			free = append(free, address)
		}
	}
	if len(free) == 0 {
		return netip.Addr{}, errClientAddressUnavailable
	}
	index, err := cryptorand.Int(cryptorand.Reader, big.NewInt(int64(len(free))))
	if err != nil {
		return netip.Addr{}, fmt.Errorf("choose random Client Overlay address: %w", err)
	}
	return free[index.Int64()], nil
}

func firstAvailableClientAddress(prefix netip.Prefix, occupied map[netip.Addr]struct{}) (netip.Addr, error) {
	if !prefix.IsValid() || !prefix.Addr().Is4() || prefix.Bits() < 16 || prefix.Bits() > 30 {
		return netip.Addr{}, errors.New("Client dynamic address pool must be IPv4 /16 through /30")
	}
	base := prefix.Masked().Addr().As4()
	baseValue := uint64(base[0])<<24 | uint64(base[1])<<16 | uint64(base[2])<<8 | uint64(base[3])
	hostCount := uint64(1) << uint(32-prefix.Bits())
	for value := baseValue + 1; value < baseValue+hostCount-1; value++ {
		address := netip.AddrFrom4([4]byte{byte(value >> 24), byte(value >> 16), byte(value >> 8), byte(value)})
		if _, exists := occupied[address]; !exists {
			return address, nil
		}
	}
	return netip.Addr{}, errClientAddressUnavailable
}
