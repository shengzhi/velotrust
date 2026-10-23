package policy

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/netip"
	"regexp"
	"strconv"
	"strings"

	servicecontext "velotrust/management/internal/context"
	model "velotrust/management/internal/model/ztna"
	appErrors "velotrust/pkg/errors"
)

const (
	readPermission          = "ztna.policy.read"
	writePermission         = "ztna.policy.write"
	resourceReadPermission  = "ztna.resource.read"
	resourceWritePermission = "ztna.resource.write"
)

var policyCodePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$`)
var policyRuleIDPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$`)

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

type PermissionChecker interface {
	HasPermission(context.Context, int64, string) (bool, error)
}

type Service struct {
	store       Store
	permissions PermissionChecker
}

func New(store Store, permissions PermissionChecker) (*Service, error) {
	if store == nil || permissions == nil {
		return nil, errors.New("access-policy store and IAM permission checker are required")
	}
	return &Service{store: store, permissions: permissions}, nil
}

func (s *Service) ListPolicies(ctx *servicecontext.Context) ([]model.AccessPolicyView, error) {
	if _, err := s.authorize(ctx, readPermission); err != nil {
		return nil, err
	}
	policies, err := s.store.ListAccessPolicies(ctx)
	if err != nil {
		return nil, err
	}
	items := make([]model.AccessPolicyView, 0, len(policies))
	for _, policy := range policies {
		data, err := s.store.GetAccessPolicy(ctx, policy.ID)
		if err != nil {
			return nil, fmt.Errorf("load access policy %d relationships: %w", policy.ID, err)
		}
		var subjects []model.PolicySubjectRef
		var resourceIDs []int64
		if data != nil {
			subjects, resourceIDs = data.Subjects, data.ResourceIDs
		}
		item, err := policyView(policy, subjects, resourceIDs)
		if err != nil {
			return nil, fmt.Errorf("decode access policy %d: %w", policy.ID, err)
		}
		items = append(items, *item)
	}
	return items, nil
}

func (s *Service) ListGateways(ctx *servicecontext.Context) ([]model.PolicyGatewayOption, error) {
	if _, err := s.authorize(ctx, readPermission); err != nil {
		return nil, err
	}
	items, err := s.store.ListPolicyGateways(ctx)
	if err != nil {
		return nil, err
	}
	for _, item := range items {
		if item.ID <= 0 {
			return nil, errors.New("Gateway option has an invalid node ID")
		}
	}
	return items, nil
}

func (s *Service) ListPolicyGroups(ctx *servicecontext.Context) ([]model.PolicyGroupOption, error) {
	if _, err := s.authorize(ctx, readPermission); err != nil {
		return nil, err
	}
	items, err := s.store.ListPolicyGroups(ctx)
	if err != nil {
		return nil, err
	}
	for _, item := range items {
		if item.ID <= 0 {
			return nil, errors.New("security group option has an invalid group ID")
		}
	}
	return items, nil
}

func (s *Service) ListPolicyResources(ctx *servicecontext.Context) ([]model.PolicyResourceOption, error) {
	if _, err := s.authorize(ctx, readPermission); err != nil {
		return nil, err
	}
	resources, err := s.store.ListProtectedResources(ctx)
	if err != nil {
		return nil, err
	}
	items := make([]model.PolicyResourceOption, 0, len(resources))
	for _, resource := range resources {
		// Only expose resources that can currently produce a safe Gateway ACL:
		// they need an IPv4 target, an active site, an assigned Gateway, and a
		// complete port range. Any port is represented explicitly as 1-65535.
		portRange := resource.PortStart >= 1 && resource.PortEnd >= resource.PortStart && resource.PortEnd <= 65535
		if resource.Status != "active" || resource.NetworkID <= 0 || resource.GatewayNodeID <= 0 ||
			strings.TrimSpace(resource.Address) == "" || (resource.Protocol != "tcp" && resource.Protocol != "udp") ||
			!portRange {
			continue
		}
		item := model.PolicyResourceOption{
			ID: resource.ID, VersionNo: resource.VersionNo, Name: resource.ResourceName, Code: resource.ResourceCode,
			Address: optionalValue(resource.Address), NetworkID: int64Pointer(resource.NetworkID),
			NetworkName: optionalValue(resource.NetworkName), NetworkCIDR: optionalValue(resource.NetworkCIDR),
			GatewayNodeID: int64Pointer(resource.GatewayNodeID), GatewayName: optionalValue(resource.GatewayName),
			Protocol: optionalValue(resource.Protocol), PortStart: intPointer(resource.PortStart), PortEnd: intPointer(resource.PortEnd),
			Status: resource.Status, Description: resource.Description, Category: resource.Category,
		}
		items = append(items, item)
	}
	return items, nil
}

