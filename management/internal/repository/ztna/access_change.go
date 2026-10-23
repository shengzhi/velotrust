package ztna

import (
	"errors"
	"fmt"
	"strconv"

	servicecontext "velotrust/management/internal/context"
	model "velotrust/management/internal/model/ztna"
	"velotrust/shared/repository/database"
	"xorm.io/xorm"
)

// CreateAccessChange appends an authorization change to the durable HUB
// outbox. PostgreSQL NOTIFY wakes connected HUB instances; polling the outbox
// remains the recovery path if notifications are lost.
func CreateAccessChange(ctx *servicecontext.Context, tx *database.DBTransaction, subjectType string, subjectID int64, changeType string) (int64, error) {
	if ctx == nil || tx == nil || subjectID <= 0 {
		return 0, errors.New("service context, transaction, and access-change subject are required")
	}
	if subjectType != "user" && subjectType != "device" && subjectType != "node" && subjectType != "policy" {
		return 0, errors.New("unsupported access-change subject type")
	}
	if changeType != "disabled" && changeType != "enabled" && changeType != "authorization_changed" {
		return 0, errors.New("unsupported access-change type")
	}
	event := &model.AccessChangeEvent{
		SubjectType: subjectType,
		SubjectID:   subjectID,
		ChangeType:  changeType,
		Status:      "pending",
	}
	event.CreateNew(ctx.UserName())
	if _, err := tx.Save(event); err != nil {
		return 0, err
	}
	if _, err := tx.Session.Exec("SELECT pg_notify('velotrust_access_change', ?)", strconv.FormatInt(event.ID, 10)); err != nil {
		return 0, err
	}
	return event.ID, nil
}

// CreateGatewayGroupClientRefreshes queues Client configuration refreshes for
// employees whose active identity may be admitted by the Gateway's groups.
func CreateGatewayGroupClientRefreshes(ctx *servicecontext.Context, tx *database.DBTransaction, gatewayID int64) error {
	if ctx == nil || tx == nil || gatewayID <= 0 {
		return errors.New("service context, transaction, and Gateway are required")
	}
	var users []struct {
		ID int64 `xorm:"'id'"`
	}
	if err := tx.Session.SQL(`SELECT DISTINCT employee.id AS id
		FROM public.t_ztna_gateway_group_binding AS binding
		JOIN public.t_iam_user_group AS membership ON membership.group_id=binding.group_id
		JOIN public.t_iam_user AS employee ON employee.id=membership.user_id
		WHERE binding.gateway_node_id=? AND employee.account_type='employee' AND employee.user_status='active'
		ORDER BY employee.id`, gatewayID).Find(&users); err != nil {
		return fmt.Errorf("find employees affected by Gateway configuration: %w", err)
	}
	for _, user := range users {
		if _, err := CreateAccessChange(ctx, tx, "user", user.ID, "authorization_changed"); err != nil {
			return fmt.Errorf("queue Client configuration refresh for employee %d: %w", user.ID, err)
		}
	}
	return nil
}

// CreateGroupGatewayRefreshes queues current Gateways bound to a security group.
// Call this while the membership or group status transaction still owns the
// change so the peer sets cannot remain stale after revocation.
func CreateGroupGatewayRefreshes(ctx *servicecontext.Context, tx *database.DBTransaction, groupID int64) error {
	if ctx == nil || tx == nil || groupID <= 0 {
		return errors.New("service context, transaction, and security group are required")
	}
	var gateways []struct {
		ID int64 `xorm:"'id'"`
	}
	if err := tx.Session.SQL(`SELECT DISTINCT gateway.id AS id
		FROM (
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
		) AS admission
		JOIN public.t_ztna_node AS gateway ON gateway.id=admission.gateway_node_id
		WHERE admission.group_id=? AND gateway.node_type='gateway'
		ORDER BY gateway.id`, groupID).Find(&gateways); err != nil {
		return fmt.Errorf("find Gateways affected by security-group change: %w", err)
	}
	for _, gateway := range gateways {
		if _, err := CreateAccessChange(ctx, tx, "node", gateway.ID, "authorization_changed"); err != nil {
			return fmt.Errorf("queue Gateway configuration refresh for Gateway %d: %w", gateway.ID, err)
		}
	}
	return nil
}

// CreateSitePolicyRefreshes queues policy propagation after a site's Gateway,
// status, or NAT mapping changes. Those values determine the Client routes and
// the Gateway destination prefix generated for each protected resource.
func CreateSitePolicyRefreshes(ctx *servicecontext.Context, tx *database.DBTransaction, siteID int64) error {
	if ctx == nil || tx == nil || siteID <= 0 {
		return errors.New("service context, transaction, and network site are required")
	}
	var policies []struct {
		ID int64 `xorm:"'id'"`
	}
	if err := tx.Session.SQL(`SELECT DISTINCT policy.id
		FROM public.t_ztna_access_policy AS policy
		JOIN public.t_ztna_policy_resource AS relation ON relation.policy_id=policy.id
		JOIN public.t_ztna_protected_resource AS resource ON resource.id=relation.protected_resource_id
		WHERE policy.status='active' AND resource.network_id=? ORDER BY policy.id`, siteID).Find(&policies); err != nil {
		return fmt.Errorf("find active policies affected by network-site change: %w", err)
	}
	for _, policy := range policies {
		if _, err := CreateAccessChange(ctx, tx, "policy", policy.ID, "authorization_changed"); err != nil {
			return fmt.Errorf("queue Gateway and Client refresh for policy %d after network-site change: %w", policy.ID, err)
		}
	}
	return nil
}

// AccessChangeStatus returns the newest durable propagation state for a
// subject. HUB fills delivery rows and acknowledges them after the Gateway
// confirms it has applied the latest snapshot.
func AccessChangeStatus(ctx *servicecontext.Context, s *xorm.Session, subjectType string, subjectID int64) (*model.AccessChangeStatusView, error) {
	if ctx == nil || s == nil || subjectID <= 0 {
		return nil, errors.New("request context, transaction session, and subject are required")
	}
	var result model.AccessChangeStatusView
	found, err := s.SQL(`
		SELECT event.id AS event_id, event.subject_type, event.subject_id,
		       event.status, COUNT(delivery.id) AS target_count,
	       COUNT(delivery.id) FILTER (WHERE delivery.status='acknowledged') AS acknowledged_count,
	       MAX(delivery.last_error) FILTER (WHERE delivery.status='pending') AS last_error,
	       event.created_at, MAX(delivery.acknowledged_at) AS last_acknowledged_at
		FROM public.t_ztna_access_change_event AS event
		LEFT JOIN public.t_ztna_access_change_delivery AS delivery ON delivery.event_id=event.id
		WHERE event.subject_type=? AND event.subject_id=?
		GROUP BY event.id
		ORDER BY event.created_at DESC, event.id DESC
		LIMIT 1
	`, subjectType, subjectID).Get(&result)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, nil
	}
	return &result, nil
}
