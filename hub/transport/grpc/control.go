package grpc

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"crypto/tls"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"strings"
	"time"

	"velotrust/hub/internal/auth"
	"velotrust/hub/internal/configsync"
	"velotrust/hub/internal/report"
	"velotrust/hub/internal/session"
	"velotrust/pkg/hubauth"
	hubv1 "velotrust/shared/gen/go/hub/v1"

	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"
)

// ControlHandler implements the Gateway authentication-first gRPC control
// stream. Runtime status and configuration receipts are accepted only after
// authentication and bound to the active Gateway identity.
type ControlHandler struct {
	hubv1.UnimplementedHubControlServer
	instanceID          string
	plaintextAllowed    bool
	authenticationTTL   time.Duration
	heartbeatInterval   time.Duration
	sessionLeaseTTL     time.Duration
	authenticator       *auth.GatewayAuthenticator
	registrations       GatewayRegistrationRepository
	clientAuth          *auth.ClientAuthenticator
	clientRegistrations ClientRegistrationRepository
	sessions            session.Store
	heartbeats          report.HeartbeatRecorder
	reporter            report.Reporter
	configurations      configsync.Source
	activeStreams       *session.ActiveRegistry
}

type GatewayRegistrationRepository interface {
	CheckCredential(context.Context, string) (auth.GatewayIdentity, error)
	RegisterGateway(context.Context, string, string, []byte, string, time.Time) (auth.GatewayIdentity, bool, error)
	RegistrationState(context.Context, string) (auth.GatewayIdentity, error)
}

type ClientRegistrationRepository interface {
	LoginClient(context.Context, string, []byte, []byte, string, string, string, string, time.Time) (auth.ClientIdentity, bool, error)
}

type ClientControlDependencies struct {
	Authenticator *auth.ClientAuthenticator
	Registrations ClientRegistrationRepository
}

var errRegistrationCredentialNotApplicable = errors.New("Gateway registration credential is not applicable")

const (
	registrationReasonNotRegistered = "GATEWAY_NOT_REGISTERED"
	registrationReasonApprovalWait  = "GATEWAY_APPROVAL_PENDING"
)

const registrationApprovalPollInterval = 2 * time.Second

func NewControlHandler(instanceID string, authenticator *auth.GatewayAuthenticator, registrations GatewayRegistrationRepository, sessions session.Store, heartbeats report.HeartbeatRecorder, reporter report.Reporter, configurations configsync.Source, authenticationTTL, heartbeatInterval, sessionLeaseTTL time.Duration, plaintextAllowed bool, clientDependencies ...ClientControlDependencies) (*ControlHandler, error) {
	if instanceID == "" {
		return nil, errors.New("HUB instance ID is required")
	}
	if authenticator == nil {
		return nil, errors.New("Gateway authenticator is required")
	}
	if registrations == nil || sessions == nil || heartbeats == nil || reporter == nil || configurations == nil {
		return nil, errors.New("HUB registration, session, runtime, and configuration repositories are required")
	}
	if authenticationTTL <= 0 {
		return nil, errors.New("HUB authentication timeout must be positive")
	}
	if heartbeatInterval <= 0 || sessionLeaseTTL <= heartbeatInterval {
		return nil, errors.New("HUB session lease must exceed the positive heartbeat interval")
	}
	handler := &ControlHandler{
		instanceID:        instanceID,
		plaintextAllowed:  plaintextAllowed,
		authenticationTTL: authenticationTTL,
		heartbeatInterval: heartbeatInterval,
		sessionLeaseTTL:   sessionLeaseTTL,
		authenticator:     authenticator,
		registrations:     registrations,
		sessions:          sessions,
		heartbeats:        heartbeats,
		reporter:          reporter,
		configurations:    configurations,
		activeStreams:     session.NewActiveRegistry(),
	}
	if len(clientDependencies) > 1 {
		return nil, errors.New("at most one Client control dependency set may be configured")
	}
	if len(clientDependencies) == 1 {
		dependencies := clientDependencies[0]
		if dependencies.Authenticator == nil || dependencies.Registrations == nil {
			return nil, errors.New("Client authentication and registration repositories are both required")
		}
		handler.clientAuth = dependencies.Authenticator
		handler.clientRegistrations = dependencies.Registrations
	}
	return handler, nil
}

func (h *ControlHandler) RegisterGRPC(server *Server) {
	server.RegisterService(func(registrar grpc.ServiceRegistrar) {
		hubv1.RegisterHubControlServer(registrar, h)
	})
}

func (h *ControlHandler) NotifyConfigurationChange(nodeCode, reason string) bool {
	return h.activeStreams.Notify(nodeCode, reason)
}

