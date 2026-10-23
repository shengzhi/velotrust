// Package control implements the Client Agent's HUB-only control stream.
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
	"path/filepath"
	"strings"
	"time"

	clientconfig "velotrust/client/internal/config"
	winplatform "velotrust/client/system/windows"
	"velotrust/pkg/hubauth"
	"velotrust/shared/configpayload"
	hubv1 "velotrust/shared/gen/go/hub/v1"

	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/status"
)

const (
	initialRetryDelay   = time.Second
	maximumRetryDelay   = 30 * time.Second
	clientAuthTimeout   = 45 * time.Second
	maximumHeartbeatGap = time.Hour
)

var (
	errAccessConfigurationChanged = errors.New("HUB requested a Client configuration refresh")
	errAccessRevoked              = errors.New("HUB revoked Client authorization")
)

type RuntimeStatus struct {
	AppliedConfigHash string
	WireGuardStatus   string
	HealthStatus      string
	LastErrorCode     string
}

type StatusProvider interface{ Status() RuntimeStatus }
type SnapshotApplier func(context.Context, *hubv1.ConfigSnapshot) error
type DisconnectHandler func() error

type LoginCredentials struct {
	Username   string
	Password   []byte
	DeviceName string
	Platform   string
}

type receivedMessage struct {
	message *hubv1.HubMessage
	err     error
}

type messageReader struct {
	ctx     context.Context
	results chan receivedMessage
}

