package enrollment

import (
	"time"

	ztnaModel "velotrust/management/internal/model/ztna"
	"velotrust/shared/repository/database"
)

const (
	TableGateway    = ztnaModel.TableNode
	TableCredential = ztnaModel.TableGatewayRegistrationToken
	StatusPending   = "pending"
	StatusUsed      = "used"
	StatusExpired   = "expired"
	StatusRevoked   = "revoked"
	GatewayPending  = "pending_enrollment"

	GatewayTypeSite       = "site_gateway"
	GatewayTypeService    = "service_gateway"
	GatewayTypeRelay      = "relay_gateway"
	GatewayTypeHub        = "hub_gateway"
	GatewayTypeDatacenter = "datacenter_gateway"
	GatewayTypeBranch     = "branch_gateway"
)

// Gateway is the pre-created logical identity to which an enrollment
// credential is bound. Private keys and runtime WireGuard state are not stored
// here.
type Gateway struct {
	database.BaseModel `xorm:"extends"`
	ID                 int64      `xorm:"pk autoincr 'id'" json:"id"`
	GatewayID          string     `xorm:"'node_code'" json:"gateway_id"`
	GatewayName        string     `xorm:"'node_name'" json:"gateway_name"`
	GatewayType        string     `xorm:"'gateway_type'" json:"gateway_type"`
	NetworkName        string     `xorm:"-" json:"network_name"`
	GatewayNetworkName string     `xorm:"'gateway_network_name'" json:"-"`
	AllowedSubnets     []string   `xorm:"-" json:"allowed_subnets"`
	AllowedNetworkIDs  []int64    `xorm:"-" json:"-"`
	RequireApproval    bool       `xorm:"'require_approval'" json:"require_approval"`
	Status             string     `xorm:"-" json:"status"`
	NodeType           string     `xorm:"'node_type'" json:"-"`
	NodeStatus         string     `xorm:"'node_status'" json:"-"`
	RegisteredAt       *time.Time `xorm:"'registered_at'" json:"registered_at,omitempty"`
	CreatedByName      string     `xorm:"-" json:"created_by_name,omitempty"`
}

func (Gateway) TableName() string { return TableGateway }

// Credential contains metadata only. TokenHash is always excluded from JSON;
// the plaintext token is returned separately once from the issue operation.
type Credential struct {
	database.BaseModel `xorm:"extends"`
	ID                 int64      `xorm:"pk autoincr 'id'" json:"id"`
	NodeID             int64      `xorm:"'node_id'" json:"-"`
	GatewayID          string     `xorm:"-" json:"gateway_id"`
	GatewayName        string     `xorm:"-" json:"gateway_name"`
	GatewayType        string     `xorm:"-" json:"gateway_type"`
	NetworkName        string     `xorm:"-" json:"network_name"`
	AllowedSubnets     []string   `xorm:"-" json:"allowed_subnets"`
	RequireApproval    bool       `xorm:"-" json:"require_approval"`
	TokenHash          string     `xorm:"'token_hash'" json:"-"`
	Status             string     `xorm:"'status'" json:"status"`
	MaxUses            int        `xorm:"'max_uses'" json:"max_uses"`
	UsedCount          int        `xorm:"'used_count'" json:"used_count"`
	ExpiresAt          time.Time  `xorm:"'expires_at'" json:"expires_at"`
	Notes              *string    `xorm:"'notes'" json:"notes,omitempty"`
	CreatedByName      string     `xorm:"-" json:"created_by_name"`
	UsedAt             *time.Time `xorm:"'used_at'" json:"used_at,omitempty"`
	UsedIP             *string    `xorm:"'used_ip'" json:"used_ip,omitempty"`
	RevokedAt          *time.Time `xorm:"'revoked_at'" json:"revoked_at,omitempty"`
	RevocationReason   *string    `xorm:"'revocation_reason'" json:"revocation_reason,omitempty"`
}

func (Credential) TableName() string { return TableCredential }

type CredentialGrant struct {
	Credential Credential `json:"credential"`
	Token      string     `json:"token"`
}

// AuditEvent contains safe metadata only. Credential plaintext and digests
// must never be included in ChangesJSON.
type AuditEvent struct {
	EventID     string
	ActorID     int64
	ActorName   string
	Action      string
	TargetType  string
	TargetID    string
	TargetName  string
	Result      string
	Reason      string
	SourceIP    string
	RequestID   string
	ChangesJSON []byte
	OccurredAt  time.Time
}

var (
	_ database.IModel = (*Gateway)(nil)
	_ database.IModel = (*Credential)(nil)
)
