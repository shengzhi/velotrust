package node

import (
	"errors"
	"fmt"

	"velotrust/management/internal/middleware"
	model "velotrust/management/internal/model/ztna"
	appNode "velotrust/management/internal/service/node"
	appErrors "velotrust/pkg/errors"
	"velotrust/pkg/jrpc"
)

type Handler struct{ service *appNode.Service }

var methodPolicies = map[string]middleware.IAMMethodPolicy{
	"ListNodes":     {Authenticated: true, Permission: "ztna.node.read"},
	"GetNode":       {Authenticated: true, Permission: "ztna.node.read"},
	"SetNodeStatus": {Authenticated: true, Permission: "ztna.node.write"},
}

func Register(server *jrpc.Server, service *appNode.Service, checker middleware.PermissionChecker) error {
	if server == nil || service == nil || checker == nil {
		return errors.New("JRPC server, virtual-node service, and permission checker are required")
	}
	if err := server.RegisterServiceWithOptions(&Handler{service: service}, "NODE", jrpc.ServiceOptions{}, middleware.RequireIAMMethodPolicy(checker, methodPolicies)); err != nil {
		return err
	}
	registered := make(map[string]struct{}, len(methodPolicies))
	for _, descriptor := range server.Methods() {
		if descriptor.Service != "NODE" {
			continue
		}
		if _, ok := methodPolicies[descriptor.Method]; !ok {
			return fmt.Errorf("NODE JRPC method %s has no access declaration", descriptor.Method)
		}
		if descriptor.Access != jrpc.AccessAuthenticated {
			return fmt.Errorf("NODE JRPC method %s must require authentication", descriptor.Method)
		}
		registered[descriptor.Method] = struct{}{}
	}
	if len(registered) != len(methodPolicies) {
		return fmt.Errorf("NODE JRPC methods registered=%d access declarations=%d", len(registered), len(methodPolicies))
	}
	return nil
}

type ListNodesReply struct {
	Items []model.VirtualNodeView `json:"items"`
}

func (h *Handler) ListNodes(ctx *jrpc.Context, _ *struct{}) (*ListNodesReply, error) {
	items, err := h.service.ListNodes(middleware.ServiceContext(ctx))
	if err != nil {
		return nil, err
	}
	return &ListNodesReply{Items: items}, nil
}

type GetNodeArgs struct {
	ID int64 `json:"id" validate:"required,gt=0"`
}

func (h *Handler) GetNode(ctx *jrpc.Context, args *GetNodeArgs) (*model.VirtualNodeView, error) {
	if args == nil {
		return nil, errors.New("virtual-node ID is required")
	}
	item, err := h.service.GetNode(middleware.ServiceContext(ctx), args.ID)
	if err != nil {
		return nil, err
	}
	if item == nil {
		return nil, appErrors.NewResourceError("ZTNA_NODE_NOT_FOUND", "virtual node", true)
	}
	return item, nil
}

type SetNodeStatusArgs struct {
	ID              int64  `json:"id" validate:"required,gt=0"`
	NodeStatus      string `json:"node_status" validate:"required,oneof=active disabled quarantined revoked"`
	ExpectedVersion int64  `json:"expected_version" validate:"required,gt=0"`
}

type MutationReply struct {
	Success bool `json:"success"`
}

func (h *Handler) SetNodeStatus(ctx *jrpc.Context, args *SetNodeStatusArgs) (*MutationReply, error) {
	if args == nil {
		return nil, errors.New("virtual-node status arguments are required")
	}
	if err := h.service.SetNodeStatus(middleware.ServiceContext(ctx), args.ID, args.NodeStatus, args.ExpectedVersion); err != nil {
		return nil, err
	}
	return &MutationReply{Success: true}, nil
}
