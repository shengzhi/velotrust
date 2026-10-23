// Package ztna contains persistence models for the ZTNA business schema.
package ztna

import (
	"time"

	"velotrust/shared/repository/database"
)

const (
	TableDevice                   = "t_ztna_device"
	TableDeviceCredential         = "t_ztna_device_credential"
	TableDevicePostureCheck       = "t_ztna_device_posture_check"
	TableNode                     = "t_ztna_node"
	TableGatewayRegistrationToken = "t_gateway_registration_token"
	TableProtectedResource        = "t_ztna_protected_resource"
	TableAccessPolicy             = "t_ztna_access_policy"
	TablePolicySubject            = "t_ztna_policy_subject"
	TablePolicyResource           = "t_ztna_policy_resource"
	TableAuthorizationDecision    = "t_ztna_authorization_decision"
	TableAuthorizationGrant       = "t_ztna_authorization_grant"
	TableTunnel                   = "t_ztna_tunnel"
	TableTunnelPeer               = "t_ztna_tunnel_peer"
	TableNodeConfigTask           = "t_ztna_node_config_task"
	TableNodeConfigTaskEvent      = "t_ztna_node_config_task_event"
	TableNodeStatusReport         = "t_ztna_node_status_report"
	TableRelay                    = "t_ztna_relay"
	TableSecurityEvent            = "t_ztna_security_event"
	TableAccessChangeEvent        = "t_ztna_access_change_event"
	TableAccessChangeDelivery     = "t_ztna_access_change_delivery"
)

type Device struct {
	database.BaseModel `xorm:"extends"`
	ID                 int64      `xorm:"pk autoincr 'id'"`
	DeviceName         string     `xorm:"'device_name'"`
	DeviceUUID         string     `xorm:"'device_uuid'"`
	DeviceType         string     `xorm:"'device_type'"`
	Platform           string     `xorm:"'platform'"`
	PlatformVersion    *string    `xorm:"'platform_version'"`
	AgentVersion       *string    `xorm:"'agent_version'"`
	OwnerUserID        *int64     `xorm:"'owner_user_id'"`
	NodeID             *int64     `xorm:"'node_id'"`
	Status             string     `xorm:"'status'"`
	TrustStatus        string     `xorm:"'trust_status'"`
	LastSeenAt         *time.Time `xorm:"'last_seen_at'"`
	LastSeenIP         *string    `xorm:"'last_seen_ip'"`
	PostureJSON        []byte     `xorm:"'posture_json'"`
	RegisteredAt       *time.Time `xorm:"'registered_at'"`
	RevokedAt          *time.Time `xorm:"'revoked_at'"`
}

func (Device) TableName() string { return TableDevice }

// AccessChangeEvent is a durable request to refresh authorization state on
// connected Gateways. HUB owns delivery and acknowledgement state.
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

// AccessChangeDelivery tracks one Gateway's application of a current
// authorization snapshot for an event.
type AccessChangeDelivery struct {
	database.BaseModel `xorm:"extends"`
	ID                 int64      `xorm:"pk autoincr 'id'"`
	EventID            int64      `xorm:"'event_id'"`
	GatewayNodeID      int64      `xorm:"'gateway_node_id'"`
	Status             string     `xorm:"'status'"`
	LastNotifiedAt     *time.Time `xorm:"'last_notified_at'"`
	AcknowledgedAt     *time.Time `xorm:"'acknowledged_at'"`
	LastError          *string    `xorm:"'last_error'"`
}

func (AccessChangeDelivery) TableName() string { return TableAccessChangeDelivery }

type AccessChangeStatusView struct {
	EventID            int64      `json:"event_id"`
	SubjectType        string     `json:"subject_type"`
	SubjectID          int64      `json:"subject_id"`
	Status             string     `json:"status"`
	TargetCount        int64      `json:"target_count"`
	AcknowledgedCount  int64      `json:"acknowledged_count"`
	LastError          *string    `json:"last_error,omitempty"`
	CreatedAt          time.Time  `json:"created_at"`
	LastAcknowledgedAt *time.Time `json:"last_acknowledged_at,omitempty"`
}