func newMessageReader(ctx context.Context, stream grpc.BidiStreamingClient[hubv1.AgentMessage, hubv1.HubMessage]) *messageReader {
	reader := &messageReader{ctx: ctx, results: make(chan receivedMessage, 1)}
	go func() {
		for {
			message, err := stream.Recv()
			select {
			case reader.results <- receivedMessage{message: message, err: err}:
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

func (r *messageReader) Receive() (*hubv1.HubMessage, error) {
	select {
	case result := <-r.results:
		return result.message, result.err
	case <-r.ctx.Done():
		return nil, r.ctx.Err()
	}
}

// Run authenticates the employee and Client device to HUB, registers an unseen
// device on first login, keeps the control stream alive, and applies HUB
// configuration snapshots. It never opens a Management API connection.
func Run(ctx context.Context, hub clientconfig.HubConfig, configPath, privateKeyFile, agentVersion string, login LoginCredentials, statusProvider StatusProvider, apply SnapshotApplier, onAuthenticated func(), onDisconnected DisconnectHandler) error {
	if ctx == nil || statusProvider == nil || apply == nil {
		return errors.New("Client HUB context, status provider, and configuration handler are required")
	}
	defer clear(login.Password)
	login.Username = strings.TrimSpace(login.Username)
	if login.Username == "" || len(login.Password) < 8 || len(login.Password) > 1024 || strings.TrimSpace(login.DeviceName) == "" || strings.TrimSpace(login.Platform) == "" {
		return errors.New("employee username, password, device name, and platform are required")
	}
	privateKeyPath := privateKeyFile
	if !filepath.IsAbs(privateKeyPath) {
		privateKeyPath = filepath.Join(filepath.Dir(configPath), privateKeyPath)
	}
	privateKey, err := winplatform.LoadWireGuardPrivateKey(privateKeyPath)
	if err != nil {
		return fmt.Errorf("load Client HUB identity key: %w", err)
	}
	defer clear(privateKey[:])
	if !hub.TLSEnabled() {
		slog.Warn("Client HUB TLS is disabled; control traffic is plaintext and HUB server identity is not authenticated", "endpoint", hub.Endpoint)
	}
	backoff := initialRetryDelay
	applyForAgent := func(_ context.Context, snapshot *hubv1.ConfigSnapshot) error {
		return apply(ctx, snapshot)
	}
	for {
		err := runSession(ctx, hub, configPath, agentVersion, privateKey, login, statusProvider, applyForAgent, onAuthenticated, onDisconnected)
		if ctx.Err() != nil {
			return nil
		}
		if onDisconnected != nil && shouldFailClosed(err) {
			if closeErr := onDisconnected(); closeErr != nil {
				err = errors.Join(err, fmt.Errorf("stop Client tunnel after HUB authorization failure: %w", closeErr))
			}
		}
		if reason := authorizationReason(err); reason == "CLIENT_LOGIN_FAILED" {
			return errors.New("HUB rejected the employee login; verify the username, password, and account status")
		}
		if reason := authorizationReason(err); reason == "CLIENT_APPROVAL_PENDING" {
			slog.Info("Client device registration is awaiting Management approval", "endpoint", hub.Endpoint, "retry_in", backoff)
		} else if err == nil {
			err = errors.New("HUB control stream closed")
		}
		if errors.Is(err, errAccessConfigurationChanged) || errors.Is(err, errAccessRevoked) {
			backoff = initialRetryDelay
		}
		slog.Warn("Client HUB control stream ended; reconnecting", "endpoint", hub.Endpoint, "error", err, "retry_in", backoff)
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

func runSession(ctx context.Context, hub clientconfig.HubConfig, configPath, agentVersion string, privateKey wgtypes.Key, login LoginCredentials, statusProvider StatusProvider, apply SnapshotApplier, onAuthenticated func(), onDisconnected DisconnectHandler) error {
	if hub.CAFile != "" && !filepath.IsAbs(hub.CAFile) {
		hub.CAFile = filepath.Join(filepath.Dir(configPath), hub.CAFile)
	}
	transport, err := newTransportCredentials(hub)
	if err != nil {
		return err
	}
	connection, err := grpc.NewClient(hub.Endpoint,
		grpc.WithTransportCredentials(transport),
		grpc.WithKeepaliveParams(keepalive.ClientParameters{Time: 20 * time.Second, Timeout: 10 * time.Second}),
	)
	if err != nil {
		return fmt.Errorf("create HUB gRPC connection: %w", err)
	}
	defer connection.Close()
	streamCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	authTimer := time.AfterFunc(clientAuthTimeout, cancel)
	defer authTimer.Stop()
	stream, err := hubv1.NewHubControlClient(connection).Connect(streamCtx)
	if err != nil {
		return fmt.Errorf("open HUB control stream: %w", err)
	}
	defer stream.CloseSend()
	hello := &hubv1.Hello{
		NodeKind: hubv1.NodeKind_NODE_KIND_CLIENT, WireguardPublicKey: privateKey.PublicKey().String(),
		AppliedConfigHash: statusProvider.Status().AppliedConfigHash,
		ClientUsername:    login.Username, ClientPassword: string(login.Password),
		ClientDeviceName: login.DeviceName, ClientPlatform: login.Platform, AgentVersion: agentVersion,
	}
	if err := stream.Send(&hubv1.AgentMessage{Body: &hubv1.AgentMessage_Hello{Hello: hello}}); err != nil {
		return fmt.Errorf("send Client HUB hello: %w", err)
	}
	reader := newMessageReader(streamCtx, stream)
	first, err := reader.Receive()
	if err != nil {
		return err
	}
	if registration := first.GetClientRegistrationResult(); registration != nil {
		if !registration.GetRegistered() {
			return errors.New("HUB rejected employee device registration")
		}
		slog.Info("Client device registered with HUB", "node_status", registration.GetNodeStatus())
		first, err = reader.Receive()
		if err != nil {
			return err
		}
	}
	authenticatedNodeID := ""
	if challengeWire := first.GetX25519AuthChallenge(); challengeWire != nil {
		challenge, err := challengeFromWire(challengeWire)
		if err != nil {
			return err
		}
		authenticatedNodeID = challenge.NodeID
		binding, err := channelBinding(transport, hub, challenge.ID)
		if err != nil {
			return fmt.Errorf("derive HUB control channel binding: %w", err)
		}
		proof, err := hubauth.ClientProof(privateKey[:], challenge, binding)
		clear(binding)
		if err != nil {
			return fmt.Errorf("create Client X25519 proof: %w", err)
		}
		if err := stream.Send(&hubv1.AgentMessage{Body: &hubv1.AgentMessage_X25519AuthProof{X25519AuthProof: &hubv1.X25519AuthProof{ChallengeId: challenge.ID, Proof: proof}}}); err != nil {
			clear(proof)
			return fmt.Errorf("send Client X25519 proof: %w", err)
		}
		clear(proof)
		first, err = reader.Receive()
		if err != nil {
			return err
		}
	}
	welcome := first.GetWelcome()
	if welcome == nil || welcome.GetSessionId() == "" || welcome.GetHeartbeatIntervalSeconds() == 0 || time.Duration(welcome.GetHeartbeatIntervalSeconds())*time.Second > maximumHeartbeatGap {
		return errors.New("HUB welcome contains invalid session or heartbeat settings")
	}
	authTimer.Stop()
	if welcome.GetConfigSnapshot() != nil {
		if err := applyAndAcknowledgeSnapshot(streamCtx, stream, welcome.GetConfigSnapshot(), apply); err != nil {
			return fmt.Errorf("apply initial HUB Client configuration: %w", err)
		}
	} else if statusProvider.Status().AppliedConfigHash == "" {
		slog.Info("Client authenticated; waiting for HUB tunnel configuration; address assignment or Gateway setup may still be pending", "node_id", authenticatedNodeID, "endpoint", hub.Endpoint)
	}
	slog.Info("Client HUB control stream authenticated", "node_id", authenticatedNodeID, "endpoint", hub.Endpoint, "heartbeat_interval_seconds", welcome.GetHeartbeatIntervalSeconds())
	if onAuthenticated != nil {
		onAuthenticated()
	}
	err = serveControlMessages(streamCtx, stream, reader, welcome.GetHeartbeatIntervalSeconds(), agentVersion, statusProvider, apply)
	if ctx.Err() == nil && onDisconnected != nil && (shouldFailClosed(err) || errors.Is(err, errAccessRevoked)) {
		if closeErr := onDisconnected(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("stop Client tunnel after HUB authorization failure: %w", closeErr))
		}
	}
	return err
}

func challengeFromWire(wire *hubv1.X25519AuthChallenge) (hubauth.Challenge, error) {
	challenge := hubauth.Challenge{
		ID: append([]byte(nil), wire.GetChallengeId()...), Nonce: append([]byte(nil), wire.GetNonce()...),
		HubPublicKey: append([]byte(nil), wire.GetHubEphemeralPublicKey()...),
		ExpiresAt:    time.UnixMilli(wire.GetExpiresAtUnixMillis()), HubInstanceID: wire.GetHubInstanceId(), NodeID: wire.GetNodeId(),
	}
	if len(challenge.ID) != 32 || len(challenge.Nonce) != 32 || len(challenge.HubPublicKey) != 32 || challenge.NodeID == "" || challenge.HubInstanceID == "" || !time.Now().Before(challenge.ExpiresAt) {
		return hubauth.Challenge{}, errors.New("HUB X25519 challenge is malformed or expired")
	}
	return challenge, nil
}

func serveControlMessages(ctx context.Context, stream grpc.BidiStreamingClient[hubv1.AgentMessage, hubv1.HubMessage], reader *messageReader, intervalSeconds uint32, agentVersion string, statusProvider StatusProvider, apply SnapshotApplier) error {
	interval := time.Duration(intervalSeconds) * time.Second
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	var heartbeatSequence, statusSequence uint64
	configuration, err := sendRuntimeStatus(ctx, stream, reader, agentVersion, statusProvider, &statusSequence)
	if err != nil {
		return err
	}
	if configuration != nil {
		if err := applyAndAcknowledgeSnapshot(ctx, stream, configuration, apply); err != nil {
			return fmt.Errorf("apply updated HUB Client configuration: %w", err)
		}
	}
	for {
		select {
		case <-ctx.Done():
			return nil
		case message := <-reader.results:
			if message.err != nil {
				return fmt.Errorf("receive unsolicited HUB control message: %w", message.err)
			}
			if reauth := message.message.GetReauthenticate(); reauth != nil {
				return accessChangeError(reauth)
			}
			return errors.New("HUB sent an unexpected Client control message")
		case <-ticker.C:
			heartbeatSequence++
			if err := stream.Send(&hubv1.AgentMessage{Body: &hubv1.AgentMessage_Heartbeat{Heartbeat: &hubv1.Heartbeat{Sequence: heartbeatSequence}}}); err != nil {
				return fmt.Errorf("send Client heartbeat: %w", err)
			}
			ackMessage, err := reader.Receive()
			if err != nil {
				return fmt.Errorf("receive Client heartbeat acknowledgment: %w", err)
			}
			if reauth := ackMessage.GetReauthenticate(); reauth != nil {
				return accessChangeError(reauth)
			}
			ack := ackMessage.GetHeartbeatAck()
			if ack == nil || ack.GetSequence() != heartbeatSequence {
				return errors.New("HUB heartbeat acknowledgment does not match the request")
			}
			configuration, err := sendRuntimeStatus(ctx, stream, reader, agentVersion, statusProvider, &statusSequence)
			if err != nil {
				return err
			}
			if configuration != nil {
				if err := applyAndAcknowledgeSnapshot(ctx, stream, configuration, apply); err != nil {
					return fmt.Errorf("apply updated HUB Client configuration: %w", err)
				}
			}
		}
	}
}

func sendRuntimeStatus(ctx context.Context, stream grpc.BidiStreamingClient[hubv1.AgentMessage, hubv1.HubMessage], reader *messageReader, agentVersion string, provider StatusProvider, sequence *uint64) (*hubv1.ConfigSnapshot, error) {
	if ctx.Err() != nil {
		return nil, nil
	}
	*sequence++
	state := provider.Status()
	wireguard := state.WireGuardStatus
	if wireguard == "" {
		wireguard = "unknown"
	}
	health := state.HealthStatus
	if health == "" {
		health = "unknown"
	}
	if err := stream.Send(&hubv1.AgentMessage{Body: &hubv1.AgentMessage_RuntimeStatus{RuntimeStatus: &hubv1.RuntimeStatus{
		AppliedConfigHash: state.AppliedConfigHash, WireguardStatus: wireguard, HealthStatus: health,
		LastErrorCode: state.LastErrorCode, AgentVersion: agentVersion, Sequence: *sequence,
		ConnectedPeerCountKnown: false,
	}}}); err != nil {
		return nil, fmt.Errorf("send Client runtime status: %w", err)
	}
	message, err := reader.Receive()
	if err != nil {
		return nil, fmt.Errorf("receive Client runtime status acknowledgment: %w", err)
	}
	if reauth := message.GetReauthenticate(); reauth != nil {
		return nil, accessChangeError(reauth)
	}
	ack := message.GetRuntimeStatusAck()
	if ack == nil || ack.GetSequence() != *sequence {
		return nil, errors.New("HUB runtime status acknowledgment does not match the request")
	}
	return ack.GetConfigSnapshot(), nil
}

func applyAndAcknowledgeSnapshot(ctx context.Context, stream grpc.BidiStreamingClient[hubv1.AgentMessage, hubv1.HubMessage], snapshot *hubv1.ConfigSnapshot, apply SnapshotApplier) error {
	if snapshot == nil || snapshot.GetConfigHash() == "" || len(snapshot.GetPayload()) == 0 {
		return errors.New("HUB Client configuration snapshot is incomplete")
	}
	hash, err := configpayload.Hash(snapshot.GetPayload())
	if err != nil || !strings.EqualFold(hash, snapshot.GetConfigHash()) {
		return errors.New("HUB Client configuration snapshot hash is invalid")
	}
	applyErr := apply(ctx, snapshot)
	receipt := &hubv1.ConfigApplied{TaskId: snapshot.GetTaskId(), ConfigHash: snapshot.GetConfigHash(), Success: applyErr == nil}
	if applyErr != nil {
		receipt.ErrorCode = "CONFIG_APPLY_FAILED"
		receipt.ErrorMessage = applyErr.Error()
	}
	if err := stream.Send(&hubv1.AgentMessage{Body: &hubv1.AgentMessage_ConfigApplied{ConfigApplied: receipt}}); err != nil {
		if applyErr != nil {
			return errors.Join(applyErr, fmt.Errorf("send Client configuration failure receipt: %w", err))
		}
		return fmt.Errorf("send Client configuration receipt: %w", err)
	}
	if applyErr != nil {
		return applyErr
	}
	slog.Info("Client configuration snapshot applied", "config_hash", snapshot.GetConfigHash(), "task_id", snapshot.GetTaskId())
	return nil
}

func accessChangeError(message *hubv1.Reauthenticate) error {
	if message != nil && message.GetReasonCode() == "ACCESS_REVOKED" {
		return errAccessRevoked
	}
	return errAccessConfigurationChanged
}

func shouldFailClosed(err error) bool {
	if err == nil || errors.Is(err, io.EOF) || errors.Is(err, errAccessConfigurationChanged) {
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

func authorizationReason(err error) string {
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

func newTransportCredentials(hub clientconfig.HubConfig) (credentials.TransportCredentials, error) {
	if !hub.TLSEnabled() {
		return insecure.NewCredentials(), nil
	}
	config := &tls.Config{MinVersion: tls.VersionTLS12, ServerName: hub.ServerName}
	if hub.CAFile != "" {
		data, err := os.ReadFile(hub.CAFile)
		if err != nil {
			return nil, fmt.Errorf("read HUB CA certificate: %w", err)
		}
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM(data) {
			return nil, errors.New("HUB CA file contains no PEM certificates")
		}
		config.RootCAs = pool
	}
	return &tlsStateCredentials{inner: credentials.NewTLS(config), state: &tlsStateHolder{}}, nil
}

func channelBinding(transport credentials.TransportCredentials, hub clientconfig.HubConfig, challengeID []byte) ([]byte, error) {
	if !hub.TLSEnabled() {
		return hubauth.PlaintextChannelBinding(challengeID)
	}
	credentials, ok := transport.(*tlsStateCredentials)
	if !ok {
		return nil, errors.New("captured TLS transport state is unavailable")
	}
	credentialsState, ok := credentials.ConnectionState()
	if !ok {
		return nil, errors.New("TLS state is unavailable for HUB channel binding")
	}
	return hubauth.TLSExporterBinding(credentialsState, challengeID)
}
