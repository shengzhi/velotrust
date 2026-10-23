package model

import (
	"time"

	"velotrust/shared/repository/database"
)

const (
	TableNode                = "t_ztna_node"
	TableNodeStatusReport    = "t_ztna_node_status_report"
	TableNodeConfigTask      = "t_ztna_node_config_task"
	TableNodeConfigTaskEvent = "t_ztna_node_config_task_event"
	TableAccessChangeEvent   = "t_ztna_access_change_event"
)

// Node is the HUB persistence projection of the Management-owned node table.
// HUB creates Client node identity during a successful employee login, then
// updates only runtime-owned columns for existing nodes.
type Node struct {
	database.BaseModel `xorm:"extends"`
	ID                 int64      `xorm:"pk autoincr 'id'"`
	NodeName           string     `xorm:"'node_name'"`
	NodeCode           string     `xorm:"'node_code'"`
	NodeType           string     `xorm:"'node_type'"`
	NodeStatus         string     `xorm:"'node_status'"`
	AgentVersion       *string    `xorm:"'agent_version'"`
	CapabilitiesJSON   []byte     `xorm:"'capabilities_json'"`
	PublicKey          *string    `xorm:"'public_key'"`
	LastHeartbeatAt    *time.Time `xorm:"'last_heartbeat_at'"`
	LastConfigHash     *string    `xorm:"'last_config_hash'"`
	ActualConfigHash   *string    `xorm:"'actual_config_hash'"`
	HealthStatus       string     `xorm:"'health_status'"`
	RequireApproval    bool       `xorm:"'require_approval'"`
	RegisteredAt       *time.Time `xorm:"'registered_at'"`
}

func (Node) TableName() string { return TableNode }

// NodeStatusReport stores a timestamped, sanitized view of the runtime state
// accepted on an authenticated Gateway stream.
type NodeStatusReport struct {
	database.BaseModel `xorm:"extends"`
	ID                 int64     `xorm:"pk autoincr 'id'"`
	NodeID             int64     `xorm:"'node_id'"`
	ReportedAt         time.Time `xorm:"'reported_at'"`
	AgentVersion       *string   `xorm:"'agent_version'"`
	ActualConfigHash   *string   `xorm:"'actual_config_hash'"`
	WireGuardStatus    string    `xorm:"'wireguard_status'"`
	HealthStatus       string    `xorm:"'health_status'"`
	ConnectedPeerCount int       `xorm:"'connected_peer_count'"`
	MetricsJSON        []byte    `xorm:"'metrics_json'"`
}

func (NodeStatusReport) TableName() string { return TableNodeStatusReport }

// NodeConfigTask is the HUB's runtime projection of a Management-owned
// configuration delivery task.
type NodeConfigTask struct {
	database.BaseModel `xorm:"extends"`
	ID                 int64      `xorm:"pk autoincr 'id'"`
	TaskID             string     `xorm:"'task_id'"`
	NodeID             int64      `xorm:"'node_id'"`
	TargetConfigHash   *string    `xorm:"'target_config_hash'"`
	TaskType           string     `xorm:"'task_type'"`
	Status             string     `xorm:"'status'"`
	RetryCount         int        `xorm:"'retry_count'"`
	NextRetryAt        *time.Time `xorm:"'next_retry_at'"`
	LastErrorCode      *string    `xorm:"'last_error_code'"`
	LastErrorMessage   *string    `xorm:"'last_error_message'"`
	StartedAt          *time.Time `xorm:"'started_at'"`
	FinishedAt         *time.Time `xorm:"'finished_at'"`
}

func (NodeConfigTask) TableName() string { return TableNodeConfigTask }

// NodeConfigTaskEvent records an append-only state transition for a delivery.
type NodeConfigTaskEvent struct {
	database.BaseModel `xorm:"extends"`
	ID                 int64     `xorm:"pk autoincr 'id'"`
	TaskID             string    `xorm:"'task_id'"`
	NodeID             int64     `xorm:"'node_id'"`
	EventType          string    `xorm:"'event_type'"`
	FromStatus         *string   `xorm:"'from_status'"`
	ToStatus           *string   `xorm:"'to_status'"`
	DetailJSON         []byte    `xorm:"'detail_json'"`
	OccurredAt         time.Time `xorm:"'occurred_at'"`
}

func (NodeConfigTaskEvent) TableName() string { return TableNodeConfigTaskEvent }

// AccessChangeEvent is the HUB projection of the shared durable authorization
// outbox. HUB appends node-scoped refresh events when a Client receives a new
// Overlay lease so all bound Gateways install the corresponding WireGuard peer.
type AccessChangeEvent struct {
	database.BaseModel `xorm:"extends"`
	ID                 int64  `xorm:"pk autoincr 'id'"`
	SubjectType        string `xorm:"'subject_type'"`
	SubjectID          int64  `xorm:"'subject_id'"`
	ChangeType         string `xorm:"'change_type'"`
	Status             string `xorm:"'status'"`
	TargetsInitialized bool   `xorm:"'targets_initialized'"`
}

func (AccessChangeEvent) TableName() string { return TableAccessChangeEvent }

var (
	_ database.IModel = (*Node)(nil)
	_ database.IModel = (*NodeStatusReport)(nil)
	_ database.IModel = (*NodeConfigTask)(nil)
	_ database.IModel = (*NodeConfigTaskEvent)(nil)
	_ database.IModel = (*AccessChangeEvent)(nil)
)
