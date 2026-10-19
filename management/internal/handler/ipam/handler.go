package ipam

import (
	"errors"
	"fmt"

	"velotrust/management/internal/middleware"
	model "velotrust/management/internal/model/ipam"
	appIPAM "velotrust/management/internal/service/ipam"
	"velotrust/pkg/jrpc"
)

type Handler struct{ service *appIPAM.Service }

var methodPolicies = map[string]middleware.IAMMethodPolicy{
	"ListNetworks":           {Authenticated: true, Permission: "ipam.network.read"},
	"ListSelectableNetworks": {Authenticated: true, Permission: "ipam.network.read"},
	"CreateNetwork":          {Authenticated: true, Permission: "ipam.network.write"},
	"SetNetworkStatus":       {Authenticated: true, Permission: "ipam.network.write"},
	"Overview":               {Authenticated: true, Permission: "ipam.network.read"},
	"SaveOverlayNetwork":     {Authenticated: true, Permission: "ipam.network.write"},
	"ListPools":              {Authenticated: true, Permission: "ipam.network.read"},
	"CreatePool":             {Authenticated: true, Permission: "ipam.network.write"},
	"UpdatePool":             {Authenticated: true, Permission: "ipam.network.write"},
	"SetPoolStatus":          {Authenticated: true, Permission: "ipam.network.write"},
	"ListLeases":             {Authenticated: true, Permission: "ipam.network.read"},
	"AllocateAddress":        {Authenticated: true, Permission: "ipam.network.write"},
	"ReleaseLease":           {Authenticated: true, Permission: "ipam.network.write"},
	"CheckPrefix":            {Authenticated: true, Permission: "ipam.network.read"},
}

func Register(server *jrpc.Server, service *appIPAM.Service, checker middleware.PermissionChecker) error {
	if server == nil || service == nil || checker == nil {
		return errors.New("JRPC server, IPAM service, and permission checker are required")
	}
	handler := &Handler{service: service}
	if err := server.RegisterServiceWithOptions(handler, "IPAM", jrpc.ServiceOptions{}, middleware.RequireIAMMethodPolicy(checker, methodPolicies)); err != nil {
		return err
	}
	registered := make(map[string]struct{}, len(methodPolicies))
	for _, descriptor := range server.Methods() {
		if descriptor.Service != "IPAM" {
			continue
		}
		if _, ok := methodPolicies[descriptor.Method]; !ok {
			return fmt.Errorf("IPAM JRPC method %s has no access declaration", descriptor.Method)
		}
		if descriptor.Access != jrpc.AccessAuthenticated {
			return fmt.Errorf("IPAM JRPC method %s must require authentication", descriptor.Method)
		}
		registered[descriptor.Method] = struct{}{}
	}
	if len(registered) != len(methodPolicies) {
		return fmt.Errorf("IPAM JRPC methods registered=%d access declarations=%d", len(registered), len(methodPolicies))
	}
	return nil
}

type ListNetworksReply struct {
	Items []model.Network `json:"items"`
}

func (h *Handler) ListNetworks(ctx *jrpc.Context, _ *struct{}) (*ListNetworksReply, error) {
	items, err := h.service.ListNetworks(middleware.ServiceContext(ctx))
	if err != nil {
		return nil, err
	}
	return &ListNetworksReply{Items: items}, nil
}

func (h *Handler) ListSelectableNetworks(ctx *jrpc.Context, _ *struct{}) (*ListNetworksReply, error) {
	items, err := h.service.ListSelectableNetworks(middleware.ServiceContext(ctx))
	if err != nil {
		return nil, err
	}
	return &ListNetworksReply{Items: items}, nil
}

func (h *Handler) Overview(ctx *jrpc.Context, _ *struct{}) (*model.Overview, error) {
	return h.service.Overview(middleware.ServiceContext(ctx))
}

type SaveOverlayNetworkArgs struct {
	CIDR            string `json:"cidr" validate:"required,max=64"`
	ExpectedVersion int64  `json:"expected_version" validate:"gte=0"`
}

