package hubauth

import (
	"crypto/ecdh"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"crypto/tls"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"time"

	"golang.org/x/crypto/hkdf"
)

const (
	x25519AuthDomain               = "velotrust/hub/gateway-x25519-auth/v1"
	x25519RegistrationDomain       = "velotrust/hub/gateway-x25519-registration/v1"
	x25519ClientAuthDomain         = "velotrust/hub/client-x25519-auth/v1"
	x25519ClientRegistrationDomain = "velotrust/hub/client-x25519-registration/v1"
	tlsExporterLabel               = "EXPORTER-VeloTrust-HUB-Gateway-X25519-v1"
	plaintextBindingDomain         = "velotrust/hub/plaintext-channel-binding/v1"
	challengeIDSize                = 32
	challengeNonceSize             = 32
	proofSize                      = sha256.Size
)

// TLSExporterBinding derives a per-challenge channel binding. Both gRPC peers
// must call this with the TLS state of the same authenticated connection.
func TLSExporterBinding(state tls.ConnectionState, challengeID []byte) ([]byte, error) {
	if len(challengeID) != challengeIDSize {
		return nil, errors.New("authentication challenge ID must contain 32 bytes")
	}
	binding, err := state.ExportKeyingMaterial(tlsExporterLabel, challengeID, sha256.Size)
	if err != nil {
		return nil, fmt.Errorf("derive TLS exporter binding: %w", err)
	}
	return binding, nil
}

// PlaintextChannelBinding domain-separates an explicitly unencrypted control
// stream from TLS-bound authentication. It does not encrypt or authenticate
// the transport and must not be treated as a replacement for TLS.
func PlaintextChannelBinding(challengeID []byte) ([]byte, error) {
	if len(challengeID) != challengeIDSize {
		return nil, errors.New("authentication challenge ID must contain 32 bytes")
	}
	hash := sha256.New()
	_, _ = hash.Write([]byte(plaintextBindingDomain))
	_, _ = hash.Write(challengeID)
	return hash.Sum(nil), nil
}

// Challenge contains the public, single-use part of a Gateway authentication
// exchange. The channel binding is deliberately not carried on the wire; both
// peers derive it from the TLS connection or the explicitly selected
// plaintext challenge context.
type Challenge struct {
	ID            []byte
	Nonce         []byte
	HubPublicKey  []byte
	ExpiresAt     time.Time
	HubInstanceID string
	NodeID        string
}

// PendingChallenge retains the proof key only for one stream's short-lived
// challenge. Verify consumes the challenge whether the supplied proof is valid
// or not, preventing retries against the same challenge.
type PendingChallenge struct {
	challenge  Challenge
	proofKey   [sha256.Size]byte
	transcript []byte
	consumed   bool
}

// NewGatewayChallenge creates a fresh HUB X25519 key and derives the one-use
// proof key using the registered Gateway public key and transport binding.
func NewGatewayChallenge(nodeID, hubInstanceID string, challengeID, gatewayPublicKey, channelBinding []byte, now time.Time, ttl time.Duration) (Challenge, *PendingChallenge, error) {
	return newGatewayChallenge(x25519AuthDomain, nodeID, hubInstanceID, challengeID, gatewayPublicKey, channelBinding, now, ttl)
}

// NewGatewayRegistrationChallenge creates a registration-specific proof
// challenge. Its transcript is domain-separated from ordinary reconnect auth.
func NewGatewayRegistrationChallenge(nodeID, hubInstanceID string, challengeID, gatewayPublicKey, channelBinding []byte, now time.Time, ttl time.Duration) (Challenge, *PendingChallenge, error) {
	return newGatewayChallenge(x25519RegistrationDomain, nodeID, hubInstanceID, challengeID, gatewayPublicKey, channelBinding, now, ttl)
}

// NewClientChallenge creates a Client Agent authentication challenge. Client
// proofs are domain-separated from Gateway proofs even when the same X25519
// key material is accidentally reused.
func NewClientChallenge(nodeID, hubInstanceID string, challengeID, clientPublicKey, channelBinding []byte, now time.Time, ttl time.Duration) (Challenge, *PendingChallenge, error) {
	return newGatewayChallenge(x25519ClientAuthDomain, nodeID, hubInstanceID, challengeID, clientPublicKey, channelBinding, now, ttl)
}

