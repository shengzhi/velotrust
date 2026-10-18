package enrollment

import (
	servicecontext "velotrust/management/internal/context"
	model "velotrust/management/internal/model/enrollment"
)

type Store interface {
	ListGateways(*servicecontext.Context) ([]model.Gateway, error)
	GetGateway(*servicecontext.Context, int64) (*model.Gateway, error)
	ListCredentials(*servicecontext.Context) ([]model.Credential, error)
	CreateGatewayAndCredential(*servicecontext.Context, int64, *model.Gateway, *model.Credential, *model.AuditEvent) error
	CreateCredential(*servicecontext.Context, int64, int64, *model.Credential, *model.AuditEvent) error
	RevokeCredential(*servicecontext.Context, int64, int64, string, *model.AuditEvent) error
	ExtendCredential(*servicecontext.Context, int64, int64, int, *model.AuditEvent) error
}