func (h *ControlHandler) Connect(stream hubv1.HubControl_ConnectServer) error {
	first, err := h.receiveWithin(stream, h.authenticationTTL)
	if err != nil {
		return h.mapAuthenticationReceiveError("receive Gateway hello", err)
	}
	hello := first.GetHello()
	if hello == nil {
		return status.Error(codes.Unauthenticated, "the first control message must be a Gateway hello")
	}
	if hello.GetNodeKind() == hubv1.NodeKind_NODE_KIND_CLIENT {
		return h.connectClient(stream, hello)
	}
	if hello.GetNodeKind() != hubv1.NodeKind_NODE_KIND_GATEWAY {
		return status.Error(codes.Unauthenticated, "this HUB control stream currently accepts authenticated Gateways only")
	}
	claimedKey, err := base64.StdEncoding.DecodeString(hello.GetWireguardPublicKey())
	if err != nil || len(claimedKey) != 32 {
		return status.Error(codes.Unauthenticated, "Gateway X25519 public key is invalid")
	}
	defer clear(claimedKey)

	var identity auth.GatewayIdentity
	registrationCompleted := false
	if registrationToken := hello.GetRegistrationToken(); registrationToken != "" {
		hello.RegistrationToken = ""
		identity, err = h.registerGateway(stream, registrationToken, claimedKey)
		if errors.Is(err, errRegistrationCredentialNotApplicable) {
			return status.Error(codes.Unauthenticated, "Gateway registration credential is invalid or no longer applicable")
		} else if err == nil {
			registrationCompleted = true
		}
		if err != nil {
			return err
		}
	} else {
		identity, err = h.authenticateGateway(stream, claimedKey)
		if err != nil {
			return err
		}
	}
	if registrationCompleted {
		if err := stream.Send(&hubv1.HubMessage{Body: &hubv1.HubMessage_GatewayRegistrationResult{GatewayRegistrationResult: &hubv1.GatewayRegistrationResult{
			Registered:       true,
			ApprovalRequired: identity.NodeStatus != "active",
			NodeStatus:       identity.NodeStatus,
		}}}); err != nil {
			return fmt.Errorf("send Gateway registration result: %w", err)
		}
		if identity.NodeStatus != "active" {
			return h.waitForGatewayApproval(stream, identity.NodeID, claimedKey)
		}
		return nil
	}
	initialSnapshot, hasInitialSnapshot, err := h.configurations.Current(stream.Context(), identity.NodeID, hello.GetAppliedConfigHash())
	if err != nil {
		return mapConfigurationSourceError("load Gateway desired configuration", identity.NodeID, err)
	}
	if !hasInitialSnapshot && hello.GetAppliedConfigHash() != "" {
		if err := h.configurations.MarkApplied(stream.Context(), identity.NodeID, hello.GetAppliedConfigHash()); err != nil {
			slog.Warn("reconcile Gateway configuration task", "node_id", identity.NodeID, "error", err)
		}
		h.acknowledgeAccessChangesIfCurrent(stream.Context(), identity.NodeID, hello.GetAppliedConfigHash())
	}

	sessionID, err := newSessionID()
	if err != nil {
		return status.Error(codes.Internal, "could not initialize Gateway session")
	}
	now := time.Now()
	if err := h.sessions.Acquire(stream.Context(), session.Record{
		SessionID:  sessionID,
		NodeID:     identity.NodeID,
		InstanceID: h.instanceID,
		LeaseUntil: now.Add(h.sessionLeaseTTL),
	}); err != nil {
		slog.Error("acquire HUB Gateway session", "node_id", identity.NodeID, "error", err)
		return status.Error(codes.Unavailable, "Gateway session could not be established")
	}
	defer h.releaseSession(sessionID, identity.NodeID)
	changeSignal, unregister := h.activeStreams.Register(identity.NodeID)
	defer unregister()
	welcome := &hubv1.Welcome{
		SessionId:                sessionID,
		HeartbeatIntervalSeconds: uint32(h.heartbeatInterval / time.Second),
	}
	if hasInitialSnapshot {
		welcome.ConfigSnapshot = toConfigSnapshot(initialSnapshot)
	}
	if err := stream.Send(&hubv1.HubMessage{Body: &hubv1.HubMessage_Welcome{Welcome: welcome}}); err != nil {
		return fmt.Errorf("send authenticated Gateway welcome: %w", err)
	}
	if hasInitialSnapshot {
		if err := h.configurations.MarkSent(stream.Context(), identity.NodeID, initialSnapshot); err != nil {
			return status.Error(codes.Unavailable, "Gateway configuration task could not be updated")
		}
	}
	slog.Info("Gateway control identity authenticated", "node_id", identity.NodeID, "hub_instance_id", h.instanceID, "session_id", sessionID)
	var pendingSnapshot *configsync.Snapshot
	if hasInitialSnapshot {
		pendingSnapshot = &initialSnapshot
	}
	return h.serveAuthenticatedStream(stream, identity, sessionID, pendingSnapshot, changeSignal)
}

