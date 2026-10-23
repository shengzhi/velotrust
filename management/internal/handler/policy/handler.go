package policy

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
	"ListPolicies":    {Authenticated: true, Permission: "ztna.policy.read"},
	"ListGateways":    {Authenticated: true, Permission: "ztna.policy.read"},
	"ListGroups":      {Authenticated: true, Permission: "ztna.policy.read"},
	"ListResources":   {Authenticated: true, Permission: "ztna.policy.read"},
	"SavePolicy":      {Authenticated: true, Permission: "ztna.policy.write"},
	"SetPolicyStatus": {Authenticated: true, Permission: "ztna.policy.write"},
	"DeletePolicy":    {Authenticated: true, Permission: "ztna.policy.write"},
}

func Register(server *jrpc.Server, service *appPolicy.Service, checker middleware.PermissionChecker) error {
	if server == nil || service == nil || checker == nil {
		return errors.New("JRPC server, access-policy service, and permission checker are required")
	}
	if err := server.RegisterServiceWithOptions(&Handler{service: service}, "POLICY", jrpc.ServiceOptions{}, middleware.RequireIAMMethodPolicy(checker, methodPolicies)); err != nil {
		return err
	}
	registered := make(map[string]struct{}, len(methodPolicies))
	for _, descriptor := range server.Methods() {
		if descriptor.Service != "POLICY" {
			continue
		}
		if _, ok := methodPolicies[descriptor.Method]; !ok {
			return fmt.Errorf("POLICY JRPC method %s has no access declaration", descriptor.Method)
		}
		if descriptor.Access != jrpc.AccessAuthenticated {
			return fmt.Errorf("POLICY JRPC method %s must require authentication", descriptor.Method)
		}
		registered[descriptor.Method] = struct{}{}
	}
	if len(registered) != len(methodPolicies) {
		return fmt.Errorf("POLICY JRPC methods registered=%d access declarations=%d", len(registered), len(methodPolicies))
	}
	return nil
}

type ListPoliciesReply struct {
	Items []model.AccessPolicyView `json:"items"`
}

func (h *Handler) ListPolicies(ctx *jrpc.Context, _ *struct{}) (*ListPoliciesReply, error) {
	items, err := h.service.ListPolicies(middleware.ServiceContext(ctx))
	if err != nil {
		return nil, err
	}
	return &ListPoliciesReply{Items: items}, nil
}

type ListGatewaysReply struct {
	Items []model.PolicyGatewayOption `json:"items"`
}

func (h *Handler) ListGateways(ctx *jrpc.Context, _ *struct{}) (*ListGatewaysReply, error) {
	items, err := h.service.ListGateways(middleware.ServiceContext(ctx))
	if err != nil {
		return nil, err
	}
	return &ListGatewaysReply{Items: items}, nil
}

type ListGroupsReply struct {
	Items []model.PolicyGroupOption `json:"items"`
}

func (h *Handler) ListGroups(ctx *jrpc.Context, _ *struct{}) (*ListGroupsReply, error) {
	items, err := h.service.ListPolicyGroups(middleware.ServiceContext(ctx))
	if err != nil {
		return nil, err
	}
	return &ListGroupsReply{Items: items}, nil
}

type ListResourcesReply struct {
	Items []model.PolicyResourceOption `json:"items"`
}

func (h *Handler) ListResources(ctx *jrpc.Context, _ *struct{}) (*ListResourcesReply, error) {
	items, err := h.service.ListPolicyResources(middleware.ServiceContext(ctx))
	if err != nil {
		return nil, err
	}
	return &ListResourcesReply{Items: items}, nil
}

type SavePolicyArgs struct {
	ID          int64                    `json:"id"`
	VersionNo   int64                    `json:"version_no"`
	PolicyName  string                   `json:"policy_name"`
	PolicyCode  string                   `json:"policy_code"`
	Description *string                  `json:"description"`
	Priority    int                      `json:"priority"`
	Status      string                   `json:"status"`
	Rules       []model.AccessPolicyRule `json:"rules"`
	Subjects    []model.PolicySubjectRef `json:"subjects"`
	ResourceIDs []int64                  `json:"resource_ids"`
}

func (h *Handler) SavePolicy(ctx *jrpc.Context, args *SavePolicyArgs) (*model.AccessPolicyView, error) {
	if args == nil {
		return nil, errors.New("access-policy arguments are required")
	}
	return h.service.SavePolicy(middleware.ServiceContext(ctx), model.AccessPolicyView{
		ID: args.ID, VersionNo: args.VersionNo, PolicyName: args.PolicyName, PolicyCode: args.PolicyCode,
		Description: args.Description, Priority: args.Priority, Status: args.Status,
		Rules: args.Rules, Subjects: args.Subjects, ResourceIDs: args.ResourceIDs,
	})
}

type SetPolicyStatusArgs struct {
	ID              int64  `json:"id" validate:"required,gt=0"`
	ExpectedVersion int64  `json:"expected_version" validate:"required,gt=0"`
	Status          string `json:"status" validate:"required,oneof=draft active disabled"`
}

type MutationReply struct {
	Success bool `json:"success"`
}

func (h *Handler) SetPolicyStatus(ctx *jrpc.Context, args *SetPolicyStatusArgs) (*MutationReply, error) {
	if args == nil {
		return nil, errors.New("access-policy status arguments are required")
	}
	if err := h.service.SetPolicyStatus(middleware.ServiceContext(ctx), args.ID, args.ExpectedVersion, args.Status); err != nil {
		return nil, err
	}
	return &MutationReply{Success: true}, nil
}

type DeletePolicyArgs struct {
	ID              int64 `json:"id" validate:"required,gt=0"`
	ExpectedVersion int64 `json:"expected_version" validate:"required,gt=0"`
}

func (h *Handler) DeletePolicy(ctx *jrpc.Context, args *DeletePolicyArgs) (*MutationReply, error) {
	if args == nil {
		return nil, errors.New("access-policy deletion arguments are required")
	}
	if err := h.service.DeletePolicy(middleware.ServiceContext(ctx), args.ID, args.ExpectedVersion); err != nil {
		return nil, err
	}
	return &MutationReply{Success: true}, nil
}
