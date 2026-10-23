package ztna

import (
	servicecontext "velotrust/management/internal/context"
	model "velotrust/management/internal/model/ztna"
)

type Store interface {
	ListAccessPolicies(*servicecontext.Context) ([]model.AccessPolicy, error)
	GetAccessPolicy(*servicecontext.Context, int64) (*model.AccessPolicyData, error)
	SaveAccessPolicy(*servicecontext.Context, *model.AccessPolicy, []model.PolicySubjectRef, []int64) error
	DeleteAccessPolicy(*servicecontext.Context, int64, int64) error
	ListPolicyGateways(*servicecontext.Context) ([]model.PolicyGatewayOption, error)
	ListPolicyGroups(*servicecontext.Context) ([]model.PolicyGroupOption, error)
	ListProtectedResources(*servicecontext.Context) ([]model.ProtectedResourceView, error)
	SaveProtectedResource(*servicecontext.Context, *model.ProtectedResource, int64) error
	DeleteProtectedResource(*servicecontext.Context, int64, int64) error
}