func (s *Service) SavePolicy(ctx *servicecontext.Context, input model.AccessPolicyView) (*model.AccessPolicyView, error) {
	if _, err := s.authorize(ctx, writePermission); err != nil {
		return nil, err
	}
	if input.ID < 0 || input.VersionNo < 0 || (input.ID == 0 && input.VersionNo != 0) || (input.ID > 0 && input.VersionNo <= 0) {
		return nil, invalid("id", "策略 ID 与版本号无效")
	}
	if input.PolicyName = strings.TrimSpace(input.PolicyName); input.PolicyName == "" || len(input.PolicyName) > 128 {
		return nil, invalid("policy_name", "策略名称必填且不能超过 128 个字符")
	}
	if input.PolicyCode = strings.TrimSpace(input.PolicyCode); !policyCodePattern.MatchString(input.PolicyCode) {
		return nil, invalid("policy_code", "策略编码只能包含字母、数字、点、下划线和中划线")
	}
	if input.Description != nil {
		value := strings.TrimSpace(*input.Description)
		if len(value) > 512 {
			return nil, invalid("description", "说明不能超过 512 个字符")
		}
		input.Description = &value
	}
	if input.Priority < 0 || input.Priority > 1_000_000 {
		return nil, invalid("priority", "策略优先级必须在 0 到 1000000 之间")
	}
	if len(input.Rules) != 1 {
		return nil, invalid("rules", "每条策略必须明确设置一个授权结果；策略优先级用于处理多条策略之间的冲突")
	}
	switch input.Status {
	case "draft", "active", "disabled":
	default:
		return nil, invalid("status", "策略状态必须是 draft、active 或 disabled")
	}
	if err := normalizeAndValidateRules(input.Rules); err != nil {
		return nil, invalid("rules", err.Error())
	}
	if input.Status == "active" && (len(input.Rules) == 0 || len(input.Subjects) == 0 || len(input.ResourceIDs) == 0) {
		return nil, invalid("policy", "启用策略必须至少关联一条授权动作、一个安全用户组和一个有效企业资源")
	}
	for i := range input.Subjects {
		input.Subjects[i].Type = strings.TrimSpace(input.Subjects[i].Type)
		if input.Subjects[i].Type != "group" {
			return nil, invalid("subjects", "访问策略目前只支持关联安全用户组")
		}
	}
	rulesJSON, err := json.Marshal(input.Rules)
	if err != nil {
		return nil, fmt.Errorf("encode access policy rules: %w", err)
	}
	policy := &model.AccessPolicy{
		PolicyName: input.PolicyName, PolicyCode: input.PolicyCode, Description: input.Description,
		Priority: input.Priority, Status: input.Status, RulesJSON: rulesJSON,
	}
	policy.ID = input.ID
	policy.VersionNo = input.VersionNo
	if err := s.store.SaveAccessPolicy(ctx, policy, input.Subjects, input.ResourceIDs); err != nil {
		return nil, err
	}
	return policyView(*policy, input.Subjects, input.ResourceIDs)
}

func (s *Service) ListProtectedResources(ctx *servicecontext.Context) ([]model.ProtectedResourceView, error) {
	if _, err := s.authorize(ctx, resourceReadPermission); err != nil {
		return nil, err
	}
	return s.store.ListProtectedResources(ctx)
}

