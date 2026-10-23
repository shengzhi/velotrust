package node

import (
	servicecontext "velotrust/management/internal/context"
	model "velotrust/management/internal/model/ztna"
)

type Store interface {
	ListNodes(*servicecontext.Context) ([]model.VirtualNodeView, error)
	GetNode(*servicecontext.Context, int64) (*model.VirtualNodeView, error)
	GetAccessChangeStatus(*servicecontext.Context, string, int64) (*model.AccessChangeStatusView, error)
	ListGatewayTunnelOptions(*servicecontext.Context) ([]model.GatewayTunnelOption, error)
	CreateLogicalTunnel(*servicecontext.Context, int64, string, string) (*model.GatewayTunnelOption, error)
	AssignGatewayTunnel(*servicecontext.Context, int64, int64, int64, int64) error
	SetNodeStatus(*servicecontext.Context, int64, int64, string, int64) error
	SetNodePublicEndpoint(*servicecontext.Context, int64, int64, string, int64) error
}
