package control

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
	"time"

	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/status"

	"velotrust/gateway/config"
	"velotrust/gateway/internal/agent"
	"velotrust/gateway/internal/buildinfo"
	"velotrust/pkg/hubauth"
	"velotrust/shared/configpayload"
	hubv1 "velotrust/shared/gen/go/hub/v1"
)

const (
	initialRetryDelay   = time.Second
	maximumRetryDelay   = 30 * time.Second
	clientAuthTimeout   = 45 * time.Second
	maximumHeartbeatGap = time.Hour
)

var errRegistrationCompleted = errors.New("Gateway registration completed; identity authentication must be repeated")
var errAccessConfigurationChanged = errors.New("HUB requested an immediate authorization refresh")
var errAccessRevoked = errors.New("HUB revoked Gateway forwarding authorization")

type SnapshotApplier func(context.Context, *hubv1.ConfigSnapshot) error
type DisconnectHandler func() error

type controlReceiveResult struct {
	message *hubv1.HubMessage
	err     error
}

type controlMessageReader struct {
	ctx     context.Context
	results chan controlReceiveResult
}

func newControlMessageReader(ctx context.Context, stream grpc.BidiStreamingClient[hubv1.AgentMessage, hubv1.HubMessage]) *controlMessageReader {
	reader := &controlMessageReader{ctx: ctx, results: make(chan controlReceiveResult, 1)}
	go func() {
		for {
			message, err := stream.Recv()
			select {
			case reader.results <- controlReceiveResult{message: message, err: err}:
			case <-ctx.Done():
				return
			}
			if err != nil {
				return
			}
		}
	}()
	return reader
}

func (r *controlMessageReader) Receive() (*hubv1.HubMessage, error) {
	select {
	case result := <-r.results:
		return result.message, result.err
	case <-r.ctx.Done():
		return nil, r.ctx.Err()
	}
}

const (
	registrationReasonNotRegistered = "GATEWAY_NOT_REGISTERED"
	registrationReasonApprovalWait  = "GATEWAY_APPROVAL_PENDING"
)

// Run keeps one authenticated Gateway control stream open and reconnects with
// bounded backoff. Status is read from the reconciler each time it is sent so
// a failed local configuration immediately appears in the next report.
func Run(ctx context.Context, hubConfig config.HubConfig, privateKeyText string, registrationToken []byte, statusProvider interface{ Status() agent.Snapshot }, applySnapshot SnapshotApplier, onAuthenticated func(), onDisconnected DisconnectHandler) error {
	if ctx == nil {
		return errors.New("Gateway control context is required")
	}
	if statusProvider == nil {
		return errors.New("Gateway status provider is required")
	}
	privateKey, err := wgtypes.ParseKey(privateKeyText)
	if err != nil {
		return fmt.Errorf("parse Gateway control identity key: %w", err)
	}
	defer clear(privateKey[:])
	publicKey := privateKey.PublicKey().String()
	if !hubConfig.TLSEnabled() {
		slog.Warn("Gateway HUB TLS is disabled; control traffic is plaintext and the HUB server is not authenticated", "endpoint", hubConfig.Endpoint)
	}
	backoff := initialRetryDelay
	for {
		err := runSession(ctx, hubConfig, privateKeyText, publicKey, privateKey[:], registrationToken, statusProvider, applySnapshot, onAuthenticated, onDisconnected)
		if ctx.Err() != nil {
			return nil
		}
		if errors.Is(err, errRegistrationCompleted) {
			clear(registrationToken)
			registrationToken = nil
			backoff = initialRetryDelay
			continue
		}
		if err == nil {
			err = errors.New("HUB control stream closed")
		}
		if errors.Is(err, errAccessConfigurationChanged) || errors.Is(err, errAccessRevoked) {
			backoff = initialRetryDelay
		}
		slog.Warn("Gateway HUB control stream ended; reconnecting", "endpoint", hubConfig.Endpoint, "error", err, "retry_in", backoff)
		timer := time.NewTimer(backoff)
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil
		case <-timer.C:
		}
		if errors.Is(err, errAccessConfigurationChanged) || errors.Is(err, errAccessRevoked) {
			backoff = initialRetryDelay
		} else if backoff < maximumRetryDelay/2 {
			backoff *= 2
		} else {
			backoff = maximumRetryDelay
		}
	}
}