func (h *ControlHandler) connectClient(stream hubv1.HubControl_ConnectServer, hello *hubv1.Hello) error {
	if h.clientAuth == nil || h.clientRegistrations == nil {
		return status.Error(codes.Unavailable, "HUB Client control is not configured")
	}
	if hello.GetRegistrationToken() != "" {
		return status.Error(codes.Unauthenticated, "Client registration tokens are no longer accepted; employee login is required")
	}
	claimedKey, err := base64.StdEncoding.DecodeString(hello.GetWireguardPublicKey())
	if err != nil || len(claimedKey) != 32 {
		return status.Error(codes.Unauthenticated, "Client X25519 public key is invalid")
	}
	defer clear(claimedKey)
	password := []byte(hello.GetClientPassword())
	hello.ClientPassword = ""
	defer clear(password)
	if _, err := tlsStateFromContext(stream.Context()); err != nil && !h.plaintextAllowed {
		return gatewayAuthorizationStateError("CLIENT_LOGIN_FAILED", "Client employee login requires a TLS-protected HUB connection")
	}
	identity, created, err := h.clientRegistrations.LoginClient(stream.Context(), hello.GetClientUsername(), password, claimedKey,
		hello.GetClientDeviceName(), hello.GetClientPlatform(), gatewayPeerIP(stream.Context()), hello.GetAgentVersion(), time.Now().UTC())
	if err != nil {
		switch {
		case errors.Is(err, auth.ErrClientLoginFailed):
			slog.Warn("reject Client employee login")
			return gatewayAuthorizationStateError("CLIENT_LOGIN_FAILED", "Client employee login was rejected")
		case errors.Is(err, auth.ErrClientApprovalPending):
			return gatewayAuthorizationStateError("CLIENT_APPROVAL_PENDING", "Client device registration is awaiting approval")
		case errors.Is(err, auth.ErrClientNotAllowed):
			return status.Error(codes.Unauthenticated, "Client device is not authorized")
		default:
			slog.Error("authenticate Client employee login", "error", err)
			return status.Error(codes.Unavailable, "Client employee authentication is unavailable")
		}
	}
	if created {
		if err := stream.Send(&hubv1.HubMessage{Body: &hubv1.HubMessage_ClientRegistrationResult{ClientRegistrationResult: &hubv1.ClientRegistrationResult{
			Registered: true, NodeStatus: identity.NodeStatus,
		}}}); err != nil {
			return fmt.Errorf("send Client device registration result: %w", err)
		}
		slog.Info("registered Client device from employee login", "node_id", identity.NodeID, "owner_id", identity.OwnerID, "device_id", identity.DeviceID)
	}
	identity, err = h.authenticateClient(stream, claimedKey)
	if err != nil {
		return err
	}
	initialSnapshot, hasSnapshot, err := h.configurations.Current(stream.Context(), identity.NodeID, hello.GetAppliedConfigHash())
	if err != nil {
		return mapConfigurationSourceError("load Client desired configuration", identity.NodeID, err)
	}
	if !hasSnapshot && hello.GetAppliedConfigHash() != "" {
		if err := h.configurations.MarkApplied(stream.Context(), identity.NodeID, hello.GetAppliedConfigHash()); err != nil {
			slog.Warn("reconcile Client configuration task", "node_id", identity.NodeID, "error", err)
		}
	}
	sessionID, err := newSessionID()
	if err != nil {
		return status.Error(codes.Internal, "could not initialize Client session")
	}
	now := time.Now()
	if err := h.sessions.Acquire(stream.Context(), session.Record{
		SessionID: sessionID, NodeID: identity.NodeID, InstanceID: h.instanceID,
		LeaseUntil: now.Add(h.sessionLeaseTTL),
	}); err != nil {
		slog.Error("acquire HUB Client session", "node_id", identity.NodeID, "error", err)
		return status.Error(codes.Unavailable, "Client session could not be established")
	}
	defer h.releaseSession(sessionID, identity.NodeID)
	changeSignal, unregister := h.activeStreams.Register(identity.NodeID)
	defer unregister()
	welcome := &hubv1.Welcome{SessionId: sessionID, HeartbeatIntervalSeconds: uint32(h.heartbeatInterval / time.Second)}
	if hasSnapshot {
		welcome.ConfigSnapshot = toConfigSnapshot(initialSnapshot)
	}
	if err := stream.Send(&hubv1.HubMessage{Body: &hubv1.HubMessage_Welcome{Welcome: welcome}}); err != nil {
		return fmt.Errorf("send authenticated Client welcome: %w", err)
	}
	if hasSnapshot {
		if err := h.configurations.MarkSent(stream.Context(), identity.NodeID, initialSnapshot); err != nil {
			return status.Error(codes.Unavailable, "Client configuration task could not be updated")
		}
	}
	slog.Info("Client control identity authenticated", "node_id", identity.NodeID, "hub_instance_id", h.instanceID, "session_id", sessionID)
	var pending *configsync.Snapshot
	if hasSnapshot {
		pending = &initialSnapshot
	}
	return h.serveAuthenticatedStream(stream, identity, sessionID, pending, changeSignal)
}

