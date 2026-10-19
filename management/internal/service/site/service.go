package site

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"net/netip"
	"strconv"
	"strings"

	servicecontext "velotrust/management/internal/context"
	ipamModel "velotrust/management/internal/model/ipam"
	model "velotrust/management/internal/model/site"
	"velotrust/management/internal/repository/site"
	appErrors "velotrust/pkg/errors"
)

const (
	readPermission  = "ztna.site.read"
	writePermission = "ztna.site.write"
)

type PermissionChecker interface {
	HasPermission(context.Context, int64, string) (bool, error)
}

type SiteNATService interface {
	SaveSiteNATMapping(*servicecontext.Context, ipamModel.NATMapping) (*ipamModel.NATMapping, error)
}

type Service struct {
	store       site.Store
	permissions PermissionChecker
	nat         SiteNATService
}

func New(store site.Store, permissions PermissionChecker, nat SiteNATService) (*Service, error) {
	if store == nil || permissions == nil || nat == nil {
		return nil, errors.New("network-site store, NAT service, and IAM permission checker are required")
	}
	return &Service{store: store, permissions: permissions, nat: nat}, nil
}

func (s *Service) ListSites(ctx *servicecontext.Context) ([]model.PhysicalSite, error) {
	if _, err := s.authorize(ctx, readPermission); err != nil {
		return nil, err
	}
	return s.store.ListSites(ctx)
}

func (s *Service) ListGateways(ctx *servicecontext.Context) ([]model.GatewayOption, error) {
	if _, err := s.authorize(ctx, readPermission); err != nil {
		return nil, err
	}
	return s.store.ListGateways(ctx)
}

func (s *Service) SaveSite(ctx *servicecontext.Context, input model.PhysicalSite) (*model.PhysicalSite, error) {
	actorID, err := s.authorize(ctx, writePermission)
	if err != nil {
		return nil, err
	}
	input.Name = strings.TrimSpace(input.Name)
	input.SiteCode = strings.TrimSpace(input.SiteCode)
	input.LocalSubnet = strings.TrimSpace(input.LocalSubnet)
	if input.Name == "" || len(input.Name) > 128 {
		return nil, invalid("name", "站点名称必填且不能超过 128 个字符")
	}
	prefix, err := netip.ParsePrefix(input.LocalSubnet)
	if err != nil || prefix.Bits() == 0 {
		return nil, invalid("local_subnet", "必须是有效的 IPv4 或 IPv6 CIDR，且不能是默认路由")
	}
	input.LocalSubnet = prefix.Masked().String()
	if input.Status != model.StatusActive && input.Status != model.StatusDisabled {
		return nil, invalid("status", "站点状态必须是 active 或 disabled")
	}
	if input.GatewayNodeID != nil && *input.GatewayNodeID <= 0 {
		input.GatewayNodeID = nil
	}
	if input.ID == 0 {
		input.SiteCode, err = newSiteCode()
		if err != nil {
			return nil, err
		}
	} else if input.VersionNo <= 0 {
		return nil, invalid("expected_version", "编辑站点时必须提供有效版本号")
	}
	if err := s.store.SaveSite(ctx, actorID, &input); err != nil {
		return nil, err
	}
	return &input, nil
}

func (s *Service) DeleteSite(ctx *servicecontext.Context, siteID, expectedVersion int64) error {
	actorID, err := s.authorize(ctx, writePermission)
	if err != nil {
		return err
	}
	if siteID <= 0 || expectedVersion <= 0 {
		return invalid("site_id", "站点 ID 和版本号必须有效")
	}
	return s.store.DeleteSite(ctx, actorID, siteID, expectedVersion)
}

func (s *Service) SaveNATMapping(ctx *servicecontext.Context, mapping ipamModel.NATMapping) (*ipamModel.NATMapping, error) {
	if _, err := s.authorize(ctx, writePermission); err != nil {
		return nil, err
	}
	return s.nat.SaveSiteNATMapping(ctx, mapping)
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
		return 0, fmt.Errorf("check network-site permission: %w", err)
	}
	if !allowed {
		return 0, appErrors.NewAuthError("IAM_PERMISSION_DENIED", appErrors.AuthPermissionDenied)
	}
	return actorID, nil
}

func newSiteCode() (string, error) {
	var random [8]byte
	if _, err := rand.Read(random[:]); err != nil {
		return "", fmt.Errorf("generate network-site code: %w", err)
	}
	return "site-" + hex.EncodeToString(random[:]), nil
}

func invalid(field, message string) error {
	return appErrors.NewClientError("SITE_INVALID_ARGUMENT", map[string]string{field: message})
}
