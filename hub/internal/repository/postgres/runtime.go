package postgres

import (
	"context"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"velotrust/hub/internal/auth"
	"velotrust/hub/internal/model"
	"velotrust/hub/internal/report"
	"velotrust/shared/repository/database"
)

// RuntimeRepository persists authenticated node liveness and status. It
// never mutates Management-owned lifecycle fields.
type RuntimeRepository struct {
	db *database.Database
}

func NewRuntimeRepository(db *database.Database, instanceID string) (*RuntimeRepository, error) {
	if db == nil || db.Engine() == nil {
		return nil, errors.New("HUB database is required")
	}
	if db.Driver() != "postgres" {
		return nil, fmt.Errorf("HUB runtime repository requires PostgreSQL, got %q", db.Driver())
	}
	if strings.TrimSpace(instanceID) == "" {
		return nil, errors.New("HUB instance ID is required")
	}
	return &RuntimeRepository{db: db}, nil
}

func (r *RuntimeRepository) RecordHeartbeat(ctx context.Context, nodeID string, expectedPublicKey []byte, at time.Time) error {
	if ctx == nil || strings.TrimSpace(nodeID) == "" || len(expectedPublicKey) != 32 || at.IsZero() {
		return errors.New("valid context, node identity, public key, and heartbeat time are required")
	}
	return r.db.WithSaveTransaction(ctx, func(tx *database.DBTransaction) error {
		node, err := r.lockActiveRuntimeNode(tx, nodeID, expectedPublicKey)
		if err != nil {
			return err
		}
		heartbeatAt, err := databaseCurrentTime(tx)
		if err != nil {
			return fmt.Errorf("read database time for node heartbeat: %w", err)
		}
		node.LastHeartbeatAt = &heartbeatAt
		actor := nodeAuditActor(node.NodeType, node.NodeCode)
		if actor == "" {
			return errors.New("unsupported node runtime audit principal")
		}
		node.Update(actor)
		node.Cols("last_heartbeat_at")
		if _, err := tx.Save(&node); err != nil {
			return fmt.Errorf("save node heartbeat: %w", err)
		}
		return nil
	})
}

func (r *RuntimeRepository) Report(ctx context.Context, status report.NodeStatus) error {
	if ctx == nil || strings.TrimSpace(status.NodeID) == "" || len(status.RegisteredPublicKey) != 32 {
		return fmt.Errorf("%w: authenticated Gateway identity is required", report.ErrInvalidRuntimeStatus)
	}
	if status.ReportedAt.IsZero() {
		return fmt.Errorf("%w: timestamp is required", report.ErrInvalidRuntimeStatus)
	}
	if status.ConnectedPeerCount > math.MaxInt32 {
		return fmt.Errorf("%w: connected peer count exceeds the database range", report.ErrInvalidRuntimeStatus)
	}
	if !validWireGuardStatus(status.WireGuardStatus) || !validHealthStatus(status.HealthStatus) {
		return fmt.Errorf("%w: unsupported status value", report.ErrInvalidRuntimeStatus)
	}
	if len(status.AgentVersion) > 64 || len(status.AppliedConfigHash) > 128 || len(status.LastErrorCode) > 128 {
		return fmt.Errorf("%w: a field exceeds its database limit", report.ErrInvalidRuntimeStatus)
	}
	if strings.ContainsAny(status.LastErrorCode, "\r\n\x00") {
		return fmt.Errorf("%w: error code contains invalid characters", report.ErrInvalidRuntimeStatus)
	}

	metrics, err := json.Marshal(map[string]any{
		"last_error_code":            status.LastErrorCode,
		"connected_peer_count_known": status.ConnectedPeerCountKnown,
		"status_source":              "agent_report",
	})
	if err != nil {
		return fmt.Errorf("encode node runtime status metadata: %w", err)
	}
	return r.db.WithSaveTransaction(ctx, func(tx *database.DBTransaction) error {
		node, err := r.lockActiveRuntimeNode(tx, status.NodeID, status.RegisteredPublicKey)
		if err != nil {
			return err
		}
		reportedAt := status.ReportedAt.UTC()
		heartbeatAt, err := databaseCurrentTime(tx)
		if err != nil {
			return fmt.Errorf("read database time for node runtime status: %w", err)
		}
		agentVersion := optionalString(status.AgentVersion)
		configHash := optionalString(status.AppliedConfigHash)
		node.AgentVersion = agentVersion
		node.ActualConfigHash = configHash
		node.HealthStatus = status.HealthStatus
		node.LastHeartbeatAt = &heartbeatAt
		actor := nodeAuditActor(node.NodeType, node.NodeCode)
		if actor == "" {
			return errors.New("unsupported node runtime audit principal")
		}
		node.Update(actor)
		node.Cols("agent_version", "actual_config_hash", "health_status", "last_heartbeat_at")
		if _, err := tx.Save(&node); err != nil {
			return fmt.Errorf("save node current runtime state: %w", err)
		}

		nodeStatus := model.NodeStatusReport{
			NodeID:             node.ID,
			ReportedAt:         reportedAt,
			AgentVersion:       agentVersion,
			ActualConfigHash:   configHash,
			WireGuardStatus:    status.WireGuardStatus,
			HealthStatus:       status.HealthStatus,
			ConnectedPeerCount: int(status.ConnectedPeerCount),
			MetricsJSON:        metrics,
		}
		nodeStatus.CreateNew(actor)
		if _, err := tx.Save(&nodeStatus); err != nil {
			return fmt.Errorf("save node status report: %w", err)
		}
		return nil
	})
}