type DeviceCredential struct {
	database.BaseModel `xorm:"extends"`
	ID                 int64      `xorm:"pk autoincr 'id'"`
	DeviceID           int64      `xorm:"'device_id'"`
	CredentialType     string     `xorm:"'credential_type'"`
	PublicKey          string     `xorm:"'public_key'"`
	Fingerprint        string     `xorm:"'fingerprint'"`
	KeyVersion         int        `xorm:"'key_version'"`
	Status             string     `xorm:"'status'"`
	NotBefore          time.Time  `xorm:"'not_before'"`
	ExpiresAt          *time.Time `xorm:"'expires_at'"`
	RevokedAt          *time.Time `xorm:"'revoked_at'"`
	RevocationReason   *string    `xorm:"'revocation_reason'"`
	LastUsedAt         *time.Time `xorm:"'last_used_at'"`
}

func (DeviceCredential) TableName() string { return TableDeviceCredential }

type DevicePostureCheck struct {
	database.BaseModel `xorm:"extends"`
	ID                 int64      `xorm:"pk autoincr 'id'"`
	DeviceID           int64      `xorm:"'device_id'"`
	CheckType          string     `xorm:"'check_type'"`
	Result             string     `xorm:"'result'"`
	ReasonCode         *string    `xorm:"'reason_code'"`
	DetailsJSON        []byte     `xorm:"'details_json'"`
	CheckedAt          time.Time  `xorm:"'checked_at'"`
	ValidUntil         *time.Time `xorm:"'valid_until'"`
	PolicyVersion      *int64     `xorm:"'policy_version'"`
}

func (DevicePostureCheck) TableName() string { return TableDevicePostureCheck }

type Node struct {
	database.BaseModel `xorm:"extends"`
	ID                 int64      `xorm:"pk autoincr 'id'"`
	NodeName           string     `xorm:"'node_name'"`
	NodeCode           string     `xorm:"'node_code'"`
	NodeType           string     `xorm:"'node_type'"`
	NodeStatus         string     `xorm:"'node_status'"`
	AgentVersion       *string    `xorm:"'agent_version'"`
	ControlEndpoint    *string    `xorm:"'control_endpoint'"`
	PublicEndpoint     *string    `xorm:"'public_endpoint'"`
	PrivateEndpoint    *string    `xorm:"'private_endpoint'"`
	PublicKey          *string    `xorm:"'public_key'"`
	LastHeartbeatAt    *time.Time `xorm:"'last_heartbeat_at'"`
	LastConfigHash     *string    `xorm:"'last_config_hash'"`
	ActualConfigHash   *string    `xorm:"'actual_config_hash'"`
	CapabilitiesJSON   []byte     `xorm:"'capabilities_json'"`
	HealthStatus       string     `xorm:"'health_status'"`
	GatewayType        *string    `xorm:"'gateway_type'"`
	GatewayNetworkName *string    `xorm:"'gateway_network_name'"`
	RequireApproval    bool       `xorm:"'require_approval'"`
	RegisteredAt       *time.Time `xorm:"'registered_at'"`
}

func (Node) TableName() string { return TableNode }

type ProtectedResource struct {
	database.BaseModel `xorm:"extends"`
	ID                 int64   `xorm:"pk autoincr 'id'"`
	ResourceName       string  `xorm:"'resource_name'"`
	ResourceCode       string  `xorm:"'resource_code'"`
	ResourceType       string  `xorm:"'resource_type'"`
	NetworkID          *int64  `xorm:"'network_id'"`
	Host               *string `xorm:"'host'"`
	Address            *string `xorm:"'address'"`
	PortStart          *int    `xorm:"'port_start'"`
	PortEnd            *int    `xorm:"'port_end'"`
	Protocol           *string `xorm:"'protocol'"`
	AccessMode         string  `xorm:"'access_mode'"`
	GatewayNodeID      *int64  `xorm:"'gateway_node_id'"`
	Category           *string `xorm:"'category'"`
	TagsJSON           []byte  `xorm:"'tags_json'"`
	Description        *string `xorm:"'description'"`
	Status             string  `xorm:"'status'"`
}

func (ProtectedResource) TableName() string { return TableProtectedResource }

