package ztna

import (
	"errors"
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
	if subjectType != "user" && subjectType != "device" && subjectType != "node" {
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