func (h *ControlHandler) authenticateClient(stream hubv1.HubControl_ConnectServer, claimedKey []byte) (auth.ClientIdentity, error) {
	challengeID, err := hubauth.NewChallengeID()
	if err != nil {
		return auth.ClientIdentity{}, status.Error(codes.Internal, "could not initialize Client authentication")
	}
	binding, err := h.channelBinding(stream.Context(), challengeID)
	if err != nil {
		return auth.ClientIdentity{}, status.Error(codes.Unauthenticated, "control channel binding is unavailable")
	}
	identity, challenge, pending, err := h.clientAuth.Begin(stream.Context(), h.instanceID, claimedKey, challengeID, binding)
	clear(binding)
	if err != nil {
		switch {
		case errors.Is(err, auth.ErrClientNotRegistered):
			return auth.ClientIdentity{}, gatewayAuthorizationStateError("CLIENT_NOT_REGISTERED", "Client device is not registered")
		case errors.Is(err, auth.ErrClientApprovalPending):
			return identity, gatewayAuthorizationStateError("CLIENT_APPROVAL_PENDING", "Client registration is awaiting Management approval")
		case errors.Is(err, auth.ErrClientNotFound), errors.Is(err, auth.ErrClientNotAllowed), errors.Is(err, auth.ErrPublicKeyMismatch):
			return auth.ClientIdentity{}, status.Error(codes.Unauthenticated, "Client identity is not authorized")
		default:
			slog.Error("load HUB Client authorization state", "error", err)
			return auth.ClientIdentity{}, status.Error(codes.Unavailable, "Client authorization state is unavailable")
		}
	}
	if err := sendChallenge(stream, challenge, false); err != nil {
		return auth.ClientIdentity{}, fmt.Errorf("send Client authentication challenge: %w", err)
	}
	if err := receiveProofAndVerify(h, stream, pending, identity.NodeID, challenge.ExpiresAt); err != nil {
		return auth.ClientIdentity{}, err
	}
	return identity, nil
}

func (h *ControlHandler) authenticateGateway(stream hubv1.HubControl_ConnectServer, claimedKey []byte) (auth.GatewayIdentity, error) {
	challengeID, err := hubauth.NewChallengeID()
	if err != nil {
		return auth.GatewayIdentity{}, status.Error(codes.Internal, "could not initialize Gateway authentication")
	}
	channelBinding, err := h.channelBinding(stream.Context(), challengeID)
	if err != nil {
		slog.Warn("derive HUB control channel binding", "error", err)
		return auth.GatewayIdentity{}, status.Error(codes.Unauthenticated, "control channel binding is unavailable")
	}
	identity, challenge, pending, err := h.authenticator.Begin(
		stream.Context(), h.instanceID, claimedKey, challengeID, channelBinding,
	)
	clear(channelBinding)
	if err != nil {
		if errors.Is(err, auth.ErrGatewayNotRegistered) {
			return auth.GatewayIdentity{}, gatewayAuthorizationStateError(registrationReasonNotRegistered, "Gateway node is not registered")
		}
		if errors.Is(err, auth.ErrGatewayApprovalPending) {
			return auth.GatewayIdentity{}, gatewayAuthorizationStateError(registrationReasonApprovalWait, "Gateway registration is awaiting Management approval")
		}
		if errors.Is(err, auth.ErrGatewayNotFound) || errors.Is(err, auth.ErrGatewayNotAllowed) || errors.Is(err, auth.ErrPublicKeyMismatch) {
			slog.Warn("reject HUB Gateway identity", "reason", err)
			return auth.GatewayIdentity{}, status.Error(codes.Unauthenticated, "Gateway identity is not authorized")
		}
		slog.Error("load HUB Gateway authorization state", "error", err)
		return auth.GatewayIdentity{}, status.Error(codes.Unavailable, "Gateway authorization state is unavailable")
	}
	if err := sendChallenge(stream, challenge, false); err != nil {
		return auth.GatewayIdentity{}, fmt.Errorf("send Gateway authentication challenge: %w", err)
	}
	if err := receiveProofAndVerify(h, stream, pending, identity.NodeID, challenge.ExpiresAt); err != nil {
		return auth.GatewayIdentity{}, err
	}
	return identity, nil
}

func (h *ControlHandler) waitForGatewayApproval(stream hubv1.HubControl_ConnectServer, nodeID string, claimedKey []byte) error {
	slog.Info("Gateway registration is awaiting Management approval", "node_id", nodeID)
	ticker := time.NewTicker(registrationApprovalPollInterval)
	defer ticker.Stop()
	for {
		select {
		case <-stream.Context().Done():
			return stream.Context().Err()
		case <-ticker.C:
		}

		identity, err := h.registrations.RegistrationState(stream.Context(), nodeID)
		if errors.Is(err, auth.ErrGatewayNotFound) {
			return status.Error(codes.FailedPrecondition, "Gateway registration target no longer exists")
		}
		if errors.Is(err, auth.ErrRegistrationNotAllowed) {
			return status.Error(codes.FailedPrecondition, "Gateway registration state is incomplete or no longer valid")
		}
		if err != nil {
			slog.Warn("check Gateway registration approval", "node_id", nodeID, "error", err)
			continue
		}
		if len(identity.PublicKey) != len(claimedKey) || subtle.ConstantTimeCompare(identity.PublicKey, claimedKey) != 1 {
			return status.Error(codes.FailedPrecondition, "Gateway registration identity changed while awaiting approval")
		}
		if identity.NodeStatus == "pending" {
			continue
		}
		if identity.NodeStatus != "active" {
			return status.Errorf(codes.PermissionDenied, "Gateway registration was not approved (node_status=%q)", identity.NodeStatus)
		}
		if err := stream.Send(&hubv1.HubMessage{Body: &hubv1.HubMessage_GatewayRegistrationResult{GatewayRegistrationResult: &hubv1.GatewayRegistrationResult{
			Registered:       true,
			ApprovalRequired: true,
			NodeStatus:       identity.NodeStatus,
		}}}); err != nil {
			return fmt.Errorf("send approved Gateway registration result: %w", err)
		}
		return nil
	}
}

