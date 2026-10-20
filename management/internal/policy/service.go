// Package policy implements access-policy management over the current
// policy row and its current subject/resource relationships.
package policy

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	servicecontext "velotrust/management/internal/context"
	model "velotrust/management/internal/model/ztna"
)

const (
	readPermission  = "ztna.policy.read"
	writePermission = "ztna.policy.write"
)

type Store interface {
	SaveAccessPolicy(*servicecontext.Context, *model.AccessPolicy, []model.PolicySubjectRef, []int64) error
	GetAccessPolicy(*servicecontext.Context, int64) (*model.AccessPolicyData, error)
	ListAccessPolicies(*servicecontext.Context) ([]model.AccessPolicy, error)
}

type PermissionChecker interface {
	HasPermission(context.Context, int64, string) (bool, error)
}

type Service struct {
	store       Store
	permissions PermissionChecker
}

func New(store Store, permissions PermissionChecker) (*Service, error) {
	if store == nil || permissions == nil {
		return nil, errors.New("policy store and IAM permission checker are required")
	}
	return &Service{store: store, permissions: permissions}, nil
}

// SavePolicy replaces the current policy and its relationship sets atomically.
// The returned version_no is only the shared optimistic-concurrency marker.
func (s *Service) SavePolicy(ctx *servicecontext.Context, policy *model.AccessPolicy, subjects []model.PolicySubjectRef, resourceIDs []int64) error {
	if _, err := s.authorize(ctx, writePermission); err != nil {
		return err
	}
	return s.store.SaveAccessPolicy(ctx, policy, subjects, resourceIDs)
}

func (s *Service) GetPolicy(ctx *servicecontext.Context, policyID int64) (*model.AccessPolicyData, error) {
	if _, err := s.authorize(ctx, readPermission); err != nil {
		return nil, err
	}
	if policyID <= 0 {
		return nil, errors.New("policy ID must be positive")
	}
	return s.store.GetAccessPolicy(ctx, policyID)
}

func (s *Service) ListPolicies(ctx *servicecontext.Context) ([]model.AccessPolicy, error) {
	if _, err := s.authorize(ctx, readPermission); err != nil {
		return nil, err
	}
	return s.store.ListAccessPolicies(ctx)
}

func (s *Service) authorize(ctx *servicecontext.Context, permission string) (int64, error) {
	if ctx == nil {
		return 0, errors.New("authenticated service context is required")
	}
	userID, err := strconv.ParseInt(ctx.UserID(), 10, 64)
	if err != nil || userID <= 0 {
		return 0, errors.New("authenticated IAM user is required")
	}
	allowed, err := s.permissions.HasPermission(ctx, userID, permission)
	if err != nil {
		return 0, fmt.Errorf("check access-policy permission: %w", err)
	}
	if !allowed {
		return 0, fmt.Errorf("IAM permission %q is required", permission)
	}
	return userID, nil
}