func (h *Handler) SaveOverlayNetwork(ctx *jrpc.Context, args *SaveOverlayNetworkArgs) (*model.OverlayNetwork, error) {
	if args == nil {
		return nil, errors.New("global Overlay network arguments are required")
	}
	return h.service.SaveOverlayNetwork(middleware.ServiceContext(ctx), args.CIDR, args.ExpectedVersion)
}

type ListPoolsReply struct {
	Items []model.Pool `json:"items"`
}

func (h *Handler) ListPools(ctx *jrpc.Context, _ *struct{}) (*ListPoolsReply, error) {
	items, err := h.service.ListPools(middleware.ServiceContext(ctx))
	if err != nil {
		return nil, err
	}
	return &ListPoolsReply{Items: items}, nil
}

type PoolArgs struct {
	PoolName       string  `json:"pool_name" validate:"required,max=128"`
	CIDR           string  `json:"cidr" validate:"required,max=64"`
	PoolType       string  `json:"pool_type" validate:"required,oneof=Gateway User-Device Service-VIP"`
	AllocationMode string  `json:"allocation_mode" validate:"required,oneof=static dynamic"`
	IDPGroup       *string `json:"idp_group" validate:"omitempty,max=128"`
	GatewayBinding *string `json:"gateway_binding" validate:"omitempty,max=128"`
	Color          string  `json:"color" validate:"omitempty,len=7"`
	Description    *string `json:"description" validate:"omitempty,max=512"`
}
type CreatePoolArgs struct{ PoolArgs }

func (h *Handler) CreatePool(ctx *jrpc.Context, args *CreatePoolArgs) (*model.Pool, error) {
	if args == nil {
		return nil, errors.New("IPAM pool arguments are required")
	}
	return h.service.CreatePool(middleware.ServiceContext(ctx), model.Pool{PoolName: args.PoolName, NetworkCIDR: args.CIDR, Type: args.PoolType, AllocationMode: args.AllocationMode, IDPGroup: args.IDPGroup, GatewayBinding: args.GatewayBinding, Color: args.Color, Description: args.Description})
}

type UpdatePoolArgs struct {
	PoolID          int64 `json:"pool_id" validate:"required,gt=0"`
	ExpectedVersion int64 `json:"expected_version" validate:"required,gt=0"`
	PoolArgs
}

func (h *Handler) UpdatePool(ctx *jrpc.Context, args *UpdatePoolArgs) (*model.Pool, error) {
	if args == nil {
		return nil, errors.New("IPAM pool arguments are required")
	}
	pool := model.Pool{ID: args.PoolID, PoolName: args.PoolName, NetworkCIDR: args.CIDR, Type: args.PoolType, AllocationMode: args.AllocationMode, IDPGroup: args.IDPGroup, GatewayBinding: args.GatewayBinding, Color: args.Color, Description: args.Description}
	pool.VersionNo = args.ExpectedVersion
	return h.service.UpdatePool(middleware.ServiceContext(ctx), pool)
}

type SetPoolStatusArgs struct {
	PoolID          int64  `json:"pool_id" validate:"required,gt=0"`
	Status          string `json:"status" validate:"required,oneof=active disabled"`
	ExpectedVersion int64  `json:"expected_version" validate:"required,gt=0"`
}

func (h *Handler) SetPoolStatus(ctx *jrpc.Context, args *SetPoolStatusArgs) (*MutationReply, error) {
	if args == nil {
		return nil, errors.New("IPAM pool status arguments are required")
	}
	if err := h.service.SetPoolStatus(middleware.ServiceContext(ctx), args.PoolID, args.Status, args.ExpectedVersion); err != nil {
		return nil, err
	}
	return &MutationReply{Success: true}, nil
}

type ListLeasesReply struct {
	Items []model.Lease `json:"items"`
}

func (h *Handler) ListLeases(ctx *jrpc.Context, _ *struct{}) (*ListLeasesReply, error) {
	items, err := h.service.ListLeases(middleware.ServiceContext(ctx))
	if err != nil {
		return nil, err
	}
	return &ListLeasesReply{Items: items}, nil
}