func gatewayAuthorizationStateError(reason, message string) error {
	errInfo := &errdetails.ErrorInfo{Reason: reason, Domain: "velotrust.hub"}
	result, err := status.New(codes.FailedPrecondition, message).WithDetails(errInfo)
	if err != nil {
		return status.Error(codes.FailedPrecondition, message)
	}
	return result.Err()
}

func (h *ControlHandler) registerGateway(stream hubv1.HubControl_ConnectServer, token string, claimedKey []byte) (auth.GatewayIdentity, error) {
	if len(token) > 128 || !strings.HasPrefix(token, "vtreg_") {
		return auth.GatewayIdentity{}, status.Error(codes.Unauthenticated, "Gateway registration was rejected")
	}
	encodedSecret := strings.TrimPrefix(token, "vtreg_")
	secret, err := base64.RawURLEncoding.DecodeString(encodedSecret)
	if err != nil || len(secret) != 32 {
		return auth.GatewayIdentity{}, status.Error(codes.Unauthenticated, "Gateway registration was rejected")
	}
	clear(secret)
	digest := sha256.Sum256([]byte(token))
	tokenHash := hex.EncodeToString(digest[:])
	identity, err := h.registrations.CheckCredential(stream.Context(), tokenHash)
	if err != nil {
		if errors.Is(err, auth.ErrRegistrationNotAllowed) {
			return auth.GatewayIdentity{}, errRegistrationCredentialNotApplicable
		}
		slog.Error("check Gateway registration credential", "error", err)
		return auth.GatewayIdentity{}, status.Error(codes.Unavailable, "Gateway registration service is unavailable")
	}
	nodeID := identity.NodeID

	challengeID, err := hubauth.NewChallengeID()
	if err != nil {
		return auth.GatewayIdentity{}, status.Error(codes.Internal, "could not initialize Gateway registration")
	}
	channelBinding, err := h.channelBinding(stream.Context(), challengeID)
	if err != nil {
		return auth.GatewayIdentity{}, status.Error(codes.Unauthenticated, "control channel binding is unavailable")
	}
	challenge, pending, err := hubauth.NewGatewayRegistrationChallenge(nodeID, h.instanceID, challengeID, claimedKey, channelBinding, time.Now(), h.authenticationTTL)
	clear(channelBinding)
	if err != nil {
		return auth.GatewayIdentity{}, status.Error(codes.Unauthenticated, "Gateway registration proof could not be initialized")
	}
	if err := sendChallenge(stream, challenge, true); err != nil {
		return auth.GatewayIdentity{}, fmt.Errorf("send Gateway registration challenge: %w", err)
	}
	proofMessage, err := h.receiveWithin(stream, time.Until(challenge.ExpiresAt))
	if err != nil {
		return auth.GatewayIdentity{}, h.mapAuthenticationReceiveError("receive Gateway registration proof", err)
	}
	proof := proofMessage.GetX25519AuthProof()
	if proof == nil {
		return auth.GatewayIdentity{}, status.Error(codes.Unauthenticated, "Gateway must answer the registration challenge")
	}
	if err := pending.Verify(proof.GetChallengeId(), proof.GetProof(), time.Now()); err != nil {
		slog.Warn("reject Gateway registration key proof", "node_id", nodeID, "error", err)
		return auth.GatewayIdentity{}, status.Error(codes.Unauthenticated, "Gateway registration proof is invalid or expired")
	}
	usedIP := gatewayPeerIP(stream.Context())
	identity, _, err = h.registrations.RegisterGateway(stream.Context(), nodeID, tokenHash, claimedKey, usedIP, time.Now().UTC())
	if err != nil {
		if errors.Is(err, auth.ErrRegistrationNotAllowed) || errors.Is(err, auth.ErrPublicKeyAlreadyBound) {
			slog.Warn("reject Gateway registration", "node_id", nodeID, "reason", err)
			return auth.GatewayIdentity{}, status.Error(codes.Unauthenticated, "Gateway registration was rejected")
		}
		slog.Error("complete Gateway registration", "node_id", nodeID, "error", err)
		return auth.GatewayIdentity{}, status.Error(codes.Unavailable, "Gateway registration could not be completed")
	}
	return identity, nil
}

func (h *ControlHandler) channelBinding(ctx context.Context, challengeID []byte) ([]byte, error) {
	tlsState, err := tlsStateFromContext(ctx)
	if err == nil {
		return hubauth.TLSExporterBinding(tlsState, challengeID)
	}
	if !h.plaintextAllowed {
		return nil, errors.New("TLS channel binding is unavailable and plaintext mode is disabled")
	}
	return hubauth.PlaintextChannelBinding(challengeID)
}

func sendChallenge(stream hubv1.HubControl_ConnectServer, challenge hubauth.Challenge, registration bool) error {
	return stream.Send(&hubv1.HubMessage{Body: &hubv1.HubMessage_X25519AuthChallenge{X25519AuthChallenge: &hubv1.X25519AuthChallenge{
		ChallengeId:           challenge.ID,
		Nonce:                 challenge.Nonce,
		HubEphemeralPublicKey: challenge.HubPublicKey,
		ExpiresAtUnixMillis:   challenge.ExpiresAt.UnixMilli(),
		HubInstanceId:         challenge.HubInstanceID,
		RegistrationChallenge: registration,
		NodeId:                challenge.NodeID,
	}}})
}

