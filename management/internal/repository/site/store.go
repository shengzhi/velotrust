package site

import (
	servicecontext "velotrust/management/internal/context"
	model "velotrust/management/internal/model/site"
)

type Store interface {
	ListSites(*servicecontext.Context) ([]model.PhysicalSite, error)
	ListGateways(*servicecontext.Context) ([]model.GatewayOption, error)
	SaveSite(*servicecontext.Context, int64, *model.PhysicalSite) error
	DeleteSite(*servicecontext.Context, int64, int64, int64) error
}