func runSession(ctx context.Context, cfg config.HubConfig, privateKeyText, publicKey string, privateKey, registrationToken []byte, statusProvider interface{ Status() agent.Snapshot }, applySnapshot SnapshotApplier, onAuthenticated func(), onDisconnected DisconnectHandler) error {
	credentials, err := newTLSCredentials(cfg)
	if err != nil {
		return err
	}
	connection, err := grpc.NewClient(
		cfg.Endpoint,
		grpc.WithTransportCredentials(credentials),
		grpc.WithKeepaliveParams(keepalive.ClientParameters{Time: 20 * time.Second, Timeout: 10 * time.Second}),
	)
	if err != nil {
		return fmt.Errorf("create HUB gRPC connection: %w", err)
	}
	defer connection.Close()

	streamContext, cancel := context.WithCancel(ctx)
	defer cancel()
	authTimer := time.AfterFunc(clientAuthTimeout, cancel)
	defer authTimer.Stop()
	stream, err := hubv1.NewHubControlClient(connection).Connect(streamContext)
	if err != nil {
		return fmt.Errorf("open HUB control stream: %w", err)
	}
	defer stream.CloseSend()
	if err := stream.Send(&hubv1.AgentMessage{Body: &hubv1.AgentMessage_Hello{Hello: &hubv1.Hello{
		NodeKind:           hubv1.NodeKind_NODE_KIND_GATEWAY,
		WireguardPublicKey: publicKey,
		AppliedConfigHash:  statusProvider.Status().AppliedConfigHash,
	}}}); err != nil {
		return fmt.Errorf("send Gateway hello: %w", err)
	}
	reader := newControlMessageReader(streamContext, stream)

	challengeMessage, err := reader.Receive()
	if err != nil {
		err = applyFailClosedIfRequired(onDisconnected, err)
		switch gatewayAuthorizationReason(err) {
		case registrationReasonNotRegistered:
			if len(registrationToken) == 0 {
				return errors.New("HUB reports that this Gateway is not registered; configure hub.registration_token")
			}
			_ = stream.CloseSend()
			cancel()
			_ = connection.Close()
			slog.Info("HUB reports that this Gateway key is not registered; submitting configured registration credential", "endpoint", cfg.Endpoint)
			if err := registerGatewayAndWait(ctx, cfg, privateKeyText, registrationToken); err != nil {
				return err
			}
			return errRegistrationCompleted
		case registrationReasonApprovalWait:
			slog.Info("Gateway registration is awaiting Management approval", "endpoint", cfg.Endpoint)
		}
		return fmt.Errorf("receive HUB authentication challenge: %w", err)
	}
	challengeWire := challengeMessage.GetX25519AuthChallenge()
	if challengeWire == nil {
		return errors.New("HUB did not send an X25519 authentication challenge")
	}
	nodeID := challengeWire.GetNodeId()
	if nodeID == "" {
		return errors.New("HUB authentication challenge does not identify the resolved Gateway node")
	}
	challenge := hubauth.Challenge{
		ID:            append([]byte(nil), challengeWire.GetChallengeId()...),
		Nonce:         append([]byte(nil), challengeWire.GetNonce()...),
		HubPublicKey:  append([]byte(nil), challengeWire.GetHubEphemeralPublicKey()...),
		ExpiresAt:     time.UnixMilli(challengeWire.GetExpiresAtUnixMillis()),
		HubInstanceID: challengeWire.GetHubInstanceId(),
		NodeID:        nodeID,
	}
	if challengeWire.GetRegistrationChallenge() {
		return errors.New("HUB requested registration during an authenticated Gateway session")
	}
	if len(challenge.ID) != 32 || len(challenge.Nonce) != 32 || len(challenge.HubPublicKey) != 32 || challenge.HubInstanceID == "" || !time.Now().Before(challenge.ExpiresAt) {
		return errors.New("HUB authentication challenge is malformed or expired")
	}
	tlsBinding, err := channelBinding(credentials, cfg, challenge.ID)
	if err != nil {
		return fmt.Errorf("derive HUB control channel binding: %w", err)
	}
	proof, err := hubauth.GatewayProof(privateKey, challenge, tlsBinding)
	clear(tlsBinding)
	if err != nil {
		return fmt.Errorf("create Gateway X25519 proof: %w", err)
	}
	if err := stream.Send(&hubv1.AgentMessage{Body: &hubv1.AgentMessage_X25519AuthProof{X25519AuthProof: &hubv1.X25519AuthProof{
		ChallengeId: challenge.ID,
		Proof:       proof,
	}}}); err != nil {
		clear(proof)
		return fmt.Errorf("send Gateway X25519 proof: %w", err)
	}
	clear(proof)
	welcomeMessage, err := reader.Receive()
	if err != nil {
		return applyFailClosedIfRequired(onDisconnected, fmt.Errorf("receive HUB welcome: %w", err))
	}
	welcome := welcomeMessage.GetWelcome()
	if welcome == nil || welcome.GetSessionId() == "" || welcome.GetHeartbeatIntervalSeconds() == 0 || time.Duration(welcome.GetHeartbeatIntervalSeconds())*time.Second > maximumHeartbeatGap {
		return errors.New("HUB welcome contains invalid session or heartbeat settings")
	}
	authTimer.Stop()
	if welcome.GetConfigSnapshot() != nil {
		if err := applyAndAcknowledgeSnapshot(streamContext, stream, welcome.GetConfigSnapshot(), applySnapshot); err != nil {
			return fmt.Errorf("apply initial HUB Gateway configuration: %w", err)
		}
	}
	slog.Info("Gateway control stream authenticated", "node_id", nodeID, "hub_instance_id", challenge.HubInstanceID, "endpoint", cfg.Endpoint, "heartbeat_interval_seconds", welcome.GetHeartbeatIntervalSeconds())
	if onAuthenticated != nil {
		onAuthenticated()
	}
	err = serveControlMessages(streamContext, stream, reader, welcome.GetHeartbeatIntervalSeconds(), statusProvider, applySnapshot)
	if ctx.Err() == nil && onDisconnected != nil && errors.Is(err, errAccessRevoked) {
		if closeErr := onDisconnected(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("block Gateway dataplane after access revocation: %w", closeErr))
		}
	} else if ctx.Err() == nil && onDisconnected != nil && shouldFailClosedAfterControlError(err) {
		if closeErr := onDisconnected(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("block Gateway dataplane after HUB authorization/configuration failure: %w", closeErr))
		}
	}
	return err
}

