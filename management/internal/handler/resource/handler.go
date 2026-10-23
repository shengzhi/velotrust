package resource

import (
	"errors"
	"fmt"

	"velotrust/management/internal/middleware"
	model "velotrust/management/internal/model/ztna"
	appPolicy "velotrust/management/internal/service/policy"
	"velotrust/pkg/jrpc"
)

type Handler struct{ service *appPolicy.Service }

var methodPolicies = map[string]middleware.IAMMethodPolicy{
	"ListResources":  {Authenticated: true, Permission: "ztna.resource.read"},
	"SaveResource":   {Authenticated: true, Permission: "ztna.resource.write"},
	"DeleteResource": {Authenticated: true, Permission: "ztna.resource.write"},
}

func Register(server *jrpc.Server, service *appPolicy.Service, checker middleware.PermissionChecker) error {
	if server == nil || service == nil || checker == nil {
		return errors.New("JRPC server, protected-resource service, and permission checker are required")
	}
	if err := server.RegisterServiceWithOptions(&Handler{service: service}, "RESOURCE", jrpc.ServiceOptions{}, middleware.RequireIAMMethodPolicy(checker, methodPolicies)); err != nil {
		return err
	}
	registered := make(map[string]struct{}, len(methodPolicies))
	for _, descriptor := range server.Methods() {
		if descriptor.Service != "RESOURCE" {
			continue
		}
		if _, ok := methodPolicies[descriptor.Method]; !ok {
			return fmt.Errorf("RESOURCE JRPC method %s has no access declaration", descriptor.Method)
		}
		if descriptor.Access != jrpc.AccessAuthenticated {
			return fmt.Errorf("RESOURCE JRPC method %s must require authentication", descriptor.Method)
		}
		registered[descriptor.Method] = struct{}{}
	}
	if len(registered) != len(methodPolicies) {
		return fmt.Errorf("RESOURCE JRPC methods registered=%d access declarations=%d", len(registered), len(methodPolicies))
	}
	return nil
}

type ListResourcesReply struct {
	Items []model.ProtectedResourceView `json:"items"`
}

func (h *Handler) ListResources(ctx *jrpc.Context, _ *struct{}) (*ListResourcesReply, error) {
	items, err := h.service.ListProtectedResources(middleware.ServiceContext(ctx))
	if err != nil {
		return nil, err
	}
	return &ListResourcesReply{Items: items}, nil
}

type SaveResourceArgs struct{ model.ProtectedResourceView }

func (h *Handler) SaveResource(ctx *jrpc.Context, args *SaveResourceArgs) (*model.ProtectedResourceView, error) {
	if args == nil {
		return nil, errors.New("protected-resource arguments are required")
	}
	return h.service.SaveProtectedResource(middleware.ServiceContext(ctx), args.ProtectedResourceView)
}

type DeleteResourceArgs struct {
	ID              int64 `json:"id" validate:"required,gt=0"`
	ExpectedVersion int64 `json:"expected_version" validate:"required,gt=0"`
}

type MutationReply struct {
	Success bool `json:"success"`
}

func (h *Handler) DeleteResource(ctx *jrpc.Context, args *DeleteResourceArgs) (*MutationReply, error) {
	if args == nil {
		return nil, errors.New("protected-resource deletion arguments are required")
	}
	if err := h.service.DeleteProtectedResource(middleware.ServiceContext(ctx), args.ID, args.ExpectedVersion); err != nil {
		return nil, err
	}
	return &MutationReply{Success: true}, nil
}
