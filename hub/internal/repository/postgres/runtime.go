package postgres

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
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

// RuntimeRepository persists authenticated Gateway liveness and status. It
// never mutates Management-owned lifecycle fields.
type RuntimeRepository struct {
	db    *database.Database
	actor string
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
	return &RuntimeRepository{db: db, actor: auditActor(instanceID)}, nil
}

func (r *RuntimeRepository) RecordHeartbeat(ctx context.Context, nodeID string, expectedPublicKey []byte, at time.Time) error {
	if ctx == nil || strings.TrimSpace(nodeID) == "" || len(expectedPublicKey) != 32 || at.IsZero() {
		return errors.New("valid context, Gateway identity, public key, and heartbeat time are required")
	}
	return r.db.WithSaveTransaction(ctx, func(tx *database.DBTransaction) error {
		node, err := r.lockActiveGateway(tx, nodeID, expectedPublicKey)
		if err != nil {
			return err
		}
		heartbeatAt := at.UTC()
		node.LastHeartbeatAt = &heartbeatAt
		node.Update(r.actor)
		node.Cols("last_heartbeat_at")
		if _, err := tx.Save(&node); err != nil {
			return fmt.Errorf("save Gateway heartbeat: %w", err)
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
		"status_source":              "gateway_reconciler",
	})
	if err != nil {
		return fmt.Errorf("encode Gateway runtime status metadata: %w", err)
	}
	return r.db.WithSaveTransaction(ctx, func(tx *database.DBTransaction) error {
		node, err := r.lockActiveGateway(tx, status.NodeID, status.RegisteredPublicKey)
		if err != nil {
			return err
		}
		reportedAt := status.ReportedAt.UTC()
		agentVersion := optionalString(status.AgentVersion)
		configHash := optionalString(status.AppliedConfigHash)
		node.AgentVersion = agentVersion
		node.ActualConfigHash = configHash
		node.HealthStatus = status.HealthStatus
		node.Update(r.actor)
		node.Cols("agent_version", "actual_config_hash", "health_status")
		if _, err := tx.Save(&node); err != nil {
			return fmt.Errorf("save Gateway current runtime state: %w", err)
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
		nodeStatus.CreateNew(r.actor)
		if _, err := tx.Save(&nodeStatus); err != nil {
			return fmt.Errorf("save Gateway status report: %w", err)
		}
		return nil
	})
}

func (r *RuntimeRepository) lockActiveGateway(tx *database.DBTransaction, nodeID string, expectedPublicKey []byte) (model.Node, error) {
	var node model.Node
	found, err := tx.SQL(`
		SELECT id, node_code, node_type, node_status, agent_version, public_key,
		       last_heartbeat_at, actual_config_hash, health_status,
		       created_at, created_by, updated_at, updated_by, version_no
		FROM public.t_ztna_node
		WHERE node_code = ? AND node_type = 'gateway'
		FOR UPDATE
	`, nodeID).Get(&node)
	if err != nil {
		return model.Node{}, fmt.Errorf("lock Gateway runtime identity: %w", err)
	}
	if !found {
		return model.Node{}, auth.ErrGatewayNotFound
	}
	if node.NodeStatus != "active" {
		return model.Node{}, auth.ErrGatewayNotAllowed
	}
	if node.PublicKey == nil {
		return model.Node{}, auth.ErrPublicKeyMismatch
	}
	registeredKey, err := base64.StdEncoding.DecodeString(*node.PublicKey)
	if err != nil || len(registeredKey) != 32 || subtle.ConstantTimeCompare(registeredKey, expectedPublicKey) != 1 {
		return model.Node{}, auth.ErrPublicKeyMismatch
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

func auditActor(instanceID string) string {
	actor := "hub:" + instanceID
	if len(actor) <= 64 {
		return actor
	}
	hash := sha256.Sum256([]byte(instanceID))
	return "hub:" + hex.EncodeToString(hash[:])
}

var _ report.Reporter = (*RuntimeRepository)(nil)
var _ report.HeartbeatRecorder = (*RuntimeRepository)(nil)