func applyFailClosedIfRequired(onDisconnected DisconnectHandler, err error) error {
	if onDisconnected == nil || !shouldFailClosedAfterControlError(err) {
		return err
	}
	if closeErr := onDisconnected(); closeErr != nil {
		return errors.Join(err, fmt.Errorf("block Gateway dataplane after HUB authorization/configuration failure: %w", closeErr))
	}
	return err
}

// A lost control connection permits the last verified configuration to remain
// active. Explicit authorization or protocol failures invalidate that state.
func shouldFailClosedAfterControlError(err error) bool {
	if err == nil || errors.Is(err, io.EOF) {
		return false
	}
	if errors.Is(err, errAccessConfigurationChanged) || errors.Is(err, errAccessRevoked) {
		return false
	}
	grpcStatus, ok := status.FromError(err)
	if !ok {
		return true
	}
	switch grpcStatus.Code() {
	case codes.Unavailable, codes.DeadlineExceeded, codes.Canceled:
		return false
	default:
		return true
	}
}

func gatewayAuthorizationReason(err error) string {
	grpcStatus, ok := status.FromError(err)
	if !ok || grpcStatus.Code() != codes.FailedPrecondition {
		return ""
	}
	for _, detail := range grpcStatus.Details() {
		if info, ok := detail.(*errdetails.ErrorInfo); ok && info.Domain == "velotrust.hub" {
			return info.Reason
		}
	}
	return ""
}