func toConfigSnapshot(snapshot configsync.Snapshot) *hubv1.ConfigSnapshot {
	return &hubv1.ConfigSnapshot{
		ConfigHash: snapshot.Revision,
		Payload:    append([]byte(nil), snapshot.Payload...),
		TaskId:     snapshot.TaskID,
	}
}

func receiveProofAndVerify(h *ControlHandler, stream hubv1.HubControl_ConnectServer, pending *hubauth.PendingChallenge, nodeID string, expiresAt time.Time) error {
	proofMessage, err := h.receiveWithin(stream, time.Until(expiresAt))
	if err != nil {
		return h.mapAuthenticationReceiveError("receive Gateway authentication proof", err)
	}
	proof := proofMessage.GetX25519AuthProof()
	if proof == nil {
		return status.Error(codes.Unauthenticated, "Gateway must answer the X25519 challenge before other control messages")
	}
	if err := pending.Verify(proof.GetChallengeId(), proof.GetProof(), time.Now()); err != nil {
		slog.Warn("reject HUB Gateway X25519 proof", "node_id", nodeID, "error", err)
		return status.Error(codes.Unauthenticated, "Gateway X25519 proof is invalid or expired")
	}
	return nil
}

func gatewayPeerIP(ctx context.Context) string {
	remote, ok := peer.FromContext(ctx)
	if !ok || remote.Addr == nil {
		return ""
	}
	host, _, err := net.SplitHostPort(remote.Addr.String())
	if err != nil {
		return ""
	}
	if parsed := net.ParseIP(host); parsed != nil {
		return parsed.String()
	}
	return ""
}

