package postgres

import (
	"fmt"

	"velotrust/hub/internal/configsync"
	"xorm.io/xorm"
)

// compileClientSnapshot intentionally does not derive Gateway access from
// tunnel membership. A valid IP lease identifies the Client's Overlay
// address, but only an access-policy decision may authorize Gateway peers.
// The policy-to-WireGuard compiler is a separate implementation stage.
func compileClientSnapshot(s *xorm.Session, nodeCode string) ([]byte, int64, error) {
	var target struct {
		NodeID int64 `xorm:"'node_id'"`
		Active bool  `xorm:"'active'"`
	}
	found, err := s.SQL(`SELECT node.id AS node_id,
		(node.node_status='active' AND device.status='active' AND owner.user_status='active'
		 AND owner.account_type='employee') AS active
		FROM public.t_ztna_node node
		LEFT JOIN public.t_ztna_device device ON device.node_id=node.id
		LEFT JOIN public.t_iam_user owner ON owner.id=device.owner_user_id
		WHERE node.node_code=? AND node.node_type='client_agent'`, nodeCode).Get(&target)
	if err != nil {
		return nil, 0, fmt.Errorf("read Client configuration target: %w", err)
	}
	if !found || target.NodeID <= 0 || !target.Active {
		return nil, 0, fmt.Errorf("%w: active Client was not found", configsync.ErrTargetUnavailable)
	}

	var hasLease bool
	if _, err := s.SQL(`SELECT EXISTS (
		SELECT 1 FROM public.t_ztna_node node
		JOIN public.t_ztna_address_lease lease ON lease.wireguard_pubkey=node.public_key
		WHERE node.id=? AND lease.node_type='Peer' AND lease.status='active'
	)`, target.NodeID).Get(&hasLease); err != nil {
		return nil, 0, fmt.Errorf("check Client Overlay address lease: %w", err)
	}
	if !hasLease {
		return nil, target.NodeID, configsync.ErrClientAddressNotAssigned
	}
	return nil, target.NodeID, configsync.ErrClientConfigurationNotReady
}
