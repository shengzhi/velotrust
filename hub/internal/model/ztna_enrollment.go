package model

import (
	"time"

	"velotrust/shared/repository/database"
)

const TableGatewayRegistrationToken = "t_gateway_registration_token"

// GatewayRegistrationToken is the HUB persistence projection used to consume
// a Management-issued one-time credential after the Gateway proves key
// possession. The plaintext token is never stored in this model.
type GatewayRegistrationToken struct {
	database.BaseModel `xorm:"extends"`
	ID                 int64      `xorm:"pk autoincr 'id'"`
	NodeID             int64      `xorm:"'node_id'"`
	TokenHash          string     `xorm:"'token_hash'"`
	Status             string     `xorm:"'status'"`
	MaxUses            int        `xorm:"'max_uses'"`
	UsedCount          int        `xorm:"'used_count'"`
	ExpiresAt          time.Time  `xorm:"'expires_at'"`
	UsedAt             *time.Time `xorm:"'used_at'"`
	UsedIP             *string    `xorm:"'used_ip'"`
}

func (GatewayRegistrationToken) TableName() string { return TableGatewayRegistrationToken }

var _ database.IModel = (*GatewayRegistrationToken)(nil)
