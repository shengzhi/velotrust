package node

import (
	servicecontext "velotrust/management/internal/context"
	model "velotrust/management/internal/model/ztna"
)

type Store interface {
	ListNodes(*servicecontext.Context) ([]model.VirtualNodeView, error)
	GetNode(*servicecontext.Context, int64) (*model.VirtualNodeView, error)
	SetNodeStatus(*servicecontext.Context, int64, int64, string, int64) error
}
