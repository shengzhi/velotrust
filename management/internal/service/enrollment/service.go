package enrollment

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	servicecontext "velotrust/management/internal/context"
	model "velotrust/management/internal/model/enrollment"
	modelIPAM "velotrust/management/internal/model/ipam"
	"velotrust/management/internal/repository/enrollment"
	appErrors "velotrust/pkg/errors"
)

const (
	readPermission  = "gateway.registration.read"
	writePermission = "gateway.registration.write"
)

type PermissionChecker interface {
	HasPermission(context.Context, int64, string) (bool, error)
}

type Service struct {
	store       enrollment.Store
	permissions PermissionChecker
	networks    NetworkCatalog
}

type NetworkCatalog interface {
	ListSelectableSiteNetworks(*servicecontext.Context) ([]modelIPAM.Network, error)
	ResolveActiveSiteNetworks(*servicecontext.Context, []int64) ([]modelIPAM.Network, error)
}

type CreateGatewayInput struct {
	GatewayName       string
	GatewayType       string
	NetworkName       string
	AllowedNetworkIDs []int64
	RequireApproval   bool
	ExpiresInHours    int
	Notes             string
}

func New(store enrollment.Store, permissions PermissionChecker, networks NetworkCatalog) (*Service, error) {
	if store == nil || permissions == nil || networks == nil {
		return nil, errors.New("enrollment store, network-site network catalog, and IAM permission checker are required")
	}
	return &Service{store: store, permissions: permissions, networks: networks}, nil
}

func (s *Service) ListCredentials(ctx *servicecontext.Context) ([]model.Credential, error) {
	if _, err := s.authorize(ctx, readPermission); err != nil {
		return nil, err
	}
	return s.store.ListCredentials(ctx)
}

func (s *Service) ListSelectableSiteNetworks(ctx *servicecontext.Context) ([]modelIPAM.Network, error) {
	if _, err := s.authorize(ctx, readPermission); err != nil {
		return nil, err
	}
	networks, err := s.networks.ListSelectableSiteNetworks(ctx)
	if err != nil {
		return nil, err
	}
	available := make([]modelIPAM.Network, 0, len(networks))
	for _, network := range networks {
		if network.GatewayNodeID == nil {
			available = append(available, network)
		}
	}
	return available, nil
}

func (s *Service) CreateGateway(ctx *servicecontext.Context, input CreateGatewayInput) (*model.CredentialGrant, error) {
	actorID, err := s.authorize(ctx, writePermission)
	if err != nil {
		return nil, err
	}
	input.GatewayName = strings.TrimSpace(input.GatewayName)
	input.GatewayType = strings.TrimSpace(input.GatewayType)
	input.NetworkName = strings.TrimSpace(input.NetworkName)
	input.Notes = strings.TrimSpace(input.Notes)
	if input.GatewayName == "" || len(input.GatewayName) > 128 {
		return nil, invalid("gateway_name", "required and at most 128 characters")
	}
	if !validGatewayType(input.GatewayType) {
		return nil, invalid("gateway_type", "must be one of the supported gateway types")
	}
	if input.NetworkName == "" || len(input.NetworkName) > 128 {
		return nil, invalid("network_name", "required and at most 128 characters")
	}
	if len(input.Notes) > 512 {
		return nil, invalid("notes", "must be at most 512 characters")
	}
	if input.ExpiresInHours == 0 {
		input.ExpiresInHours = 24
	}
	if err := validateExpiry(input.ExpiresInHours); err != nil {
		return nil, err
	}
	if len(input.AllowedNetworkIDs) == 0 || len(input.AllowedNetworkIDs) > 32 {
		return nil, invalid("allowed_network_ids", "select between 1 and 32 active network-site networks")
	}
	selectedNetworks, err := s.networks.ResolveActiveSiteNetworks(ctx, input.AllowedNetworkIDs)
	if err != nil {
		return nil, invalid("allowed_network_ids", err.Error())
	}
	subnets := make([]string, 0, len(selectedNetworks))
	selectedIDs := make([]int64, 0, len(selectedNetworks))
	seenCIDRs := make(map[string]struct{}, len(selectedNetworks))
	for _, network := range selectedNetworks {
		if network.GatewayNodeID != nil {
			return nil, invalid("allowed_network_ids", "selected network site is already assigned to a gateway; unbind it in Sites first")
		}
		canonical := strings.TrimSpace(network.CIDR)
		if canonical == "" {
			return nil, invalid("allowed_network_ids", "selected IPAM network has no CIDR")
		}
		if _, exists := seenCIDRs[canonical]; exists {
			return nil, invalid("allowed_network_ids", "selected IPAM networks contain duplicate CIDR ranges")
		}
		seenCIDRs[canonical] = struct{}{}
		subnets = append(subnets, canonical)
		selectedIDs = append(selectedIDs, network.ID)
	}
	now := time.Now().UTC()
	gatewayID, err := randomHex(16)
	if err != nil {
		return nil, fmt.Errorf("generate gateway identity: %w", err)
	}
	gateway := &model.Gateway{
		GatewayID: "gw_" + gatewayID, GatewayName: input.GatewayName, GatewayType: input.GatewayType,
		NetworkName: input.NetworkName, GatewayNetworkName: input.NetworkName,
		AllowedSubnets: subnets, AllowedNetworkIDs: selectedIDs,
		RequireApproval: input.RequireApproval, Status: model.GatewayPending,
		NodeType: "gateway", NodeStatus: "pending",
	}
	token, hash, err := newCredential()
	if err != nil {
		return nil, fmt.Errorf("generate gateway registration credential: %w", err)
	}
	credential := newCredentialModel(hash, now.Add(time.Duration(input.ExpiresInHours)*time.Hour), input.Notes)
	audit, err := newAudit(ctx, actorID, "gateway.registration.create", "gateway", "gateway and one-time credential created")
	if err != nil {
		return nil, err
	}
	audit.ChangesJSON, _ = json.Marshal(map[string]any{
		"gateway_name": gateway.GatewayName, "gateway_type": gateway.GatewayType,
		"network_name": gateway.NetworkName, "allowed_subnets": subnets,
		"allowed_network_ids": selectedIDs,
		"require_approval":    gateway.RequireApproval, "expires_in_hours": input.ExpiresInHours,
	})
	if err := s.store.CreateGatewayAndCredential(ctx, actorID, gateway, credential, audit); err != nil {
		return nil, err
	}
	return &model.CredentialGrant{Credential: credentialView(gateway, credential), Token: token}, nil
}

