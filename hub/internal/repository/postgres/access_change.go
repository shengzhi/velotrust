package postgres

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"velotrust/shared/repository/database"
)

// AccessChangeRepository fans durable Management outbox events to active
// Gateways and records their current-configuration acknowledgements.
type AccessChangeRepository struct {
	db       *database.Database
	hubActor string
}

func NewAccessChangeRepository(db *database.Database, instanceID string) (*AccessChangeRepository, error) {
	if db == nil || db.Engine() == nil || db.Driver() != "postgres" || strings.TrimSpace(instanceID) == "" {
		return nil, errors.New("access-change repository requires PostgreSQL and a HUB instance ID")
	}
	return &AccessChangeRepository{db: db, hubActor: auditActor(instanceID)}, nil
}

func (r *AccessChangeRepository) DispatchPending(ctx context.Context, notify func(string, string) bool) error {
	if ctx == nil || notify == nil {
		return errors.New("access-change context and Gateway notifier are required")
	}
	var events []struct {
		ID int64 `xorm:"'id'"`
	}
	if err := r.db.Engine().Context(ctx).SQL(`
		SELECT id FROM public.t_ztna_access_change_event
		WHERE status='pending' ORDER BY id LIMIT 500
	`).Find(&events); err != nil {
		return fmt.Errorf("load pending access-change events: %w", err)
	}
	for _, event := range events {
		if err := r.initializeTargets(ctx, event.ID); err != nil {
			return err
		}
	}

	s := r.db.Engine().NewSession().Context(ctx)
	defer s.Close()
	if _, err := s.Exec(`
		UPDATE public.t_ztna_access_change_delivery AS delivery
		SET status='acknowledged', acknowledged_at=COALESCE(delivery.acknowledged_at, CURRENT_TIMESTAMP),
		    updated_at=CURRENT_TIMESTAMP, updated_by=?, version_no=delivery.version_no+1
		FROM public.t_ztna_node AS gateway
		WHERE delivery.gateway_node_id=gateway.id AND gateway.node_status <> 'active'
		  AND delivery.status='pending'
	`, r.hubActor); err != nil {
		return fmt.Errorf("close access-change deliveries for inactive Gateways: %w", err)
	}
	var deliveries []struct {
		EventID    int64      `xorm:"'event_id'"`
		NodeID     int64      `xorm:"'gateway_node_id'"`
		NodeCode   string     `xorm:"'node_code'"`
		ChangeType string     `xorm:"'change_type'"`
		Notified   *time.Time `xorm:"'last_notified_at'"`
	}
	if err := s.SQL(`
		SELECT delivery.event_id, delivery.gateway_node_id, gateway.node_code, event.change_type, delivery.last_notified_at
		FROM public.t_ztna_access_change_delivery AS delivery
		JOIN public.t_ztna_access_change_event AS event ON event.id=delivery.event_id
		JOIN public.t_ztna_node AS gateway ON gateway.id=delivery.gateway_node_id
		WHERE delivery.status='pending' AND gateway.node_status='active'
		  AND (delivery.last_notified_at IS NULL OR delivery.last_notified_at < CURRENT_TIMESTAMP - INTERVAL '5 seconds')
		ORDER BY delivery.event_id, delivery.gateway_node_id
		LIMIT 1000
	`).Find(&deliveries); err != nil {
		return fmt.Errorf("load pending Gateway access-change deliveries: %w", err)
	}
	for _, delivery := range deliveries {
		reason := "ACCESS_CONFIGURATION_CHANGED"
		if delivery.ChangeType == "disabled" {
			reason = "ACCESS_REVOKED"
		}
		if !notify(delivery.NodeCode, reason) {
			continue
		}
		if _, err := s.Exec(`
			UPDATE public.t_ztna_access_change_delivery
			SET last_notified_at=CURRENT_TIMESTAMP, updated_at=CURRENT_TIMESTAMP,
			    updated_by=?, version_no=version_no+1
			WHERE event_id=? AND gateway_node_id=? AND status='pending'
		`, r.hubActor, delivery.EventID, delivery.NodeID); err != nil {
			return fmt.Errorf("record Gateway access-change notification: %w", err)
		}
	}
	var clientTargets []struct {
		NodeCode   string `xorm:"'node_code'"`
		ChangeType string `xorm:"'change_type'"`
	}
	if err := s.SQL(`
		SELECT DISTINCT node.node_code, event.change_type
		FROM public.t_ztna_access_change_event event
		JOIN public.t_ztna_node node ON node.node_type='client_agent'
		LEFT JOIN public.t_ztna_device device ON device.node_id=node.id
		WHERE event.status='pending' AND (
			(event.subject_type='node' AND node.id=event.subject_id)
			OR (event.subject_type='device' AND device.id=event.subject_id)
			OR (event.subject_type='user' AND device.owner_user_id=event.subject_id)
		)
		ORDER BY node.node_code
		LIMIT 1000
	`).Find(&clientTargets); err != nil {
		return fmt.Errorf("load Client access-change targets: %w", err)
	}
	for _, target := range clientTargets {
		reason := "ACCESS_CONFIGURATION_CHANGED"
		if target.ChangeType == "disabled" {
			reason = "ACCESS_REVOKED"
		}
		// Client Agents need an immediate local tunnel teardown on disable. The
		// active node/device/user state remains authoritative if this wakeup is
		// missed or the Client reconnects to another HUB instance.
		notify(target.NodeCode, reason)
	}
	if _, err := s.Exec(`
		UPDATE public.t_ztna_access_change_event AS event
		SET status='completed', updated_at=CURRENT_TIMESTAMP, updated_by=?, version_no=event.version_no+1
		WHERE event.status='pending' AND event.targets_initialized=TRUE
		  AND NOT EXISTS (
			SELECT 1 FROM public.t_ztna_access_change_delivery AS delivery
			WHERE delivery.event_id=event.id AND delivery.status='pending'
		  )
	`, r.hubActor); err != nil {
		return fmt.Errorf("complete acknowledged access-change events: %w", err)
	}
	return nil
}

