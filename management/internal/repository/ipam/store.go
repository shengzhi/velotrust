package ipam

import (
	servicecontext "velotrust/management/internal/context"
	model "velotrust/management/internal/model/ipam"
)

type Store interface {
	ListNetworks(*servicecontext.Context, bool) ([]model.Network, error)
	ListActiveSiteNetworks(*servicecontext.Context) ([]model.Network, error)
	ListSelectableSiteNetworks(*servicecontext.Context) ([]model.Network, error)
	ResolveActiveNetworks(*servicecontext.Context, []int64) ([]model.Network, error)
	ResolveActiveSiteNetworks(*servicecontext.Context, []int64) ([]model.Network, error)
	CreateNetwork(*servicecontext.Context, int64, *model.Network, *model.AuditEvent) error
	SetNetworkStatus(*servicecontext.Context, int64, int64, string, int64, *model.AuditEvent) error
	GetOverlayNetwork(*servicecontext.Context) (*model.OverlayNetwork, error)
	SaveOverlayNetwork(*servicecontext.Context, int64, string, int64, *model.AuditEvent) (*model.OverlayNetwork, error)
	ListPools(*servicecontext.Context) ([]model.Pool, error)
	CreatePool(*servicecontext.Context, int64, *model.Pool, *model.AuditEvent) error
	UpdatePool(*servicecontext.Context, int64, *model.Pool, *model.AuditEvent) error
	SetPoolStatus(*servicecontext.Context, int64, int64, string, int64, *model.AuditEvent) error
	ListLeases(*servicecontext.Context) ([]model.Lease, error)
	ListEmployeeGroups(*servicecontext.Context) ([]model.PoolGroupOption, error)
	AllocateAddress(*servicecontext.Context, int64, *model.Lease, *model.AuditEvent) error
	ReleaseLease(*servicecontext.Context, int64, int64, *model.AuditEvent) error
	SaveSiteNATMapping(*servicecontext.Context, int64, *model.NATMapping, *model.AuditEvent) error
	CheckPrefix(*servicecontext.Context, string) (model.Conflict, error)
}