// AccessPolicy stores the only current ruleset. BaseModel.VersionNo is used
// for optimistic persistence concurrency; this model has no business revision.
type AccessPolicy struct {
	database.BaseModel `xorm:"extends"`
	ID                 int64   `xorm:"pk autoincr 'id'"`
	PolicyName         string  `xorm:"'policy_name'"`
	PolicyCode         string  `xorm:"'policy_code'"`
	Description        *string `xorm:"'description'"`
	Priority           int     `xorm:"'priority'"`
	Status             string  `xorm:"'status'"`
	RulesJSON          []byte  `xorm:"'rules_json'"`
	CompiledHash       *string `xorm:"'compiled_hash'"`
}

func (AccessPolicy) TableName() string { return TableAccessPolicy }

type PolicySubject struct {
	database.BaseModel `xorm:"extends"`
	ID                 int64  `xorm:"pk autoincr 'id'"`
	PolicyID           int64  `xorm:"'policy_id'"`
	SubjectType        string `xorm:"'subject_type'"`
	SubjectID          int64  `xorm:"'subject_id'"`
}

func (PolicySubject) TableName() string { return TablePolicySubject }

type PolicyResource struct {
	database.BaseModel  `xorm:"extends"`
	ID                  int64 `xorm:"pk autoincr 'id'"`
	PolicyID            int64 `xorm:"'policy_id'"`
	ProtectedResourceID int64 `xorm:"'protected_resource_id'"`
}

func (PolicyResource) TableName() string { return TablePolicyResource }

type AuthorizationDecision struct {
	database.BaseModel  `xorm:"extends"`
	ID                  int64     `xorm:"pk autoincr 'id'"`
	DecisionID          string    `xorm:"'decision_id'"`
	UserID              *int64    `xorm:"'user_id'"`
	DeviceID            *int64    `xorm:"'device_id'"`
	ProtectedResourceID *int64    `xorm:"'protected_resource_id'"`
	PolicyID            *int64    `xorm:"'policy_id'"`
	Decision            string    `xorm:"'decision'"`
	ReasonCode          string    `xorm:"'reason_code'"`
	AuthLevel           *string   `xorm:"'auth_level'"`
	DeviceTrustStatus   *string   `xorm:"'device_trust_status'"`
	SourceIP            *string   `xorm:"'source_ip'"`
	RequestID           *string   `xorm:"'request_id'"`
	DecidedAt           time.Time `xorm:"'decided_at'"`
}

func (AuthorizationDecision) TableName() string { return TableAuthorizationDecision }

type AuthorizationGrant struct {
	database.BaseModel  `xorm:"extends"`
	ID                  int64      `xorm:"pk autoincr 'id'"`
	GrantID             string     `xorm:"'grant_id'"`
	UserID              int64      `xorm:"'user_id'"`
	DeviceID            *int64     `xorm:"'device_id'"`
	ProtectedResourceID int64      `xorm:"'protected_resource_id'"`
	RequestedBy         int64      `xorm:"'requested_by'"`
	ApprovedBy          *int64     `xorm:"'approved_by'"`
	Reason              string     `xorm:"'reason'"`
	Status              string     `xorm:"'status'"`
	ValidFrom           *time.Time `xorm:"'valid_from'"`
	ExpiresAt           *time.Time `xorm:"'expires_at'"`
	RevokedAt           *time.Time `xorm:"'revoked_at'"`
}

func (AuthorizationGrant) TableName() string { return TableAuthorizationGrant }

type Tunnel struct {
	database.BaseModel `xorm:"extends"`
	ID                 int64   `xorm:"pk autoincr 'id'"`
	TunnelCode         string  `xorm:"'tunnel_code'"`
	TunnelType         string  `xorm:"'tunnel_type'"`
	Status             string  `xorm:"'status'"`
	NetworkID          *int64  `xorm:"'network_id'"`
	Description        *string `xorm:"'description'"`
}

func (Tunnel) TableName() string { return TableTunnel }