// NewClientRegistrationChallenge creates a Client enrollment-specific proof
// challenge that cannot be replayed as ordinary control-stream auth.
func NewClientRegistrationChallenge(nodeID, hubInstanceID string, challengeID, clientPublicKey, channelBinding []byte, now time.Time, ttl time.Duration) (Challenge, *PendingChallenge, error) {
	return newGatewayChallenge(x25519ClientRegistrationDomain, nodeID, hubInstanceID, challengeID, clientPublicKey, channelBinding, now, ttl)
}

func newGatewayChallenge(domain, nodeID, hubInstanceID string, challengeID, gatewayPublicKey, channelBinding []byte, now time.Time, ttl time.Duration) (Challenge, *PendingChallenge, error) {
	if nodeID == "" || hubInstanceID == "" {
		return Challenge{}, nil, errors.New("node and HUB instance identifiers are required")
	}
	if len(channelBinding) < 32 {
		return Challenge{}, nil, errors.New("channel binding must contain at least 32 bytes")
	}
	if len(challengeID) != challengeIDSize {
		return Challenge{}, nil, errors.New("authentication challenge ID must contain 32 bytes")
	}
	if ttl <= 0 {
		return Challenge{}, nil, errors.New("authentication challenge TTL must be positive")
	}
	curve := ecdh.X25519()
	peerPublic, err := curve.NewPublicKey(gatewayPublicKey)
	if err != nil {
		return Challenge{}, nil, fmt.Errorf("parse registered Gateway X25519 public key: %w", err)
	}
	hubPrivate, err := curve.GenerateKey(rand.Reader)
	if err != nil {
		return Challenge{}, nil, fmt.Errorf("generate HUB ephemeral X25519 key: %w", err)
	}
	id := append([]byte(nil), challengeID...)
	nonce := make([]byte, challengeNonceSize)
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return Challenge{}, nil, fmt.Errorf("generate authentication challenge nonce: %w", err)
	}
	sharedSecret, err := hubPrivate.ECDH(peerPublic)
	if err != nil {
		return Challenge{}, nil, fmt.Errorf("derive Gateway authentication secret: %w", err)
	}
	challenge := Challenge{
		ID:            id,
		Nonce:         nonce,
		HubPublicKey:  hubPrivate.PublicKey().Bytes(),
		ExpiresAt:     now.Add(ttl),
		HubInstanceID: hubInstanceID,
		NodeID:        nodeID,
	}
	key, err := deriveProofKey(domain, sharedSecret, challenge, channelBinding)
	clear(sharedSecret)
	if err != nil {
		return Challenge{}, nil, err
	}
	pending := &PendingChallenge{
		challenge:  cloneChallenge(challenge),
		transcript: challengeTranscript(domain, challenge, channelBinding),
	}
	copy(pending.proofKey[:], key)
	clear(key)
	return challenge, pending, nil
}

// NewChallengeID creates the random context used by channel binding and the
// one-use X25519 challenge. In TLS mode, peers also use it as exporter context
// before computing the proof.
func NewChallengeID() ([]byte, error) {
	id := make([]byte, challengeIDSize)
	if _, err := io.ReadFull(rand.Reader, id); err != nil {
		return nil, fmt.Errorf("generate authentication challenge ID: %w", err)
	}
	return id, nil
}

// Verify checks a Gateway's HMAC proof and consumes this challenge exactly
// once. The proof authenticates possession of the registered X25519 private
// key; it is not a digital signature.
func (p *PendingChallenge) Verify(challengeID, proof []byte, now time.Time) error {
	if p == nil || p.consumed {
		return errors.New("authentication challenge has already been consumed")
	}
	p.consumed = true
	defer clear(p.proofKey[:])
	defer clear(p.transcript)
	if !now.Before(p.challenge.ExpiresAt) {
		return errors.New("authentication challenge has expired")
	}
	if len(challengeID) != len(p.challenge.ID) || subtle.ConstantTimeCompare(challengeID, p.challenge.ID) != 1 {
		return errors.New("authentication challenge ID does not match")
	}
	if len(proof) != proofSize {
		return errors.New("authentication proof has an invalid size")
	}
	mac := hmac.New(sha256.New, p.proofKey[:])
	_, _ = mac.Write(p.transcript)
	if !hmac.Equal(proof, mac.Sum(nil)) {
		return errors.New("Gateway X25519 proof is invalid")
	}
	return nil
}

// GatewayProof computes the client proof from the Gateway's registered static
// X25519 private key, the challenge, and the transport channel binding.
func GatewayProof(privateKey []byte, challenge Challenge, channelBinding []byte) ([]byte, error) {
	return gatewayProof(x25519AuthDomain, privateKey, challenge, channelBinding)
}