func (r *AccessChangeRepository) initializeTargets(ctx context.Context, eventID int64) error {
	s := r.db.Engine().NewSession().Context(ctx)
	defer s.Close()
	if err := s.Begin(); err != nil {
		return fmt.Errorf("begin access-change target initialization: %w", err)
	}
	defer s.Rollback()
	var row struct {
		Initialized bool `xorm:"'targets_initialized'"`
	}
	found, err := s.SQL(`SELECT targets_initialized FROM public.t_ztna_access_change_event WHERE id=? AND status='pending' FOR UPDATE`, eventID).Get(&row)
	if err != nil {
		return fmt.Errorf("lock access-change event: %w", err)
	}
	if !found || row.Initialized {
		return s.Commit()
	}
	if _, err := s.Exec(`
		INSERT INTO public.t_ztna_access_change_delivery (event_id, gateway_node_id, status, created_by, updated_by)
		SELECT event.id, gateway.id, 'pending', ?, ?
		FROM public.t_ztna_access_change_event AS event
		JOIN public.t_ztna_tunnel_peer AS affected_peer ON TRUE
		JOIN public.t_ztna_tunnel AS tunnel
		  ON tunnel.id=affected_peer.tunnel_id AND tunnel.status='active'
		JOIN public.t_ztna_tunnel_peer AS gateway_peer
		  ON gateway_peer.tunnel_id=tunnel.id AND gateway_peer.status='active'
		JOIN public.t_ztna_node AS gateway
		  ON gateway.id=gateway_peer.node_id AND gateway.node_type='gateway' AND gateway.node_status='active'
		WHERE event.id=? AND affected_peer.status='active'
		  AND (
			(event.subject_type='node' AND affected_peer.node_id=event.subject_id)
			OR (event.subject_type='device' AND EXISTS (
				SELECT 1 FROM public.t_ztna_device AS device
				WHERE device.id=event.subject_id AND (
					affected_peer.device_id=device.id
					OR affected_peer.node_id=device.node_id
					OR EXISTS (
						SELECT 1 FROM public.t_ztna_device_credential AS credential
						WHERE credential.device_id=device.id AND credential.public_key=affected_peer.public_key
					)
				)
			))
			OR (event.subject_type='user' AND EXISTS (
				SELECT 1 FROM public.t_ztna_device AS device
				WHERE device.owner_user_id=event.subject_id AND (
					affected_peer.device_id=device.id
					OR affected_peer.node_id=device.node_id
					OR EXISTS (
						SELECT 1 FROM public.t_ztna_device_credential AS credential
						WHERE credential.device_id=device.id AND credential.public_key=affected_peer.public_key
					)
				)
			))
		  )
		ON CONFLICT (event_id, gateway_node_id) DO NOTHING
	`, r.hubActor, r.hubActor, eventID); err != nil {
		return fmt.Errorf("create Gateway access-change deliveries: %w", err)
	}
	if _, err := s.Exec(`
		UPDATE public.t_ztna_access_change_event
		SET targets_initialized=TRUE, updated_at=CURRENT_TIMESTAMP, updated_by=?, version_no=version_no+1
		WHERE id=?
	`, r.hubActor, eventID); err != nil {
		return fmt.Errorf("mark access-change targets initialized: %w", err)
	}
	return s.Commit()
}

