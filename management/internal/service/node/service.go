package node

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	servicecontext "velotrust/management/internal/context"
	model "velotrust/management/internal/model/ztna"
	"velotrust/management/internal/repository/node"
	appErrors "velotrust/pkg/errors"
)

const (
	readPermission  = "ztna.node.read"
	writePermission = "ztna.node.write"
)

type PermissionChecker interface {
	HasPermission(context.Context, int64, string) (bool, error)
}

type Service struct {
	store       node.Store
	permissions PermissionChecker
}

func New(store node.Store, permissions PermissionChecker) (*Service, error) {
	if store == nil || permissions == nil {
		return nil, errors.New("virtual-node store and IAM permission checker are required")
	}
	return &Service{store: store, permissions: permissions}, nil
}

func (s *Service) ListNodes(ctx *servicecontext.Context) ([]model.VirtualNodeView, error) {
	if _, err := s.authorize(ctx, readPermission); err != nil {
		return nil, err
	}
	return s.store.ListNodes(ctx)
}

func (s *Service) GetNode(ctx *servicecontext.Context, id int64) (*model.VirtualNodeView, error) {
	if _, err := s.authorize(ctx, readPermission); err != nil {
		return nil, err
	}
	if id <= 0 {
		return nil, invalid("id", "节点 ID 必须大于零")
	}
	return s.store.GetNode(ctx, id)
}

func (s *Service) SetNodeStatus(ctx *servicecontext.Context, id int64, status string, expectedVersion int64) error {
	actorID, err := s.authorize(ctx, writePermission)
	if err != nil {
		return err
	}
	if id <= 0 || expectedVersion <= 0 {
		return invalid("id", "节点 ID 和版本号必须有效")
	}
	switch status {
	case "active", "disabled", "quarantined", "revoked":
	default:
		return invalid("node_status", "节点状态必须是 active、disabled、quarantined 或 revoked")
	}
	return s.store.SetNodeStatus(ctx, actorID, id, status, expectedVersion)
}

func (s *Service) authorize(ctx *servicecontext.Context, permission string) (int64, error) {
	if ctx == nil {
		return 0, appErrors.NewAuthError("IAM_UNAUTHENTICATED", appErrors.AuthUnauthenticated)
	}
	actorID, err := strconv.ParseInt(ctx.UserID(), 10, 64)
	if err != nil || actorID <= 0 {
		return 0, appErrors.NewAuthError("IAM_UNAUTHENTICATED", appErrors.AuthUnauthenticated)
	}
	allowed, err := s.permissions.HasPermission(ctx, actorID, permission)
	if err != nil {
		return 0, fmt.Errorf("check virtual-node permission: %w", err)
	}
	if !allowed {
		return 0, appErrors.NewAuthError("IAM_PERMISSION_DENIED", appErrors.AuthPermissionDenied)
	}
	return actorID, nil
}

func invalid(field, message string) error {
	return appErrors.NewClientError("ZTNA_NODE_INVALID_ARGUMENT", map[string]string{field: message})
}