func (s *Service) SaveProtectedResource(ctx *servicecontext.Context, input model.ProtectedResourceView) (*model.ProtectedResourceView, error) {
	if _, err := s.authorize(ctx, resourceWritePermission); err != nil {
		return nil, err
	}
	if input.ID < 0 || input.VersionNo < 0 || (input.ID == 0 && input.VersionNo != 0) || (input.ID > 0 && input.VersionNo <= 0) {
		return nil, invalid("id", "企业资源 ID 与版本号无效")
	}
	input.ResourceName = strings.TrimSpace(input.ResourceName)
	input.ResourceCode = strings.TrimSpace(input.ResourceCode)
	input.ResourceType = strings.TrimSpace(input.ResourceType)
	input.Address = strings.TrimSpace(input.Address)
	input.Protocol = strings.ToLower(strings.TrimSpace(input.Protocol))
	input.AccessMode = strings.TrimSpace(input.AccessMode)
	input.Status = strings.TrimSpace(input.Status)
	if input.ResourceName == "" || len(input.ResourceName) > 128 {
		return nil, invalid("resource_name", "资源名称必填且不能超过 128 个字符")
	}
	if !policyCodePattern.MatchString(input.ResourceCode) {
		return nil, invalid("resource_code", "资源编码只能包含字母、数字、点、下划线和中划线")
	}
	if input.ResourceType == "" || len(input.ResourceType) > 32 || input.NetworkID <= 0 {
		return nil, invalid("resource_type", "资源分类和所属网络站点必填")
	}
	if input.Address == "" {
		return nil, invalid("address", "必须填写 IP 地址或 CIDR；HUB 需要据此生成网关 ACL")
	}
	if prefix, err := netip.ParsePrefix(input.Address); err == nil && prefix.Addr().Is4() {
		input.Address = prefix.Masked().String()
	} else if address, addressErr := netip.ParseAddr(input.Address); addressErr == nil && address.Is4() {
		bits := 128
		if address.Is4() {
			bits = 32
		}
		input.Address = netip.PrefixFrom(address, bits).String()
	} else {
		return nil, invalid("address", "当前 Overlay 仅支持 IPv4，必须填写 IPv4 地址或 CIDR，不能填写 IPv6 或域名")
	}
	if input.Protocol != "tcp" && input.Protocol != "udp" {
		return nil, invalid("protocol", "网络 ACL 目前只支持 TCP 或 UDP 服务")
	}
	if input.PortStart < 1 || input.PortEnd < input.PortStart || input.PortEnd > 65535 {
		return nil, invalid("port", "端口范围必须在 1 到 65535 之间；任意端口对应完整范围 1 到 65535")
	}
	if input.AccessMode == "" {
		input.AccessMode = "network"
	}
	if input.Status != "active" && input.Status != "disabled" && input.Status != "unreachable" {
		return nil, invalid("status", "资源状态无效")
	}
	if input.Description != nil {
		value := strings.TrimSpace(*input.Description)
		if len(value) > 512 {
			return nil, invalid("description", "说明不能超过 512 个字符")
		}
		input.Description = &value
	}
	tagsJSON, err := json.Marshal(input.Tags)
	if err != nil {
		return nil, fmt.Errorf("encode resource tags: %w", err)
	}
	resource := &model.ProtectedResource{
		ResourceName: input.ResourceName, ResourceCode: input.ResourceCode, ResourceType: input.ResourceType,
		NetworkID: &input.NetworkID, Host: input.Host, Address: &input.Address,
		PortStart: &input.PortStart, PortEnd: &input.PortEnd, Protocol: &input.Protocol,
		AccessMode: input.AccessMode, Category: input.Category, TagsJSON: tagsJSON,
		Description: input.Description, Status: input.Status,
	}
	resource.ID, resource.VersionNo = input.ID, input.VersionNo
	if err := s.store.SaveProtectedResource(ctx, resource, input.VersionNo); err != nil {
		return nil, err
	}
	items, err := s.store.ListProtectedResources(ctx)
	if err != nil {
		return nil, err
	}
	for i := range items {
		if items[i].ID == resource.ID {
			return &items[i], nil
		}
	}
	return nil, errors.New("saved enterprise resource could not be reloaded")
}

func (s *Service) DeleteProtectedResource(ctx *servicecontext.Context, id, expectedVersion int64) error {
	if _, err := s.authorize(ctx, resourceWritePermission); err != nil {
		return err
	}
	if id <= 0 || expectedVersion <= 0 {
		return invalid("id", "资源 ID 与版本号必须大于零")
	}
	return s.store.DeleteProtectedResource(ctx, id, expectedVersion)
}