type AllocateAddressArgs struct {
	PoolID          int64  `json:"pool_id" validate:"required,gt=0"`
	TargetName      string `json:"target_name" validate:"required,max=128"`
	NodeType        string `json:"node_type" validate:"required,oneof=Gateway Peer Server"`
	RequestedIP     string `json:"requested_ip" validate:"omitempty,max=64"`
	IDPGroup        string `json:"idp_group" validate:"omitempty,max=128"`
	WireGuardPubKey string `json:"wireguard_pubkey" validate:"omitempty,len=44"`
	Endpoint        string `json:"endpoint" validate:"omitempty,max=255"`
}

func (h *Handler) AllocateAddress(ctx *jrpc.Context, args *AllocateAddressArgs) (*model.Lease, error) {
	if args == nil {
		return nil, errors.New("IP address allocation arguments are required")
	}
	return h.service.AllocateAddress(middleware.ServiceContext(ctx), model.Allocation{PoolID: args.PoolID, TargetName: args.TargetName, NodeType: args.NodeType, RequestedIP: args.RequestedIP, IDPGroup: args.IDPGroup, WireGuardPubKey: args.WireGuardPubKey, Endpoint: args.Endpoint})
}

type ReleaseLeaseArgs struct {
	LeaseID int64 `json:"lease_id" validate:"required,gt=0"`
}

func (h *Handler) ReleaseLease(ctx *jrpc.Context, args *ReleaseLeaseArgs) (*MutationReply, error) {
	if args == nil {
		return nil, errors.New("IP lease release arguments are required")
	}
	if err := h.service.ReleaseLease(middleware.ServiceContext(ctx), args.LeaseID); err != nil {
		return nil, err
	}
	return &MutationReply{Success: true}, nil
}

type CheckPrefixArgs struct {
	CIDR string `json:"cidr" validate:"required,max=64"`
}

func (h *Handler) CheckPrefix(ctx *jrpc.Context, args *CheckPrefixArgs) (*model.Conflict, error) {
	if args == nil {
		return nil, errors.New("CIDR check arguments are required")
	}
	result, err := h.service.CheckPrefix(middleware.ServiceContext(ctx), args.CIDR)
	if err != nil {
		return nil, err
	}
	return &result, nil
}

type CreateNetworkArgs struct {
	NetworkName string `json:"network_name" validate:"required,max=128"`
	NetworkCode string `json:"network_code" validate:"required,min=2,max=64"`
	NetworkType string `json:"network_type" validate:"required,oneof=vpc subnet segment"`
	CIDR        string `json:"cidr" validate:"required,max=64"`
	Region      string `json:"region" validate:"omitempty,max=128"`
	Environment string `json:"environment" validate:"omitempty,max=32"`
	Description string `json:"description" validate:"omitempty,max=512"`
}

func (h *Handler) CreateNetwork(ctx *jrpc.Context, args *CreateNetworkArgs) (*model.Network, error) {
	if args == nil {
		return nil, errors.New("IPAM network arguments are required")
	}
	return h.service.CreateNetwork(middleware.ServiceContext(ctx), appIPAM.CreateNetworkInput{
		NetworkName: args.NetworkName, NetworkCode: args.NetworkCode, NetworkType: args.NetworkType,
		CIDR: args.CIDR, Region: args.Region, Environment: args.Environment, Description: args.Description,
	})
}

type SetNetworkStatusArgs struct {
	NetworkID       int64  `json:"network_id" validate:"required,gt=0"`
	Status          string `json:"status" validate:"required,oneof=active disabled"`
	ExpectedVersion int64  `json:"expected_version" validate:"required,gt=0"`
}

type MutationReply struct {
	Success bool `json:"success"`
}

func (h *Handler) SetNetworkStatus(ctx *jrpc.Context, args *SetNetworkStatusArgs) (*MutationReply, error) {
	if args == nil {
		return nil, errors.New("IPAM network status arguments are required")
	}
	if err := h.service.SetNetworkStatus(middleware.ServiceContext(ctx), args.NetworkID, args.Status, args.ExpectedVersion); err != nil {
		return nil, err
	}
	return &MutationReply{Success: true}, nil
}