func (s *Service) RevokeCredential(ctx *servicecontext.Context, credentialID int64, reason string) error {
	actorID, err := s.authorize(ctx, writePermission)
	if err != nil {
		return err
	}
	if credentialID <= 0 {
		return invalid("credential_id", "must be a positive ID")
	}
	reason = strings.TrimSpace(reason)
	if reason == "" || len(reason) > 512 {
		return invalid("reason", "required and at most 512 characters")
	}
	audit, err := newAudit(ctx, actorID, "gateway.registration.revoke", "gateway_registration_credential", reason)
	if err != nil {
		return err
	}
	audit.ChangesJSON, _ = json.Marshal(map[string]string{"reason": reason})
	return s.store.RevokeCredential(ctx, actorID, credentialID, reason, audit)
}

func (s *Service) ExtendCredential(ctx *servicecontext.Context, credentialID int64, additionalHours int) error {
	actorID, err := s.authorize(ctx, writePermission)
	if err != nil {
		return err
	}
	if credentialID <= 0 {
		return invalid("credential_id", "must be a positive ID")
	}
	if err := validateExpiry(additionalHours); err != nil {
		return err
	}
	audit, err := newAudit(ctx, actorID, "gateway.registration.extend", "gateway_registration_credential", "registration credential expiry extended")
	if err != nil {
		return err
	}
	audit.ChangesJSON, _ = json.Marshal(map[string]int{"additional_hours": additionalHours})
	return s.store.ExtendCredential(ctx, actorID, credentialID, additionalHours, audit)
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
		return 0, fmt.Errorf("check registration management permission: %w", err)
	}
	if !allowed {
		return 0, appErrors.NewAuthError("IAM_PERMISSION_DENIED", appErrors.AuthPermissionDenied)
	}
	return actorID, nil
}

func validateExpiry(hours int) error {
	if hours < 1 || hours > 168 {
		return invalid("expires_in_hours", "must be between 1 and 168")
	}
	return nil
}

func validGatewayType(value string) bool {
	switch value {
	case model.GatewayTypeSite, model.GatewayTypeService, model.GatewayTypeRelay,
		model.GatewayTypeHub, model.GatewayTypeDatacenter, model.GatewayTypeBranch:
		return true
	default:
		return false
	}
}

func newCredential() (string, string, error) {
	secret := make([]byte, 32)
	if _, err := rand.Read(secret); err != nil {
		return "", "", err
	}
	token := "vtreg_" + base64.RawURLEncoding.EncodeToString(secret)
	digest := sha256.Sum256([]byte(token))
	return token, hex.EncodeToString(digest[:]), nil
}

func randomHex(size int) (string, error) {
	value := make([]byte, size)
	if _, err := rand.Read(value); err != nil {
		return "", err
	}
	return hex.EncodeToString(value), nil
}

func newCredentialModel(hash string, expires time.Time, notes string) *model.Credential {
	var note *string
	if notes != "" {
		note = &notes
	}
	return &model.Credential{
		TokenHash: hash, Status: model.StatusPending, MaxUses: 1, ExpiresAt: expires,
		Notes: note,
	}
}

func credentialView(gateway *model.Gateway, credential *model.Credential) model.Credential {
	view := *credential
	view.NodeID = gateway.ID
	view.GatewayID = gateway.GatewayID
	view.GatewayName = gateway.GatewayName
	view.GatewayType = gateway.GatewayType
	view.NetworkName = gateway.NetworkName
	view.AllowedSubnets = append([]string(nil), gateway.AllowedSubnets...)
	view.RequireApproval = gateway.RequireApproval
	view.TokenHash = ""
	return view
}

func newAudit(ctx *servicecontext.Context, actorID int64, action, targetType, reason string) (*model.AuditEvent, error) {
	eventID, err := randomHex(16)
	if err != nil {
		return nil, fmt.Errorf("generate registration audit event ID: %w", err)
	}
	event := &model.AuditEvent{
		EventID: eventID, ActorID: actorID, Action: action, TargetType: targetType,
		Result: "success", Reason: reason, OccurredAt: time.Now().UTC(),
	}
	if ctx != nil {
		event.ActorName, event.SourceIP, event.RequestID = ctx.UserName(), ctx.ClientIP(), ctx.RequestID()
	}
	return event, nil
}

func invalid(field, reason string) error {
	return appErrors.NewClientError("ENROLLMENT_INVALID_ARGUMENT", map[string]string{field: reason})
}