func (s *Service) SetPolicyStatus(ctx *servicecontext.Context, id, expectedVersion int64, status string) error {
	if _, err := s.authorize(ctx, writePermission); err != nil {
		return err
	}
	if id <= 0 || expectedVersion <= 0 {
		return invalid("id", "策略 ID 和版本号必须大于零")
	}
	if status != "active" && status != "disabled" && status != "draft" {
		return invalid("status", "策略状态必须是 draft、active 或 disabled")
	}
	data, err := s.store.GetAccessPolicy(ctx, id)
	if err != nil {
		return err
	}
	if data == nil {
		return appErrors.NewResourceError("ZTNA_POLICY_NOT_FOUND", "access policy", true)
	}
	if data.Policy.VersionNo != expectedVersion {
		return appErrors.NewClientError("ZTNA_POLICY_VERSION_CONFLICT", map[string]string{"version_no": "refresh_and_retry"})
	}
	view, err := policyView(data.Policy, data.Subjects, data.ResourceIDs)
	if err != nil {
		return err
	}
	view.Status = status
	_, err = s.SavePolicy(ctx, *view)
	return err
}

func (s *Service) DeletePolicy(ctx *servicecontext.Context, id, expectedVersion int64) error {
	if _, err := s.authorize(ctx, writePermission); err != nil {
		return err
	}
	if id <= 0 || expectedVersion <= 0 {
		return invalid("id", "策略 ID 和版本号必须大于零")
	}
	return s.store.DeleteAccessPolicy(ctx, id, expectedVersion)
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
		return 0, fmt.Errorf("check access-policy permission: %w", err)
	}
	if !allowed {
		return 0, appErrors.NewAuthError("IAM_PERMISSION_DENIED", appErrors.AuthPermissionDenied)
	}
	return actorID, nil
}

func policyView(policy model.AccessPolicy, subjects []model.PolicySubjectRef, resourceIDs []int64) (*model.AccessPolicyView, error) {
	rules := make([]model.AccessPolicyRule, 0)
	if len(policy.RulesJSON) > 0 {
		if err := json.Unmarshal(policy.RulesJSON, &rules); err != nil {
			return nil, fmt.Errorf("decode ACL rules: %w", err)
		}
	}
	return &model.AccessPolicyView{
		ID: policy.ID, VersionNo: policy.VersionNo, PolicyName: policy.PolicyName, PolicyCode: policy.PolicyCode,
		Description: policy.Description, Priority: policy.Priority, Status: policy.Status,
		Rules: rules, Subjects: append([]model.PolicySubjectRef(nil), subjects...),
		ResourceIDs: append([]int64(nil), resourceIDs...),
	}, nil
}

func normalizeAndValidateRules(rules []model.AccessPolicyRule) error {
	if len(rules) != 1 {
		return errors.New("一条策略必须只有一个授权动作")
	}
	seenIDs := make(map[string]struct{}, len(rules))
	seenPriorities := make(map[uint32]struct{}, len(rules))
	for index := range rules {
		rule := &rules[index]
		rule.ID = strings.TrimSpace(rule.ID)
		if !policyRuleIDPattern.MatchString(rule.ID) {
			return fmt.Errorf("第 %d 条规则的 ID 格式无效", index+1)
		}
		if _, exists := seenIDs[rule.ID]; exists {
			return fmt.Errorf("规则 ID %q 重复", rule.ID)
		}
		seenIDs[rule.ID] = struct{}{}
		if _, exists := seenPriorities[rule.Priority]; exists {
			return fmt.Errorf("规则优先级 %d 重复", rule.Priority)
		}
		seenPriorities[rule.Priority] = struct{}{}
		rule.Action = strings.ToLower(strings.TrimSpace(rule.Action))
		if rule.Action != "allow" && rule.Action != "deny" {
			return fmt.Errorf("第 %d 条规则的动作必须是 allow 或 deny", index+1)
		}
	}
	return nil
}

func optionalValue(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}
func int64Pointer(value int64) *int64 {
	if value == 0 {
		return nil
	}
	return &value
}
func intPointer(value int) *int {
	if value == 0 {
		return nil
	}
	return &value
}

func invalid(field, message string) error {
	return appErrors.NewClientError("ZTNA_POLICY_INVALID_ARGUMENT", map[string]string{field: message})
}
