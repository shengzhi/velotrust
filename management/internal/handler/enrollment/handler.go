package enrollment

import (
	"errors"
	"fmt"

	"velotrust/management/internal/middleware"
	model "velotrust/management/internal/model/enrollment"
	modelIPAM "velotrust/management/internal/model/ipam"
	appEnrollment "velotrust/management/internal/service/enrollment"
	"velotrust/pkg/jrpc"
)

type Handler struct{ service *appEnrollment.Service }

var methodPolicies = map[string]middleware.IAMMethodPolicy{
	"ListCredentials":            {Authenticated: true, Permission: "gateway.registration.read"},
	"ListSelectableSiteNetworks": {Authenticated: true, Permission: "gateway.registration.read"},
	"CreateGateway":              {Authenticated: true, Permission: "gateway.registration.write"},
	"RevokeCredential":           {Authenticated: true, Permission: "gateway.registration.write"},
	"ExtendCredential":           {Authenticated: true, Permission: "gateway.registration.write"},
}

func Register(server *jrpc.Server, service *appEnrollment.Service, checker middleware.PermissionChecker) error {
	if server == nil || service == nil || checker == nil {
		return errors.New("JRPC server, enrollment service, and permission checker are required")
	}
	handler := &Handler{service: service}
	if err := server.RegisterServiceWithOptions(handler, "ENROLLMENT", jrpc.ServiceOptions{}, middleware.RequireIAMMethodPolicy(checker, methodPolicies)); err != nil {
		return err
	}
	registered := make(map[string]struct{}, len(methodPolicies))
	for _, descriptor := range server.Methods() {
		if descriptor.Service != "ENROLLMENT" {
			continue
		}
		if _, ok := methodPolicies[descriptor.Method]; !ok {
			return fmt.Errorf("ENROLLMENT JRPC method %s has no access declaration", descriptor.Method)
		}
		if descriptor.Access != jrpc.AccessAuthenticated {
			return fmt.Errorf("ENROLLMENT JRPC method %s must require authentication", descriptor.Method)
		}
		registered[descriptor.Method] = struct{}{}
	}
	if len(registered) != len(methodPolicies) {
		return fmt.Errorf("ENROLLMENT JRPC methods registered=%d access declarations=%d", len(registered), len(methodPolicies))
	}
	return nil
}

type ListCredentialsReply struct {
	Items []model.Credential `json:"items"`
}

type ListSelectableSiteNetworksReply struct {
	Items []modelIPAM.Network `json:"items"`
}

func (h *Handler) ListSelectableSiteNetworks(ctx *jrpc.Context, _ *struct{}) (*ListSelectableSiteNetworksReply, error) {
	items, err := h.service.ListSelectableSiteNetworks(middleware.ServiceContext(ctx))
	if err != nil {
		return nil, err
	}
	return &ListSelectableSiteNetworksReply{Items: items}, nil
}

func (h *Handler) ListCredentials(ctx *jrpc.Context, _ *struct{}) (*ListCredentialsReply, error) {
	items, err := h.service.ListCredentials(middleware.ServiceContext(ctx))
	if err != nil {
		return nil, err
	}
	return &ListCredentialsReply{Items: items}, nil
}

type CreateGatewayArgs struct {
	GatewayName       string  `json:"gateway_name" validate:"required,max=128"`
	GatewayType       string  `json:"gateway_type" validate:"required,max=64"`
	NetworkName       string  `json:"network_name" validate:"required,max=128"`
	AllowedNetworkIDs []int64 `json:"allowed_network_ids" validate:"required,min=1,max=32,dive,gt=0"`
	RequireApproval   bool    `json:"require_approval"`
	ExpiresInHours    int     `json:"expires_in_hours" validate:"required,min=1,max=168"`
	Notes             string  `json:"notes" validate:"omitempty,max=512"`
}

func (h *Handler) CreateGateway(ctx *jrpc.Context, args *CreateGatewayArgs) (*model.CredentialGrant, error) {
	if args == nil {
		return nil, errors.New("gateway registration arguments are required")
	}
	return h.service.CreateGateway(middleware.ServiceContext(ctx), appEnrollment.CreateGatewayInput{
		GatewayName: args.GatewayName, GatewayType: args.GatewayType, NetworkName: args.NetworkName,
		AllowedNetworkIDs: args.AllowedNetworkIDs, RequireApproval: args.RequireApproval,
		ExpiresInHours: args.ExpiresInHours, Notes: args.Notes,
	})
}

type RevokeCredentialArgs struct {
	CredentialID int64  `json:"credential_id" validate:"required,gt=0"`
	Reason       string `json:"reason" validate:"required,max=512"`
}

type MutationReply struct {
	Success bool `json:"success"`
}

func (h *Handler) RevokeCredential(ctx *jrpc.Context, args *RevokeCredentialArgs) (*MutationReply, error) {
	if args == nil {
		return nil, errors.New("credential revocation arguments are required")
	}
	if err := h.service.RevokeCredential(middleware.ServiceContext(ctx), args.CredentialID, args.Reason); err != nil {
		return nil, err
	}
	return &MutationReply{Success: true}, nil
}

type ExtendCredentialArgs struct {
	CredentialID    int64 `json:"credential_id" validate:"required,gt=0"`
	AdditionalHours int   `json:"additional_hours" validate:"required,min=1,max=168"`
}

func (h *Handler) ExtendCredential(ctx *jrpc.Context, args *ExtendCredentialArgs) (*MutationReply, error) {
	if args == nil {
		return nil, errors.New("credential extension arguments are required")
	}
	if err := h.service.ExtendCredential(middleware.ServiceContext(ctx), args.CredentialID, args.AdditionalHours); err != nil {
		return nil, err
	}
	return &MutationReply{Success: true}, nil
}