func (h *ControlHandler) serveAuthenticatedStream(stream hubv1.HubControl_ConnectServer, identity auth.GatewayIdentity, sessionID string, pendingSnapshot *configsync.Snapshot, changeSignal <-chan string) error {
	var lastHeartbeatSequence uint64
	var lastStatusSequence uint64
	lastHeartbeatAt := time.Now()
	for {
		leaseUntil := lastHeartbeatAt.Add(h.sessionLeaseTTL)
		message, err, changeReason := h.receiveWithinOrNotify(stream, time.Until(leaseUntil), changeSignal)
		if changeReason != "" {
			if err := stream.Send(&hubv1.HubMessage{Body: &hubv1.HubMessage_Reauthenticate{Reauthenticate: &hubv1.Reauthenticate{ReasonCode: changeReason}}}); err != nil {
				return fmt.Errorf("notify Gateway to refresh its access configuration: %w", err)
			}
			return status.Error(codes.Unavailable, "Gateway must reconnect to apply current authorization")
		}
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return mapReceiveError("receive authenticated Gateway control message", err)
		}
		switch body := message.GetBody().(type) {
		case *hubv1.AgentMessage_Heartbeat:
			sequence := body.Heartbeat.GetSequence()
			if sequence == 0 || sequence <= lastHeartbeatSequence {
				return status.Error(codes.FailedPrecondition, "Gateway heartbeat sequence must increase")
			}
			now := time.Now()
			var recordErr error
			leaseUntil := now.Add(h.sessionLeaseTTL)
			err := h.sessions.WithCurrent(stream.Context(), sessionID, h.instanceID, leaseUntil, func() error {
				recordErr = h.heartbeats.RecordHeartbeat(stream.Context(), identity.NodeID, identity.PublicKey, now)
				return nil
			})
			if err != nil {
				return mapSessionError(err)
			}
			if recordErr != nil {
				return mapRuntimeRepositoryError("Gateway heartbeat", recordErr)
			}
			lastHeartbeatAt = now
			lastHeartbeatSequence = sequence
			if err := stream.Send(&hubv1.HubMessage{Body: &hubv1.HubMessage_HeartbeatAck{HeartbeatAck: &hubv1.HeartbeatAck{
				Sequence:       sequence,
				ServerTimeUnix: now.Unix(),
			}}}); err != nil {
				return fmt.Errorf("send Gateway heartbeat acknowledgment: %w", err)
			}
		case *hubv1.AgentMessage_RuntimeStatus:
			runtimeStatus := body.RuntimeStatus
			if runtimeStatus.GetSequence() == 0 || runtimeStatus.GetSequence() <= lastStatusSequence {
				return status.Error(codes.FailedPrecondition, "Gateway runtime status sequence must increase")
			}
			now := time.Now()
			wireGuardStatus := runtimeStatus.GetWireguardStatus()
			if wireGuardStatus == "" {
				wireGuardStatus = "unknown"
			}
			healthStatus := runtimeStatus.GetHealthStatus()
			if healthStatus == "" {
				healthStatus = "unknown"
			}
			var reportErr error
			err := h.sessions.WithCurrent(stream.Context(), sessionID, h.instanceID, leaseUntil, func() error {
				reportErr = h.reporter.Report(stream.Context(), report.NodeStatus{
					NodeID:                  identity.NodeID,
					RegisteredPublicKey:     identity.PublicKey,
					AgentVersion:            runtimeStatus.GetAgentVersion(),
					AppliedConfigHash:       runtimeStatus.GetAppliedConfigHash(),
					WireGuardStatus:         wireGuardStatus,
					HealthStatus:            healthStatus,
					ConnectedPeerCount:      runtimeStatus.GetConnectedPeerCount(),
					ConnectedPeerCountKnown: runtimeStatus.GetConnectedPeerCountKnown(),
					LastErrorCode:           runtimeStatus.GetLastErrorCode(),
					ReportedAt:              now,
				})
				return nil
			})
			if err != nil {
				return mapSessionError(err)
			}
			if reportErr != nil {
				return mapRuntimeRepositoryError("Gateway runtime status", reportErr)
			}
			lastStatusSequence = runtimeStatus.GetSequence()
			configuration, sendConfiguration, err := h.configurations.Current(stream.Context(), identity.NodeID, runtimeStatus.GetAppliedConfigHash())
			if err != nil {
				return mapConfigurationSourceError("load updated Gateway configuration", identity.NodeID, err)
			}
			ack := &hubv1.RuntimeStatusAck{Sequence: lastStatusSequence, ServerTimeUnix: now.Unix()}
			if sendConfiguration {
				ack.ConfigSnapshot = toConfigSnapshot(configuration)
			}
			if err := stream.Send(&hubv1.HubMessage{Body: &hubv1.HubMessage_RuntimeStatusAck{RuntimeStatusAck: ack}}); err != nil {
				return fmt.Errorf("send Gateway runtime status acknowledgment: %w", err)
			}
			if sendConfiguration {
				if err := h.configurations.MarkSent(stream.Context(), identity.NodeID, configuration); err != nil {
					return status.Error(codes.Unavailable, "Gateway configuration task could not be updated")
				}
				pendingSnapshot = &configuration
			} else if runtimeStatus.GetAppliedConfigHash() != "" {
				if err := h.configurations.MarkApplied(stream.Context(), identity.NodeID, runtimeStatus.GetAppliedConfigHash()); err != nil {
					slog.Warn("reconcile Gateway configuration task", "node_id", identity.NodeID, "error", err)
				}
				h.acknowledgeAccessChangesIfCurrent(stream.Context(), identity.NodeID, runtimeStatus.GetAppliedConfigHash())
			}
		case *hubv1.AgentMessage_ConfigApplied:
			applied := body.ConfigApplied
			if pendingSnapshot == nil || applied.GetConfigHash() != pendingSnapshot.Revision || applied.GetTaskId() != pendingSnapshot.TaskID {
				return status.Error(codes.FailedPrecondition, "Gateway configuration receipt does not match a delivered snapshot")
			}
			if err := h.configurations.Acknowledge(stream.Context(), identity.NodeID, applied.GetTaskId(), applied.GetConfigHash(), applied.GetSuccess(), applied.GetErrorCode(), applied.GetErrorMessage()); err != nil {
				slog.Error("record Gateway configuration result", "node_id", identity.NodeID, "task_id", applied.GetTaskId(), "error", err)
				return status.Error(codes.FailedPrecondition, "Gateway configuration result could not be recorded")
			}
			if !applied.GetSuccess() {
				if recorder, ok := h.configurations.(accessChangeFailureRecorder); ok {
					message := applied.GetErrorMessage()
					if message == "" {
						message = applied.GetErrorCode()
					}
					if err := recorder.RecordAccessChangeFailure(stream.Context(), identity.NodeID, message); err != nil {
						slog.Warn("record Gateway access-change apply failure", "node_id", identity.NodeID, "error", err)
					}
				}
				return status.Error(codes.FailedPrecondition, "Gateway failed to apply its desired configuration")
			}
			h.acknowledgeAccessChangesIfCurrent(stream.Context(), identity.NodeID, applied.GetConfigHash())
			pendingSnapshot = nil
		default:
			return status.Error(codes.FailedPrecondition, "control message is not valid after Gateway authentication")
		}
	}
}

type accessChangeAcknowledger interface {
	AcknowledgeCurrentConfig(context.Context, string) error
}

type accessChangeFailureRecorder interface {
	RecordAccessChangeFailure(context.Context, string, string) error
}

func (h *ControlHandler) acknowledgeAccessChangesIfCurrent(ctx context.Context, nodeCode, appliedHash string) {
	if appliedHash == "" {
		return
	}
	_, hasNewer, err := h.configurations.Current(ctx, nodeCode, appliedHash)
	if err != nil || hasNewer {
		return
	}
	acknowledger, ok := h.configurations.(accessChangeAcknowledger)
	if !ok {
		return
	}
	if err := acknowledger.AcknowledgeCurrentConfig(ctx, nodeCode); err != nil {
		slog.Warn("acknowledge Gateway access-change delivery", "node_id", nodeCode, "error", err)
	}
}

func mapSessionError(err error) error {
	if errors.Is(err, session.ErrStaleSession) {
		return status.Error(codes.Unauthenticated, "Gateway session is no longer current")
	}
	if errors.Is(err, context.Canceled) {
		return status.Error(codes.Canceled, "Gateway session was canceled")
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return status.Error(codes.DeadlineExceeded, "Gateway session deadline expired")
	}
	slog.Error("process HUB Gateway session", "error", err)
	return status.Error(codes.Unavailable, "Gateway session could not be renewed")
}