func serveControlMessages(ctx context.Context, stream grpc.BidiStreamingClient[hubv1.AgentMessage, hubv1.HubMessage], reader *controlMessageReader, intervalSeconds uint32, statusProvider interface{ Status() agent.Snapshot }, applySnapshot SnapshotApplier) error {
	interval := time.Duration(intervalSeconds) * time.Second
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	var heartbeatSequence, statusSequence uint64
	configuration, err := sendRuntimeStatus(ctx, stream, reader, statusProvider, &statusSequence)
	if err != nil {
		return err
	}
	if configuration != nil {
		if err := applyAndAcknowledgeSnapshot(ctx, stream, configuration, applySnapshot); err != nil {
			return fmt.Errorf("apply updated HUB Gateway configuration: %w", err)
		}
	}
	for {
		select {
		case <-ctx.Done():
			return nil
		case received := <-reader.results:
			if received.err != nil {
				return fmt.Errorf("receive unsolicited HUB control message: %w", received.err)
			}
			if reauth := received.message.GetReauthenticate(); reauth != nil {
				return accessChangeError(reauth)
			}
			return errors.New("HUB sent an unexpected control message without a pending Gateway request")
		case <-ticker.C:
			heartbeatSequence++
			if err := stream.Send(&hubv1.AgentMessage{Body: &hubv1.AgentMessage_Heartbeat{Heartbeat: &hubv1.Heartbeat{Sequence: heartbeatSequence}}}); err != nil {
				return fmt.Errorf("send Gateway heartbeat: %w", err)
			}
			ackMessage, err := reader.Receive()
			if err != nil {
				return fmt.Errorf("receive Gateway heartbeat acknowledgment: %w", err)
			}
			if reauth := ackMessage.GetReauthenticate(); reauth != nil {
				return accessChangeError(reauth)
			}
			ack := ackMessage.GetHeartbeatAck()
			if ack == nil || ack.GetSequence() != heartbeatSequence {
				return errors.New("HUB heartbeat acknowledgment does not match the request")
			}
			slog.Debug("Gateway heartbeat acknowledged", "sequence", heartbeatSequence, "server_time_unix", ack.GetServerTimeUnix())
			configuration, err := sendRuntimeStatus(ctx, stream, reader, statusProvider, &statusSequence)
			if err != nil {
				return err
			}
			if configuration != nil {
				if err := applyAndAcknowledgeSnapshot(ctx, stream, configuration, applySnapshot); err != nil {
					return fmt.Errorf("apply updated HUB Gateway configuration: %w", err)
				}
			}
		}
	}
}

func sendRuntimeStatus(ctx context.Context, stream grpc.BidiStreamingClient[hubv1.AgentMessage, hubv1.HubMessage], reader *controlMessageReader, statusProvider interface{ Status() agent.Snapshot }, sequence *uint64) (*hubv1.ConfigSnapshot, error) {
	if err := ctx.Err(); err != nil {
		return nil, nil
	}
	if sequence == nil {
		return nil, errors.New("Gateway runtime status sequence is required")
	}
	*sequence = *sequence + 1
	snapshot := statusProvider.Status()
	wireGuardStatus, healthStatus, errorCode := runtimeStatusValues(snapshot)
	if err := stream.Send(&hubv1.AgentMessage{Body: &hubv1.AgentMessage_RuntimeStatus{RuntimeStatus: &hubv1.RuntimeStatus{
		WireguardStatus:         wireGuardStatus,
		HealthStatus:            healthStatus,
		AppliedConfigHash:       snapshot.AppliedConfigHash,
		LastErrorCode:           errorCode,
		AgentVersion:            buildinfo.Version,
		Sequence:                *sequence,
		ConnectedPeerCountKnown: false,
	}}}); err != nil {
		return nil, fmt.Errorf("send Gateway runtime status: %w", err)
	}
	ackMessage, err := reader.Receive()
	if err != nil {
		return nil, fmt.Errorf("receive Gateway runtime status acknowledgment: %w", err)
	}
	if reauth := ackMessage.GetReauthenticate(); reauth != nil {
		return nil, accessChangeError(reauth)
	}
	ack := ackMessage.GetRuntimeStatusAck()
	if ack == nil || ack.GetSequence() != *sequence {
		return nil, errors.New("HUB runtime status acknowledgment does not match the request")
	}
	slog.Debug("Gateway runtime status acknowledged", "sequence", *sequence, "wireguard_status", wireGuardStatus, "health_status", healthStatus, "connected_peer_count_known", false)
	return ack.GetConfigSnapshot(), nil
}

