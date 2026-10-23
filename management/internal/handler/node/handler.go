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
	"ListNodes":                {Authenticated: true, Permission: "ztna.node.read"},
	"GetNode":                  {Authenticated: true, Permission: "ztna.node.read"},
	"GetAccessChangeStatus":    {Authenticated: true, Permission: "ztna.node.read"},
	"ListGatewayTunnelOptions": {Authenticated: true, Permission: "ztna.tunnel.read"},
	"ListGroupGatewayOptions":  {Authenticated: true, Permission: "ztna.node.read"},
	"SetGroupGateways":         {Authenticated: true, Permission: "ztna.node.write"},
	"CreateLogicalTunnel":      {Authenticated: true, Permission: "ztna.tunnel.write"},
	"AssignGatewayTunnel":      {Authenticated: true, Permission: "ztna.tunnel.write"},
	"SetNodeStatus":            {Authenticated: true, Permission: "ztna.node.write"},
	"SetNodePublicEndpoint":    {Authenticated: true, Permission: "ztna.node.write"},
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

type GetAccessChangeStatusArgs struct {
	SubjectType string `json:"subject_type" validate:"required,oneof=user device node"`
	SubjectID   int64  `json:"subject_id" validate:"required,gt=0"`
}

func (h *Handler) GetAccessChangeStatus(ctx *jrpc.Context, args *GetAccessChangeStatusArgs) (*model.AccessChangeStatusView, error) {
	if args == nil {
		return nil, errors.New("access-change subject is required")
	}
	return h.service.GetAccessChangeStatus(middleware.ServiceContext(ctx), args.SubjectType, args.SubjectID)
}

type ListGatewayTunnelOptionsReply struct {
	Items []model.GatewayTunnelOption `json:"items"`
}

func (h *Handler) ListGatewayTunnelOptions(ctx *jrpc.Context, _ *struct{}) (*ListGatewayTunnelOptionsReply, error) {
	items, err := h.service.ListGatewayTunnelOptions(middleware.ServiceContext(ctx))
	if err != nil {
		return nil, err
	}
	return &ListGatewayTunnelOptionsReply{Items: items}, nil
}

type ListGroupGatewayOptionsArgs struct {
	GroupID int64 `json:"group_id" validate:"required,gt=0"`
}

type ListGroupGatewayOptionsReply struct {
	Items []model.GroupGatewayOption `json:"items"`
}

func (h *Handler) ListGroupGatewayOptions(ctx *jrpc.Context, args *ListGroupGatewayOptionsArgs) (*ListGroupGatewayOptionsReply, error) {
	if args == nil {
		return nil, errors.New("security-group ID is required")
	}
	items, err := h.service.ListGroupGatewayOptions(middleware.ServiceContext(ctx), args.GroupID)
	if err != nil {
		return nil, err
	}
	return &ListGroupGatewayOptionsReply{Items: items}, nil
}

type SetGroupGatewaysArgs struct {
	GroupID         int64   `json:"group_id" validate:"required,gt=0"`
	ExpectedVersion int64   `json:"expected_version" validate:"required,gt=0"`
	GatewayIDs      []int64 `json:"gateway_ids" validate:"max=1000,dive,gt=0"`
}

func (h *Handler) SetGroupGateways(ctx *jrpc.Context, args *SetGroupGatewaysArgs) (*MutationReply, error) {
	if args == nil {
		return nil, errors.New("security-group Gateway binding arguments are required")
	}
	if err := h.service.SetGroupGateways(middleware.ServiceContext(ctx), args.GroupID, args.ExpectedVersion, args.GatewayIDs); err != nil {
		return nil, err
	}
	return &MutationReply{Success: true}, nil
}

type CreateLogicalTunnelArgs struct {
	Code        string `json:"tunnel_code" validate:"required,max=64"`
	Description string `json:"description" validate:"omitempty,max=512"`
}

func (h *Handler) CreateLogicalTunnel(ctx *jrpc.Context, args *CreateLogicalTunnelArgs) (*model.GatewayTunnelOption, error) {
	if args == nil {
		return nil, errors.New("logical tunnel arguments are required")
	}
	return h.service.CreateLogicalTunnel(middleware.ServiceContext(ctx), args.Code, args.Description)
}

type AssignGatewayTunnelArgs struct {
	NodeID          int64 `json:"node_id" validate:"required,gt=0"`
	TunnelID        int64 `json:"tunnel_id" validate:"required,gt=0"`
	ExpectedVersion int64 `json:"expected_version" validate:"required,gt=0"`
}

func (h *Handler) AssignGatewayTunnel(ctx *jrpc.Context, args *AssignGatewayTunnelArgs) (*MutationReply, error) {
	if args == nil {
		return nil, errors.New("Gateway tunnel assignment arguments are required")
	}
	if err := h.service.AssignGatewayTunnel(middleware.ServiceContext(ctx), args.NodeID, args.TunnelID, args.ExpectedVersion); err != nil {
		return nil, err
	}
	return &MutationReply{Success: true}, nil
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

type SetNodePublicEndpointArgs struct {
	ID              int64  `json:"id" validate:"required,gt=0"`
	PublicEndpoint  string `json:"public_endpoint" validate:"omitempty,max=255"`
	ExpectedVersion int64  `json:"expected_version" validate:"required,gt=0"`
}

func (h *Handler) SetNodePublicEndpoint(ctx *jrpc.Context, args *SetNodePublicEndpointArgs) (*MutationReply, error) {
	if args == nil {
		return nil, errors.New("virtual-node public endpoint arguments are required")
	}
	if err := h.service.SetNodePublicEndpoint(middleware.ServiceContext(ctx), args.ID, args.PublicEndpoint, args.ExpectedVersion); err != nil {
		return nil, err
	}
	return &MutationReply{Success: true}, nil
}
