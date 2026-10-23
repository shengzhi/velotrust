package site

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"

	"velotrust/management/internal/middleware"
	ipamModel "velotrust/management/internal/model/ipam"
	model "velotrust/management/internal/model/site"
	appSite "velotrust/management/internal/service/site"
	"velotrust/pkg/jrpc"
)

type Handler struct{ service *appSite.Service }

var methodPolicies = map[string]middleware.IAMMethodPolicy{
	"ListSites":      {Authenticated: true, Permission: "ztna.site.read"},
	"ListGateways":   {Authenticated: true, Permission: "ztna.site.read"},
	"SaveSite":       {Authenticated: true, Permission: "ztna.site.write"},
	"SaveNATMapping": {Authenticated: true, Permission: "ztna.site.write"},
	"DeleteSite":     {Authenticated: true, Permission: "ztna.site.write"},
}

func Register(server *jrpc.Server, service *appSite.Service, checker middleware.PermissionChecker) error {
	if server == nil || service == nil || checker == nil {
		return errors.New("JRPC server, network-site service, and permission checker are required")
	}
	if err := server.RegisterServiceWithOptions(&Handler{service: service}, "SITE", jrpc.ServiceOptions{}, middleware.RequireIAMMethodPolicy(checker, methodPolicies)); err != nil {
		return err
	}
	registered := make(map[string]struct{}, len(methodPolicies))
	for _, descriptor := range server.Methods() {
		if descriptor.Service != "SITE" {
			continue
		}
		if _, ok := methodPolicies[descriptor.Method]; !ok {
			return fmt.Errorf("SITE JRPC method %s has no access declaration", descriptor.Method)
		}
		if descriptor.Access != jrpc.AccessAuthenticated {
			return fmt.Errorf("SITE JRPC method %s must require authentication", descriptor.Method)
		}
		registered[descriptor.Method] = struct{}{}
	}
	if len(registered) != len(methodPolicies) {
		return fmt.Errorf("SITE JRPC methods registered=%d access declarations=%d", len(registered), len(methodPolicies))
	}
	return nil
}

type ListSitesReply struct {
	Items []model.PhysicalSite `json:"items"`
}

func (h *Handler) ListSites(ctx *jrpc.Context, _ *struct{}) (*ListSitesReply, error) {
	items, err := h.service.ListSites(middleware.ServiceContext(ctx))
	if err != nil {
		return nil, err
	}
	return &ListSitesReply{Items: items}, nil
}

type ListGatewaysReply struct {
	Items []model.GatewayOption `json:"items"`
}

func (h *Handler) ListGateways(ctx *jrpc.Context, _ *struct{}) (*ListGatewaysReply, error) {
	items, err := h.service.ListGateways(middleware.ServiceContext(ctx))
	if err != nil {
		return nil, err
	}
	return &ListGatewaysReply{Items: items}, nil
}

type SaveSiteArgs struct {
	ID              int64  `json:"id" validate:"omitempty,gt=0"`
	ExpectedVersion int64  `json:"expected_version" validate:"omitempty,gte=0"`
	Name            string `json:"name" validate:"required,max=128"`
	LocalSubnet     string `json:"local_subnet" validate:"required,max=64"`
	GatewayNodeID   *int64 `json:"gateway_node_id" validate:"omitempty,gt=0"`
	Status          string `json:"status" validate:"required,oneof=active disabled"`
}

func (args *SaveSiteArgs) UnmarshalJSON(data []byte) error {
	// A stale client must not silently clear ownership by sending the old field.
	type wireArgs SaveSiteArgs
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	return decoder.Decode((*wireArgs)(args))
}

func (h *Handler) SaveSite(ctx *jrpc.Context, args *SaveSiteArgs) (*model.PhysicalSite, error) {
	if args == nil {
		return nil, errors.New("network-site arguments are required")
	}
	if args.ID == 0 && args.ExpectedVersion != 0 {
		return nil, errors.New("new network site must not specify an expected version")
	}
	site := model.PhysicalSite{
		ID: args.ID, Name: args.Name,
		LocalSubnet:   args.LocalSubnet,
		GatewayNodeID: args.GatewayNodeID, Status: args.Status,
	}
	site.VersionNo = args.ExpectedVersion
	return h.service.SaveSite(middleware.ServiceContext(ctx), site)
}

type DeleteSiteArgs struct {
	ID              int64 `json:"id" validate:"required,gt=0"`
	ExpectedVersion int64 `json:"expected_version" validate:"required,gt=0"`
}

type SaveNATMappingArgs struct {
	ID              int64   `json:"id" validate:"omitempty,gt=0"`
	ExpectedVersion int64   `json:"expected_version" validate:"omitempty,gt=0"`
	NetworkID       int64   `json:"network_id" validate:"required,gt=0"`
	NATType         string  `json:"nat_type" validate:"required,oneof=DNAT_VirtualIP"`
	VirtualCIDR     *string `json:"virtual_cidr" validate:"omitempty,max=64"`
	Enabled         bool    `json:"enabled"`
	Description     *string `json:"description" validate:"omitempty,max=512"`
}

func (h *Handler) SaveNATMapping(ctx *jrpc.Context, args *SaveNATMappingArgs) (*ipamModel.NATMapping, error) {
	if args == nil {
		return nil, errors.New("network-site NAT mapping arguments are required")
	}
	mapping := ipamModel.NATMapping{
		ID: args.ID, NetworkID: args.NetworkID, NATType: args.NATType,
		VirtualCIDR: args.VirtualCIDR, Enabled: args.Enabled, Description: args.Description,
	}
	mapping.VersionNo = args.ExpectedVersion
	return h.service.SaveNATMapping(middleware.ServiceContext(ctx), mapping)
}

type MutationReply struct {
	Success bool `json:"success"`
}

func (h *Handler) DeleteSite(ctx *jrpc.Context, args *DeleteSiteArgs) (*MutationReply, error) {
	if args == nil {
		return nil, errors.New("network-site delete arguments are required")
	}
	if err := h.service.DeleteSite(middleware.ServiceContext(ctx), args.ID, args.ExpectedVersion); err != nil {
		return nil, err
	}
	return &MutationReply{Success: true}, nil
}