// AcknowledgeCurrentConfig is called only after HUB confirms that the
// Gateway's acknowledged hash still matches the latest authoritative snapshot.
func (r *AccessChangeRepository) AcknowledgeCurrentConfig(ctx context.Context, nodeCode string) error {
	if ctx == nil || nodeCode == "" {
		return errors.New("Gateway identity and request context are required")
	}
	actor, err := r.resolveNodeActor(ctx, nodeCode)
	if err != nil {
		return err
	}
	s := r.db.Engine().NewSession().Context(ctx)
	defer s.Close()
	if _, err := s.Exec(`
		UPDATE public.t_ztna_access_change_delivery AS delivery
		SET status='acknowledged', acknowledged_at=CURRENT_TIMESTAMP,
		    updated_at=CURRENT_TIMESTAMP, updated_by=?, version_no=delivery.version_no+1
		FROM public.t_ztna_node AS gateway
		WHERE delivery.gateway_node_id=gateway.id AND gateway.node_code=?
		  AND delivery.status='pending'
	`, actor, nodeCode); err != nil {
		return fmt.Errorf("acknowledge Gateway access-change deliveries: %w", err)
	}
	if _, err := s.Exec(`
		UPDATE public.t_ztna_access_change_event AS event
		SET status='completed', updated_at=CURRENT_TIMESTAMP, updated_by=?, version_no=event.version_no+1
		WHERE event.status='pending' AND event.targets_initialized=TRUE
		  AND NOT EXISTS (
			SELECT 1 FROM public.t_ztna_access_change_delivery AS delivery
			WHERE delivery.event_id=event.id AND delivery.status='pending'
		  )
	`, r.hubActor); err != nil {
		return fmt.Errorf("complete Gateway access-change events: %w", err)
	}
	return nil
}

func (r *AccessChangeRepository) RecordFailure(ctx context.Context, nodeCode, message string) error {
	if ctx == nil || nodeCode == "" {
		return errors.New("Gateway identity and request context are required")
	}
	actor, err := r.resolveNodeActor(ctx, nodeCode)
	if err != nil {
		return err
	}
	if message == "" {
		message = "Gateway did not apply the current configuration"
	}
	s := r.db.Engine().NewSession().Context(ctx)
	defer s.Close()
	if _, err := s.Exec(`
		UPDATE public.t_ztna_access_change_delivery AS delivery
		SET last_error=LEFT(?, 512), updated_at=CURRENT_TIMESTAMP,
		    updated_by=?, version_no=delivery.version_no+1
		FROM public.t_ztna_node AS gateway
		WHERE delivery.gateway_node_id=gateway.id AND gateway.node_code=?
		  AND delivery.status='pending'
	`, message, actor, nodeCode); err != nil {
		return fmt.Errorf("record Gateway access-change failure: %w", err)
	}
	return nil
}

func (r *AccessChangeRepository) resolveNodeActor(ctx context.Context, nodeCode string) (string, error) {
	var nodeType string
	found, err := r.db.Engine().Context(ctx).SQL(`
		SELECT node_type FROM public.t_ztna_node WHERE node_code=?
	`, nodeCode).Get(&nodeType)
	if err != nil {
		return "", fmt.Errorf("resolve access-change node audit principal: %w", err)
	}
	if !found {
		return "", errors.New("access-change node is not active")
	}
	actor := nodeAuditActor(nodeType, nodeCode)
	if actor == "" {
		return "", errors.New("unsupported access-change node audit principal")
	}
	return actor, nil
}