func (r *RuntimeRepository) lockActiveRuntimeNode(tx *database.DBTransaction, nodeID string, expectedPublicKey []byte) (model.Node, error) {
	var node model.Node
	found, err := tx.SQL(`
		SELECT id, node_code, node_type, node_status, agent_version, public_key,
		       last_heartbeat_at, actual_config_hash, health_status,
		       created_at, created_by, updated_at, updated_by, version_no
		FROM public.t_ztna_node
		WHERE node_code = ? AND node_type IN ('gateway','client_agent')
		FOR UPDATE
	`, nodeID).Get(&node)
	if err != nil {
		return model.Node{}, fmt.Errorf("lock runtime node identity: %w", err)
	}
	if !found {
		if node.NodeType == "client_agent" {
			return model.Node{}, auth.ErrClientNotFound
		}
		return model.Node{}, auth.ErrGatewayNotFound
	}
	if node.NodeStatus != "active" {
		if node.NodeType == "client_agent" {
			return model.Node{}, auth.ErrClientNotAllowed
		}
		return model.Node{}, auth.ErrGatewayNotAllowed
	}
	if node.PublicKey == nil {
		return model.Node{}, auth.ErrPublicKeyMismatch
	}
	registeredKey, err := base64.StdEncoding.DecodeString(*node.PublicKey)
	if err != nil || len(registeredKey) != 32 || subtle.ConstantTimeCompare(registeredKey, expectedPublicKey) != 1 {
		return model.Node{}, auth.ErrPublicKeyMismatch
	}
	if node.NodeType == "client_agent" {
		var authorized bool
		found, err := tx.SQL(`
			SELECT EXISTS (
				SELECT 1 FROM public.t_ztna_device device
				JOIN public.t_iam_user owner ON owner.id=device.owner_user_id AND owner.user_status='active'
				JOIN public.t_ztna_device_credential credential ON credential.device_id=device.id
				WHERE device.node_id=? AND device.status='active'
				  AND credential.public_key=? AND credential.credential_type='x25519'
				  AND credential.status='active' AND credential.not_before<=CURRENT_TIMESTAMP
				  AND (credential.expires_at IS NULL OR credential.expires_at>CURRENT_TIMESTAMP)
			)
		`, node.ID, *node.PublicKey).Get(&authorized)
		if err != nil {
			return model.Node{}, fmt.Errorf("check Client authorization before runtime update: %w", err)
		}
		if !found || !authorized {
			return model.Node{}, auth.ErrClientNotAllowed
		}
	}
	return node, nil
}

func optionalString(value string) *string {
	if value == "" {
		return nil
	}
	copyValue := value
	return &copyValue
}

func databaseCurrentTime(tx *database.DBTransaction) (time.Time, error) {
	var row struct {
		CurrentTime time.Time `xorm:"'current_time'"`
	}
	found, err := tx.SQL("SELECT clock_timestamp() AS current_time").Get(&row)
	if err != nil {
		return time.Time{}, err
	}
	if !found || row.CurrentTime.IsZero() {
		return time.Time{}, errors.New("database did not return its current time")
	}
	return row.CurrentTime.UTC(), nil
}

func validWireGuardStatus(value string) bool {
	switch value {
	case "unknown", "down", "starting", "up", "error":
		return true
	default:
		return false
	}
}

func validHealthStatus(value string) bool {
	switch value {
	case "unknown", "healthy", "degraded", "unhealthy":
		return true
	default:
		return false
	}
}

var _ report.Reporter = (*RuntimeRepository)(nil)
var _ report.HeartbeatRecorder = (*RuntimeRepository)(nil)
