package node

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"

	servicecontext "velotrust/management/internal/context"
	model "velotrust/management/internal/model/ztna"
	"velotrust/management/internal/repository/node"
	appErrors "velotrust/pkg/errors"
)

const (
	readPermission        = "ztna.node.read"
	writePermission       = "ztna.node.write"
	tunnelReadPermission  = "ztna.tunnel.read"
	tunnelWritePermission = "ztna.tunnel.write"
)

type PermissionChecker interface {
	HasPermission(context.Context, int64, string) (bool, error)
}

type Service struct {
	store       node.Store
	permissions PermissionChecker
}

func New(store node.Store, permissions PermissionChecker) (*Service, error) {
	if store == nil || permissions == nil {
		return nil, errors.New("virtual-node store and IAM permission checker are required")
	}
	return &Service{store: store, permissions: permissions}, nil
}

func (s *Service) ListNodes(ctx *servicecontext.Context) ([]model.VirtualNodeView, error) {
	if _, err := s.authorize(ctx, readPermission); err != nil {
		return nil, err
	}
	return s.store.ListNodes(ctx)
}

func (s *Service) GetNode(ctx *servicecontext.Context, id int64) (*model.VirtualNodeView, error) {
	if _, err := s.authorize(ctx, readPermission); err != nil {
		return nil, err
	}
	if id <= 0 {
		return nil, invalid("id", "节点 ID 必须大于零")
	}
	return s.store.GetNode(ctx, id)
}

func (s *Service) GetAccessChangeStatus(ctx *servicecontext.Context, subjectType string, subjectID int64) (*model.AccessChangeStatusView, error) {
	if _, err := s.authorize(ctx, readPermission); err != nil {
		return nil, err
	}
	if subjectType != "user" && subjectType != "device" && subjectType != "node" || subjectID <= 0 {
		return nil, invalid("subject", "撤销状态查询对象无效")
	}
	return s.store.GetAccessChangeStatus(ctx, subjectType, subjectID)
}

func (s *Service) ListGatewayTunnelOptions(ctx *servicecontext.Context) ([]model.GatewayTunnelOption, error) {
	if _, err := s.authorize(ctx, tunnelReadPermission); err != nil {
		return nil, err
	}
	return s.store.ListGatewayTunnelOptions(ctx)
}

func (s *Service) CreateLogicalTunnel(ctx *servicecontext.Context, code, description string) (*model.GatewayTunnelOption, error) {
	actorID, err := s.authorize(ctx, tunnelWritePermission)
	if err != nil {
		return nil, err
	}
	code, err = normalizeTunnelCode(code)
	if err != nil {
		return nil, invalid("tunnel_code", err.Error())
	}
	description = strings.TrimSpace(description)
	if len(description) > 512 {
		return nil, invalid("description", "隧道说明不能超过 512 个字符")
	}
	return s.store.CreateLogicalTunnel(ctx, actorID, code, description)
}

func (s *Service) AssignGatewayTunnel(ctx *servicecontext.Context, nodeID, tunnelID, expectedVersion int64) error {
	actorID, err := s.authorize(ctx, tunnelWritePermission)
	if err != nil {
		return err
	}
	if nodeID <= 0 || tunnelID <= 0 || expectedVersion <= 0 {
		return invalid("gateway_tunnel", "Gateway、隧道和版本号必须有效")
	}
	return s.store.AssignGatewayTunnel(ctx, actorID, nodeID, tunnelID, expectedVersion)
}

func (s *Service) SetNodeStatus(ctx *servicecontext.Context, id int64, status string, expectedVersion int64) error {
	actorID, err := s.authorize(ctx, writePermission)
	if err != nil {
		return err
	}
	if id <= 0 || expectedVersion <= 0 {
		return invalid("id", "节点 ID 和版本号必须有效")
	}
	switch status {
	case "active", "disabled", "quarantined", "revoked":
	default:
		return invalid("node_status", "节点状态必须是 active、disabled、quarantined 或 revoked")
	}
	return s.store.SetNodeStatus(ctx, actorID, id, status, expectedVersion)
}

func (s *Service) SetNodePublicEndpoint(ctx *servicecontext.Context, id int64, endpoint string, expectedVersion int64) error {
	actorID, err := s.authorize(ctx, writePermission)
	if err != nil {
		return err
	}
	if id <= 0 || expectedVersion <= 0 {
		return invalid("id", "节点 ID 和版本号必须有效")
	}
	endpoint, err = normalizePublicEndpoint(endpoint)
	if err != nil {
		return invalid("public_endpoint", err.Error())
	}
	return s.store.SetNodePublicEndpoint(ctx, actorID, id, endpoint, expectedVersion)
}

func normalizePublicEndpoint(endpoint string) (string, error) {
	endpoint = strings.TrimSpace(endpoint)
	if endpoint == "" {
		return "", nil
	}
	host, portText, err := net.SplitHostPort(endpoint)
	if err != nil || host == "" {
		return "", errors.New("公网端点格式应为主机名或 IP 加端口，例如 gateway.example.com:51820；IPv6 请使用 [地址]:端口")
	}
	port, err := strconv.ParseUint(portText, 10, 16)
	if err != nil || port == 0 {
		return "", errors.New("公网端点端口必须是 1 到 65535 之间的数字")
	}
	if net.ParseIP(host) == nil && !validEndpointHostname(host) {
		return "", errors.New("公网端点主机名或 IP 格式无效")
	}
	return net.JoinHostPort(strings.ToLower(host), strconv.FormatUint(port, 10)), nil
}

func normalizeTunnelCode(code string) (string, error) {
	code = strings.ToLower(strings.TrimSpace(code))
	if code == "" || len(code) > 64 {
		return "", errors.New("隧道编码长度必须为 1 到 64 个字符")
	}
	for index, char := range code {
		valid := char >= 'a' && char <= 'z' || char >= '0' && char <= '9' || (index > 0 && (char == '-' || char == '_' || char == '.'))
		if !valid {
			return "", errors.New("隧道编码只能使用小写字母、数字、点、下划线和中划线，并以字母或数字开头")
		}
	}
	return code, nil
}

func validEndpointHostname(host string) bool {
	host = strings.TrimSuffix(host, ".")
	if len(host) == 0 || len(host) > 253 {
		return false
	}
	for _, label := range strings.Split(host, ".") {
		if len(label) == 0 || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return false
		}
		for _, char := range label {
			if !(char >= 'a' && char <= 'z') && !(char >= 'A' && char <= 'Z') && !(char >= '0' && char <= '9') && char != '-' {
				return false
			}
		}
	}
	return true
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
		return 0, fmt.Errorf("check virtual-node permission: %w", err)
	}
	if !allowed {
		return 0, appErrors.NewAuthError("IAM_PERMISSION_DENIED", appErrors.AuthPermissionDenied)
	}
	return actorID, nil
}

func invalid(field, message string) error {
	return appErrors.NewClientError("ZTNA_NODE_INVALID_ARGUMENT", map[string]string{field: message})
}
