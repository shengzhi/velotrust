package control

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
	"google.golang.org/grpc"
	"google.golang.org/grpc/keepalive"

	"velotrust/gateway/config"
	"velotrust/pkg/hubauth"
	hubv1 "velotrust/shared/gen/go/hub/v1"
)

// registerGatewayAndWait submits a one-time credential and holds the control
// stream until HUB confirms that the Gateway is active.
func registerGatewayAndWait(ctx context.Context, hubConfig config.HubConfig, privateKeyText string, token []byte) error {
	if ctx == nil {
		return errors.New("Gateway registration context is required")
	}
	if hubConfig.Endpoint == "" {
		return errors.New("HUB endpoint is required")
	}
	if !hubConfig.TLSEnabled() {
		slog.Warn("Gateway HUB TLS is disabled for one-time registration; registration data is plaintext", "endpoint", hubConfig.Endpoint)
	}
	if err := validateRegistrationToken(token); err != nil {
		return err
	}
	privateKey, err := wgtypes.ParseKey(privateKeyText)
	if err != nil {
		return fmt.Errorf("parse Gateway registration identity key: %w", err)
	}
	defer clear(privateKey[:])

	streamContext, cancel := context.WithCancel(ctx)
	defer cancel()
	credentials, err := newTLSCredentials(hubConfig)
	if err != nil {
		return err
	}
	connection, err := grpc.NewClient(
		hubConfig.Endpoint,
		grpc.WithTransportCredentials(credentials),
		grpc.WithKeepaliveParams(keepalive.ClientParameters{Time: 20 * time.Second, Timeout: 10 * time.Second}),
	)
	if err != nil {
		return fmt.Errorf("create HUB gRPC connection: %w", err)
	}
	defer connection.Close()

	stream, err := hubv1.NewHubControlClient(connection).Connect(streamContext)
	if err != nil {
		return fmt.Errorf("open HUB registration stream: %w", err)
	}
	defer stream.CloseSend()
	hello := &hubv1.Hello{
		NodeKind:           hubv1.NodeKind_NODE_KIND_GATEWAY,
		WireguardPublicKey: privateKey.PublicKey().String(),
		RegistrationToken:  string(token),
	}
	authTimer := time.AfterFunc(clientAuthTimeout, cancel)
	defer authTimer.Stop()
	if err := stream.Send(&hubv1.AgentMessage{Body: &hubv1.AgentMessage_Hello{Hello: hello}}); err != nil {
		return fmt.Errorf("send Gateway registration hello: %w", err)
	}

	challengeMessage, err := stream.Recv()
	if err != nil {
		return fmt.Errorf("receive HUB registration challenge: %w", err)
	}
	challengeWire := challengeMessage.GetX25519AuthChallenge()
	if challengeWire == nil || !challengeWire.GetRegistrationChallenge() {
		return errors.New("HUB did not send an X25519 registration challenge")
	}
	nodeID := challengeWire.GetNodeId()
	if nodeID == "" {
		return errors.New("HUB registration challenge does not identify the resolved Gateway node")
	}
	challenge := hubauth.Challenge{
		ID:            append([]byte(nil), challengeWire.GetChallengeId()...),
		Nonce:         append([]byte(nil), challengeWire.GetNonce()...),
		HubPublicKey:  append([]byte(nil), challengeWire.GetHubEphemeralPublicKey()...),
		ExpiresAt:     time.UnixMilli(challengeWire.GetExpiresAtUnixMillis()),
		HubInstanceID: challengeWire.GetHubInstanceId(),
		NodeID:        nodeID,
	}
	if len(challenge.ID) != 32 || len(challenge.Nonce) != 32 || len(challenge.HubPublicKey) != 32 || challenge.HubInstanceID == "" || !time.Now().Before(challenge.ExpiresAt) {
		return errors.New("HUB registration challenge is malformed or expired")
	}
	tlsBinding, err := channelBinding(credentials, hubConfig, challenge.ID)
	if err != nil {
		return fmt.Errorf("derive HUB registration channel binding: %w", err)
	}
	proof, err := hubauth.GatewayRegistrationProof(privateKey[:], challenge, tlsBinding)
	clear(tlsBinding)
	if err != nil {
		return fmt.Errorf("create Gateway X25519 registration proof: %w", err)
	}
	if err := stream.Send(&hubv1.AgentMessage{Body: &hubv1.AgentMessage_X25519AuthProof{X25519AuthProof: &hubv1.X25519AuthProof{
		ChallengeId: challenge.ID,
		Proof:       proof,
	}}}); err != nil {
		clear(proof)
		return fmt.Errorf("send Gateway X25519 proof: %w", err)
	}
	clear(proof)
	hello.RegistrationToken = ""
	authTimer.Stop()

	for {
		resultMessage, err := stream.Recv()
		if err != nil {
			return fmt.Errorf("receive Gateway registration result: %w", err)
		}
		registration := resultMessage.GetGatewayRegistrationResult()
		if registration == nil {
			return errors.New("HUB returned an unexpected response to Gateway registration")
		}
		if !registration.GetRegistered() {
			return fmt.Errorf("HUB rejected Gateway registration (node_status=%q)", registration.GetNodeStatus())
		}
		if registration.GetNodeStatus() == "pending" {
			clear(token)
			slog.Info("Gateway registration is awaiting Management approval", "node_id", nodeID)
			continue
		}
		if registration.GetNodeStatus() != "active" {
			clear(token)
			return fmt.Errorf("Gateway registration is not authorized (node_status=%q)", registration.GetNodeStatus())
		}
		clear(token)
		slog.Info("Gateway registration approved; restarting HUB identity authentication", "node_id", nodeID)
		return nil
	}
}

func validateRegistrationToken(token []byte) error {
	const prefix = "vtreg_"
	if len(token) > 128 || !bytes.HasPrefix(token, []byte(prefix)) {
		return errors.New("registration token has an invalid format")
	}
	secret := make([]byte, 32)
	decoded, err := base64.RawURLEncoding.Decode(secret, token[len(prefix):])
	if err != nil || decoded != len(secret) {
		clear(secret)
		return errors.New("registration token has an invalid format")
	}
	clear(secret)
	return nil
}