type TunnelPeer struct {
	database.BaseModel  `xorm:"extends"`
	ID                  int64      `xorm:"pk autoincr 'id'"`
	TunnelID            int64      `xorm:"'tunnel_id'"`
	NodeID              int64      `xorm:"'node_id'"`
	DeviceID            *int64     `xorm:"'device_id'"`
	PublicKey           string     `xorm:"'public_key'"`
	TunnelIP            string     `xorm:"'tunnel_ip'"`
	AllowedIPs          []byte     `xorm:"'allowed_ips'"`
	Endpoint            *string    `xorm:"'endpoint'"`
	PersistentKeepalive int        `xorm:"'persistent_keepalive'"`
	Status              string     `xorm:"'status'"`
	KeyVersion          int        `xorm:"'key_version'"`
	LastHandshakeAt     *time.Time `xorm:"'last_handshake_at'"`
	LastSeenAt          *time.Time `xorm:"'last_seen_at'"`
}

func (TunnelPeer) TableName() string { return TableTunnelPeer }

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

type Relay struct {
	database.BaseModel `xorm:"extends"`
	ID                 int64      `xorm:"pk autoincr 'id'"`
	NodeID             int64      `xorm:"'node_id'"`
	RelayCode          string     `xorm:"'relay_code'"`
	Region             *string    `xorm:"'region'"`
	Endpoint           string     `xorm:"'endpoint'"`
	Capacity           int        `xorm:"'capacity'"`
	CurrentLoad        int        `xorm:"'current_load'"`
	Status             string     `xorm:"'status'"`
	LastHealthCheckAt  *time.Time `xorm:"'last_health_check_at'"`
}

func (Relay) TableName() string { return TableRelay }

type SecurityEvent struct {
	database.BaseModel  `xorm:"extends"`
	ID                  int64      `xorm:"pk autoincr 'id'"`
	EventID             string     `xorm:"'event_id'"`
	EventType           string     `xorm:"'event_type'"`
	Severity            string     `xorm:"'severity'"`
	Status              string     `xorm:"'status'"`
	UserID              *int64     `xorm:"'user_id'"`
	DeviceID            *int64     `xorm:"'device_id'"`
	NodeID              *int64     `xorm:"'node_id'"`
	ProtectedResourceID *int64     `xorm:"'protected_resource_id'"`
	ReasonCode          string     `xorm:"'reason_code'"`
	DetailsJSON         []byte     `xorm:"'details_json'"`
	OccurredAt          time.Time  `xorm:"'occurred_at'"`
	AcknowledgedAt      *time.Time `xorm:"'acknowledged_at'"`
	AcknowledgedBy      *int64     `xorm:"'acknowledged_by'"`
	ResolvedAt          *time.Time `xorm:"'resolved_at'"`
	ResolvedBy          *int64     `xorm:"'resolved_by'"`
}

func (SecurityEvent) TableName() string { return TableSecurityEvent }

// PolicySubjectRef and AccessPolicyData are service-facing projections of a
// current policy and its current relationship rows; neither is a persistence
// table or a policy revision.
type PolicySubjectRef struct {
	Type string `json:"subject_type"`
	ID   int64  `json:"subject_id"`
}

type AccessPolicyData struct {
	Policy      AccessPolicy       `json:"policy"`
	Subjects    []PolicySubjectRef `json:"subjects"`
	ResourceIDs []int64            `json:"resource_ids"`
}

var (
	_ database.IModel = (*Device)(nil)
	_ database.IModel = (*DeviceCredential)(nil)
	_ database.IModel = (*DevicePostureCheck)(nil)
	_ database.IModel = (*Node)(nil)
	_ database.IModel = (*ProtectedResource)(nil)
	_ database.IModel = (*AccessPolicy)(nil)
	_ database.IModel = (*PolicySubject)(nil)
	_ database.IModel = (*PolicyResource)(nil)
	_ database.IModel = (*AuthorizationDecision)(nil)
	_ database.IModel = (*AuthorizationGrant)(nil)
	_ database.IModel = (*Tunnel)(nil)
	_ database.IModel = (*TunnelPeer)(nil)
	_ database.IModel = (*NodeConfigTask)(nil)
	_ database.IModel = (*NodeConfigTaskEvent)(nil)
	_ database.IModel = (*NodeStatusReport)(nil)
	_ database.IModel = (*Relay)(nil)
	_ database.IModel = (*SecurityEvent)(nil)
)