// GatewayRegistrationProof computes the proof for first-time credential
// enrollment and cannot be replayed as ordinary control-stream auth.
func GatewayRegistrationProof(privateKey []byte, challenge Challenge, channelBinding []byte) ([]byte, error) {
	return gatewayProof(x25519RegistrationDomain, privateKey, challenge, channelBinding)
}

// ClientProof proves possession of the registered Client X25519 private key.
func ClientProof(privateKey []byte, challenge Challenge, channelBinding []byte) ([]byte, error) {
	return gatewayProof(x25519ClientAuthDomain, privateKey, challenge, channelBinding)
}

// ClientRegistrationProof proves possession of the Client X25519 private key
// during one-time enrollment and is distinct from ClientProof.
func ClientRegistrationProof(privateKey []byte, challenge Challenge, channelBinding []byte) ([]byte, error) {
	return gatewayProof(x25519ClientRegistrationDomain, privateKey, challenge, channelBinding)
}

func gatewayProof(domain string, privateKey []byte, challenge Challenge, channelBinding []byte) ([]byte, error) {
	if len(privateKey) != 32 {
		return nil, errors.New("Gateway X25519 private key must contain 32 bytes")
	}
	if len(channelBinding) < 32 {
		return nil, errors.New("channel binding must contain at least 32 bytes")
	}
	if len(challenge.ID) != challengeIDSize || len(challenge.Nonce) != challengeNonceSize || len(challenge.HubPublicKey) != 32 {
		return nil, errors.New("authentication challenge has invalid key or nonce sizes")
	}
	curve := ecdh.X25519()
	staticPrivate, err := curve.NewPrivateKey(privateKey)
	if err != nil {
		return nil, fmt.Errorf("parse Gateway X25519 private key: %w", err)
	}
	hubPublic, err := curve.NewPublicKey(challenge.HubPublicKey)
	if err != nil {
		return nil, fmt.Errorf("parse HUB ephemeral X25519 public key: %w", err)
	}
	sharedSecret, err := staticPrivate.ECDH(hubPublic)
	if err != nil {
		return nil, fmt.Errorf("derive Gateway authentication secret: %w", err)
	}
	defer clear(sharedSecret)
	key, err := deriveProofKey(domain, sharedSecret, challenge, channelBinding)
	if err != nil {
		return nil, err
	}
	defer clear(key)
	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write(challengeTranscript(domain, challenge, channelBinding))
	return mac.Sum(nil), nil
}

func deriveProofKey(domain string, sharedSecret []byte, challenge Challenge, channelBinding []byte) ([]byte, error) {
	if len(sharedSecret) != 32 {
		return nil, errors.New("X25519 shared secret has an invalid size")
	}
	saltInput := append(append([]byte(nil), []byte(domain+"/salt")...), challenge.Nonce...)
	saltInput = append(saltInput, channelBinding...)
	salt := sha256.Sum256(saltInput)
	clear(saltInput)
	info := []byte(domain + "/proof-key")
	reader := hkdf.New(sha256.New, sharedSecret, salt[:], info)
	key := make([]byte, sha256.Size)
	if _, err := io.ReadFull(reader, key); err != nil {
		clear(key)
		return nil, fmt.Errorf("derive Gateway authentication proof key: %w", err)
	}
	return key, nil
}

func challengeTranscript(domain string, challenge Challenge, channelBinding []byte) []byte {
	buf := make([]byte, 0, 512)
	appendField := func(value []byte) {
		var size [4]byte
		binary.BigEndian.PutUint32(size[:], uint32(len(value)))
		buf = append(buf, size[:]...)
		buf = append(buf, value...)
	}
	appendField([]byte(domain))
	appendField([]byte(challenge.NodeID))
	appendField([]byte(challenge.HubInstanceID))
	appendField(challenge.ID)
	appendField(challenge.Nonce)
	appendField(challenge.HubPublicKey)
	var expiry [8]byte
	binary.BigEndian.PutUint64(expiry[:], uint64(challenge.ExpiresAt.UnixMilli()))
	appendField(expiry[:])
	appendField(channelBinding)
	return buf
}

func cloneChallenge(challenge Challenge) Challenge {
	challenge.ID = append([]byte(nil), challenge.ID...)
	challenge.Nonce = append([]byte(nil), challenge.Nonce...)
	challenge.HubPublicKey = append([]byte(nil), challenge.HubPublicKey...)
	return challenge
}