func (h *ControlHandler) receiveWithin(stream hubv1.HubControl_ConnectServer, remaining time.Duration) (*hubv1.AgentMessage, error) {
	if remaining <= 0 {
		return nil, status.Error(codes.DeadlineExceeded, "Gateway heartbeat lease expired")
	}
	type receiveResult struct {
		message *hubv1.AgentMessage
		err     error
	}
	result := make(chan receiveResult, 1)
	go func() {
		message, err := stream.Recv()
		result <- receiveResult{message: message, err: err}
	}()
	timer := time.NewTimer(remaining)
	defer timer.Stop()
	select {
	case received := <-result:
		return received.message, received.err
	case <-timer.C:
		return nil, status.Error(codes.DeadlineExceeded, "Gateway heartbeat lease expired")
	case <-stream.Context().Done():
		return nil, stream.Context().Err()
	}
}

func (h *ControlHandler) receiveWithinOrNotify(stream hubv1.HubControl_ConnectServer, remaining time.Duration, changed <-chan string) (*hubv1.AgentMessage, error, string) {
	if remaining <= 0 {
		return nil, status.Error(codes.DeadlineExceeded, "Gateway heartbeat lease expired"), ""
	}
	type receiveResult struct {
		message *hubv1.AgentMessage
		err     error
	}
	result := make(chan receiveResult, 1)
	go func() {
		message, err := stream.Recv()
		result <- receiveResult{message: message, err: err}
	}()
	timer := time.NewTimer(remaining)
	defer timer.Stop()
	select {
	case received := <-result:
		return received.message, received.err, ""
	case <-timer.C:
		return nil, status.Error(codes.DeadlineExceeded, "Gateway heartbeat lease expired"), ""
	case <-stream.Context().Done():
		return nil, stream.Context().Err(), ""
	case reason := <-changed:
		return nil, nil, reason
	}
}

func (h *ControlHandler) mapAuthenticationReceiveError(operation string, err error) error {
	if errors.Is(err, io.EOF) {
		return status.Error(codes.Unauthenticated, "Gateway closed the authentication stream")
	}
	if grpcStatus, ok := status.FromError(err); ok && grpcStatus.Code() == codes.DeadlineExceeded {
		return status.Error(codes.Unauthenticated, "Gateway authentication exchange timed out")
	}
	return mapReceiveError(operation, err)
}

func (h *ControlHandler) releaseSession(sessionID, nodeID string) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := h.sessions.Release(ctx, sessionID, h.instanceID); err != nil {
		slog.Warn("release HUB Gateway session", "node_id", nodeID, "session_id", sessionID, "error", err)
	}
}

func mapRuntimeRepositoryError(operation string, err error) error {
	if errors.Is(err, auth.ErrGatewayNotFound) || errors.Is(err, auth.ErrGatewayNotAllowed) || errors.Is(err, auth.ErrClientNotFound) || errors.Is(err, auth.ErrClientNotAllowed) || errors.Is(err, auth.ErrPublicKeyMismatch) {
		return status.Error(codes.Unauthenticated, "node authorization has changed")
	}
	if errors.Is(err, report.ErrInvalidRuntimeStatus) {
		return status.Error(codes.InvalidArgument, "Gateway runtime status is invalid")
	}
	slog.Error(operation, "error", err)
	return status.Error(codes.Unavailable, "Gateway runtime state could not be persisted")
}

func mapConfigurationSourceError(operation, nodeID string, err error) error {
	switch {
	case errors.Is(err, configsync.ErrTargetUnavailable):
		slog.Warn(operation, "node_id", nodeID, "error", err)
		return status.Error(codes.Unauthenticated, "Gateway configuration target is no longer authorized")
	case errors.Is(err, configsync.ErrConfigurationRemoved), errors.Is(err, configsync.ErrInvalidConfiguration):
		slog.Error(operation, "node_id", nodeID, "error", err)
		return status.Error(codes.FailedPrecondition, "Gateway current configuration is invalid or missing")
	default:
		slog.Error(operation, "node_id", nodeID, "error", err)
		return status.Error(codes.Unavailable, "Gateway desired configuration is unavailable")
	}
}

func tlsStateFromContext(ctx context.Context) (tls.ConnectionState, error) {
	p, ok := peer.FromContext(ctx)
	if !ok || p.AuthInfo == nil {
		return tls.ConnectionState{}, errors.New("gRPC peer has no TLS authentication state")
	}
	switch info := p.AuthInfo.(type) {
	case credentials.TLSInfo:
		return info.State, nil
	case *credentials.TLSInfo:
		if info != nil {
			return info.State, nil
		}
	}
	return tls.ConnectionState{}, errors.New("gRPC peer is not using TLS")
}

func newSessionID() (string, error) {
	value := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, value); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(value), nil
}

func mapReceiveError(operation string, err error) error {
	if errors.Is(err, io.EOF) {
		return nil
	}
	if grpcStatus, ok := status.FromError(err); ok {
		return grpcStatus.Err()
	}
	return status.Errorf(codes.Unavailable, "%s: %v", operation, err)
}