func accessChangeError(message *hubv1.Reauthenticate) error {
	if message != nil && message.GetReasonCode() == "ACCESS_REVOKED" {
		return errAccessRevoked
	}
	return errAccessConfigurationChanged
}

func applyAndAcknowledgeSnapshot(ctx context.Context, stream grpc.BidiStreamingClient[hubv1.AgentMessage, hubv1.HubMessage], snapshot *hubv1.ConfigSnapshot, applySnapshot SnapshotApplier) error {
	if snapshot == nil || snapshot.GetConfigHash() == "" || len(snapshot.GetPayload()) == 0 {
		return errors.New("HUB configuration snapshot is incomplete")
	}
	contentHash, err := configpayload.Hash(snapshot.GetPayload())
	if err != nil || !strings.EqualFold(contentHash, snapshot.GetConfigHash()) {
		return errors.New("HUB configuration snapshot content hash is invalid")
	}
	if applySnapshot == nil {
		return errors.New("Gateway configuration snapshot handler is unavailable")
	}
	applyErr := applySnapshot(ctx, snapshot)
	receipt := &hubv1.ConfigApplied{
		TaskId:     snapshot.GetTaskId(),
		ConfigHash: snapshot.GetConfigHash(),
		Success:    applyErr == nil,
	}
	if applyErr != nil {
		receipt.ErrorCode = "CONFIG_APPLY_FAILED"
	}
	if err := stream.Send(&hubv1.AgentMessage{Body: &hubv1.AgentMessage_ConfigApplied{ConfigApplied: receipt}}); err != nil {
		if applyErr != nil {
			return errors.Join(applyErr, fmt.Errorf("send Gateway configuration failure receipt: %w", err))
		}
		return fmt.Errorf("send Gateway configuration receipt: %w", err)
	}
	if applyErr != nil {
		return applyErr
	}
	slog.Info("Gateway configuration snapshot applied", "config_hash", snapshot.GetConfigHash(), "task_id", snapshot.GetTaskId())
	return nil
}

func runtimeStatusValues(snapshot agent.Snapshot) (wireGuardStatus, healthStatus, errorCode string) {
	switch snapshot.Phase {
	case agent.PhaseReady:
		return "up", "healthy", ""
	case agent.PhasePending:
		return "starting", "unknown", ""
	case agent.PhaseDegraded:
		return "error", "degraded", "gateway_dataplane_degraded"
	case agent.PhaseStopped:
		return "down", "unhealthy", ""
	default:
		return "unknown", "unknown", ""
	}
}

func newTLSCredentials(cfg config.HubConfig) (*tlsStateCredentials, error) {
	if !cfg.TLSEnabled() {
		return &tlsStateCredentials{inner: insecure.NewCredentials(), shared: &tlsStateHolder{}}, nil
	}
	tlsConfig := &tls.Config{MinVersion: tls.VersionTLS12, ServerName: cfg.ServerName}
	if cfg.CAFile != "" {
		pemData, err := os.ReadFile(cfg.CAFile)
		if err != nil {
			return nil, fmt.Errorf("read HUB CA certificate: %w", err)
		}
		pool, err := loadCertPool(pemData)
		if err != nil {
			return nil, fmt.Errorf("load HUB CA certificate: %w", err)
		}
		tlsConfig.RootCAs = pool
	}
	return newCapturingTLSCredentials(tlsConfig), nil
}

func channelBinding(credentials *tlsStateCredentials, cfg config.HubConfig, challengeID []byte) ([]byte, error) {
	if !cfg.TLSEnabled() {
		return hubauth.PlaintextChannelBinding(challengeID)
	}
	tlsState, ok := credentials.ConnectionState()
	if !ok {
		return nil, errors.New("TLS state is unavailable for HUB channel binding")
	}
	return hubauth.TLSExporterBinding(tlsState, challengeID)
}

func loadCertPool(pemData []byte) (*x509.CertPool, error) {
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(pemData) {
		return nil, errors.New("CA file contains no PEM certificates")
	}
	return pool, nil
}
