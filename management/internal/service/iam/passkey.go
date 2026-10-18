package iam

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"time"

	"github.com/go-webauthn/webauthn/protocol"
	webauthnlib "github.com/go-webauthn/webauthn/webauthn"
	servicecontext "velotrust/management/internal/context"
	model "velotrust/management/internal/model/iam"
	repository "velotrust/management/internal/repository/iam"
	appErrors "velotrust/pkg/errors"
)

const ceremonyTTL = 5 * time.Minute

type PasskeyRegistrationStart struct {
	ChallengeID string                       `json:"challenge_id"`
	Options     *protocol.CredentialCreation `json:"options"`
}

type PasskeyLoginStart struct {
	ChallengeID string                        `json:"challenge_id"`
	Options     *protocol.CredentialAssertion `json:"options"`
}

type webAuthnUser struct {
	user        *model.User
	rows        []model.WebAuthnCredential
	credentials []webauthnlib.Credential
}

func newWebAuthnUser(user *model.User, rows []model.WebAuthnCredential) (*webAuthnUser, error) {
	if user == nil || user.ID <= 0 || len(user.WebAuthnUserHandle) == 0 || len(user.WebAuthnUserHandle) > 64 {
		return nil, errors.New("IAM user has no valid WebAuthn user handle")
	}
	result := &webAuthnUser{user: user, rows: rows, credentials: make([]webauthnlib.Credential, 0, len(rows))}
	for _, row := range rows {
		var transports []protocol.AuthenticatorTransport
		if row.Transports != "" {
			if err := json.Unmarshal([]byte(row.Transports), &transports); err != nil {
				return nil, fmt.Errorf("decode stored WebAuthn transports: %w", err)
			}
		}
		result.credentials = append(result.credentials, webauthnlib.Credential{
			ID: row.CredentialID, PublicKey: row.PublicKeyCOSE, AttestationType: "none", AttestationFormat: "none",
			Transport:     transports,
			Flags:         webauthnlib.CredentialFlags{UserPresent: true, UserVerified: true, BackupEligible: row.BackupEligible, BackupState: row.BackupState},
			Authenticator: webauthnlib.Authenticator{AAGUID: row.AAGUID, SignCount: uint32(row.SignatureCount)},
		})
	}
	return result, nil
}

func (u *webAuthnUser) WebAuthnID() []byte                            { return u.user.WebAuthnUserHandle }
func (u *webAuthnUser) WebAuthnName() string                          { return u.user.Username }
func (u *webAuthnUser) WebAuthnDisplayName() string                   { return u.user.DisplayName }
func (u *webAuthnUser) WebAuthnCredentials() []webauthnlib.Credential { return u.credentials }

func (s *Service) BeginPasskeyRegistration(ctx *servicecontext.Context, origin string) (*PasskeyRegistrationStart, error) {
	if s.webAuthn == nil || s.ceremonies == nil {
		return nil, errors.New("WebAuthn is not configured")
	}
	userID, err := parseUserID(ctxUserID(ctx))
	if err != nil {
		return nil, appErrors.NewAuthError("IAM_UNAUTHENTICATED", appErrors.AuthUnauthenticated)
	}
	if err := s.validateOrigin(origin); err != nil {
		return nil, err
	}
	user, err := s.store.FindUserByID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("load WebAuthn registration user: %w", err)
	}
	if user == nil || user.UserStatus != "active" {
		return nil, appErrors.NewAuthError("IAM_UNAUTHENTICATED", appErrors.AuthUnauthenticated)
	}
	rows, err := s.store.ListWebAuthnCredentials(ctx, userID, s.opts.WebAuthnRPID)
	if err != nil {
		return nil, err
	}
	waUser, err := newWebAuthnUser(user, rows)
	if err != nil {
		return nil, err
	}
	selection := protocol.AuthenticatorSelection{
		ResidentKey: protocol.ResidentKeyRequirementRequired, RequireResidentKey: protocol.ResidentKeyRequired(),
		UserVerification: protocol.VerificationRequired,
	}
	options, session, err := s.webAuthn.BeginRegistration(waUser,
		webauthnlib.WithRegistrationOrigin(origin),
		webauthnlib.WithAuthenticatorSelection(selection),
		webauthnlib.WithExclusions(webauthnlib.Credentials(waUser.WebAuthnCredentials()).CredentialDescriptors()),
	)
	if err != nil {
		return nil, fmt.Errorf("begin Passkey registration: %w", err)
	}
	challengeID, err := s.saveChallenge(ctx, "webauthn_registration", int64Ptr(userID), origin, session)
	if err != nil {
		return nil, err
	}
	return &PasskeyRegistrationStart{ChallengeID: challengeID, Options: options}, nil
}

func (s *Service) FinishPasskeyRegistration(ctx *servicecontext.Context, challengeID, credentialName, origin string, response []byte) error {
	if s.webAuthn == nil || s.ceremonies == nil {
		return errors.New("WebAuthn is not configured")
	}
	userID, err := parseUserID(ctxUserID(ctx))
	if err != nil {
		return appErrors.NewAuthError("IAM_UNAUTHENTICATED", appErrors.AuthUnauthenticated)
	}
	if err := s.validateOrigin(origin); err != nil {
		return err
	}
	state, challenge, err := s.consumeChallenge(ctx, challengeID, "webauthn_registration")
	if err != nil {
		return err
	}
	if challenge.UserID == nil || *challenge.UserID != userID || challenge.Origin == nil || *challenge.Origin != origin || !bytes.Equal(state.UserID, mustUserHandle(s.store, ctx, userID)) {
		return appErrors.NewAuthError("IAM_PASSKEY_CHALLENGE_INVALID", appErrors.AuthUnauthenticated)
	}
	user, err := s.store.FindUserByID(ctx, userID)
	if err != nil || user == nil || user.UserStatus != "active" {
		return appErrors.NewAuthError("IAM_UNAUTHENTICATED", appErrors.AuthUnauthenticated)
	}
	rows, err := s.store.ListWebAuthnCredentials(ctx, userID, s.opts.WebAuthnRPID)
	if err != nil {
		return err
	}
	waUser, err := newWebAuthnUser(user, rows)
	if err != nil {
		return err
	}
	request := ceremonyRequest(s.opts.WebAuthnRPID, origin, response)
	credential, err := s.webAuthn.FinishRegistration(waUser, *state, request)
	if err != nil {
		return appErrors.NewAuthError("IAM_PASSKEY_INVALID", appErrors.AuthUnauthenticated)
	}
	if !credential.Flags.UserVerified {
		return appErrors.NewAuthError("IAM_PASSKEY_USER_VERIFICATION_REQUIRED", appErrors.AuthUnauthenticated)
	}
	transports, err := json.Marshal(credential.Transport)
	if err != nil {
		return err
	}
	if len(credentialName) > 128 {
		return appErrors.NewClientError("IAM_INVALID_ARGUMENT", map[string]string{"credential_name": "max_length_128"})
	}
	actor, actorID := actorFromContext(ctx)
	audit := s.newAudit(ctx, actorID, actor, "iam.passkey.register", "webauthn_credential", hex.EncodeToString(credential.ID), "success", nil)
	stored := &model.WebAuthnCredential{
		UserID: userID, RPID: s.opts.WebAuthnRPID, CredentialID: credential.ID, PublicKeyCOSE: credential.PublicKey,
		SignatureCount: int64(credential.Authenticator.SignCount), AAGUID: credential.Authenticator.AAGUID, Transports: string(transports),
		BackupEligible: credential.Flags.BackupEligible, BackupState: credential.Flags.BackupState,
		CredentialName: strings.TrimSpace(credentialName), Status: "active",
	}
	return s.store.CreateWebAuthnCredential(ctx, stored, audit)
}

func (s *Service) BeginPasskeyLogin(ctx context.Context, origin string) (*PasskeyLoginStart, error) {
	serviceCtx := servicecontext.FromContext(ctx)
	if s.webAuthn == nil || s.ceremonies == nil {
		return nil, errors.New("WebAuthn is not configured")
	}
	if err := s.validateOrigin(origin); err != nil {
		return nil, err
	}
	options, session, err := s.webAuthn.BeginDiscoverableLogin(
		webauthnlib.WithUserVerification(protocol.VerificationRequired),
		webauthnlib.WithLoginOrigin(origin),
	)
	if err != nil {
		return nil, fmt.Errorf("begin Passkey login: %w", err)
	}
	challengeID, err := s.saveChallenge(serviceCtx, "webauthn_authentication", nil, origin, session)
	if err != nil {
		return nil, err
	}
	return &PasskeyLoginStart{ChallengeID: challengeID, Options: options}, nil
}

func (s *Service) FinishPasskeyLogin(ctx context.Context, challengeID, origin string, response []byte, sourceIP, userAgent, requestID string) (*SessionGrant, error) {
	serviceCtx := servicecontext.FromContext(ctx)
	if s.webAuthn == nil || s.ceremonies == nil {
		return nil, errors.New("WebAuthn is not configured")
	}
	if err := s.validateOrigin(origin); err != nil {
		return nil, invalidLogin()
	}
	state, challenge, err := s.consumeChallenge(serviceCtx, challengeID, "webauthn_authentication")
	if err != nil || challenge.Origin == nil || *challenge.Origin != origin {
		return nil, invalidLogin()
	}
	var candidate *webAuthnUser
	lookup := func(rawID, userHandle []byte) (webauthnlib.User, error) {
		if len(userHandle) == 0 || len(rawID) == 0 {
			return nil, errors.New("discoverable Passkey response lacks a credential ID or user handle")
		}
		user, rows, findErr := s.store.FindUserByCredential(serviceCtx, s.opts.WebAuthnRPID, rawID)
		if findErr != nil {
			return nil, findErr
		}
		if user == nil || user.UserStatus != "active" || !bytes.Equal(user.WebAuthnUserHandle, userHandle) {
			return nil, errors.New("Passkey identity is unavailable")
		}
		candidate, findErr = newWebAuthnUser(user, rows)
		return candidate, findErr
	}
	credential, err := s.webAuthn.FinishDiscoverableLogin(lookup, *state, ceremonyRequest(s.opts.WebAuthnRPID, origin, response))
	if err != nil || candidate == nil || credential == nil || credential.Authenticator.CloneWarning {
		return nil, invalidLogin()
	}
	var stored *model.WebAuthnCredential
	for i := range candidate.rows {
		if bytes.Equal(candidate.rows[i].CredentialID, credential.ID) {
			stored = &candidate.rows[i]
			break
		}
	}
	if stored == nil {
		return nil, invalidLogin()
	}
	stored.SignatureCount = int64(credential.Authenticator.SignCount)
	stored.BackupState = credential.Flags.BackupState
	if err := s.store.UpdateWebAuthnCredentialUse(serviceCtx, stored); err != nil {
		return nil, invalidLogin()
	}
	rawToken, tokenHash, err := newToken()
	if err != nil {
		return nil, err
	}
	sessionID, err := randomHex(32)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	session := &model.UserSession{SessionID: sessionID, UserID: candidate.user.ID, TokenHash: tokenHash, AuthLevel: "passkey", IssuedAt: now, ExpiresAt: now.Add(s.opts.SessionAbsoluteTTL), LastSeenAt: &now, SourceIP: optional(sourceIP), UserAgent: optional(userAgent), Status: "active"}
	loginLog := s.loginLog(candidate.user, "passkey", "success", "", sourceIP, userAgent, requestID, sessionID)
	if err := s.store.CreateLoginSession(serviceCtx, session, loginLog, sourceIP, "", ""); err != nil {
		return nil, fmt.Errorf("persist Passkey login: %w", err)
	}
	return &SessionGrant{AccessToken: rawToken, TokenType: "Bearer", ExpiresAt: now.Add(s.opts.SessionIdleTTL), User: view(candidate.user)}, nil
}

func (s *Service) RevokePasskey(ctx *servicecontext.Context, credentialID int64) error {
	userID, err := parseUserID(ctxUserID(ctx))
	if err != nil {
		return appErrors.NewAuthError("IAM_UNAUTHENTICATED", appErrors.AuthUnauthenticated)
	}
	actor, actorID := actorFromContext(ctx)
	audit := s.newAudit(ctx, actorID, actor, "iam.passkey.revoke", "webauthn_credential", fmt.Sprintf("%d", credentialID), "success", nil)
	return s.store.RevokeWebAuthnCredential(ctx, userID, credentialID, audit)
}

func (s *Service) saveChallenge(ctx *servicecontext.Context, typ string, userID *int64, origin string, session *webauthnlib.SessionData) (string, error) {
	if session == nil || session.Challenge == "" {
		return "", errors.New("WebAuthn challenge data is empty")
	}
	challengeID, err := randomHex(32)
	if err != nil {
		return "", err
	}
	expires := time.Now().UTC().Add(ceremonyTTL)
	session.Expires = expires
	challengeSum := sha256.Sum256([]byte(session.Challenge))
	rpID, originCopy := s.opts.WebAuthnRPID, origin
	challenge := &model.AuthChallenge{ChallengeID: challengeID, UserID: userID, ChallengeType: typ,
		ChallengeHash: hex.EncodeToString(challengeSum[:]), RPID: &rpID, Origin: &originCopy, ExpiresAt: expires,
		AttemptCount: 0, MaxAttempts: 1, Status: "pending"}
	if err := s.store.CreateAuthChallenge(ctx, challenge); err != nil {
		return "", err
	}
	if err := s.ceremonies.Put(ctx, challengeID, session, ceremonyTTL); err != nil {
		return "", fmt.Errorf("store WebAuthn session data: %w", err)
	}
	return challengeID, nil
}

func (s *Service) consumeChallenge(ctx *servicecontext.Context, challengeID, expectedType string) (*webauthnlib.SessionData, *model.AuthChallenge, error) {
	if strings.TrimSpace(challengeID) == "" {
		return nil, nil, appErrors.NewAuthError("IAM_PASSKEY_CHALLENGE_INVALID", appErrors.AuthUnauthenticated)
	}
	var state webauthnlib.SessionData
	found, err := s.ceremonies.Consume(ctx, challengeID, &state)
	if err != nil {
		return nil, nil, fmt.Errorf("consume WebAuthn session data: %w", err)
	}
	if !found || state.Challenge == "" || !state.Expires.After(time.Now()) {
		return nil, nil, appErrors.NewAuthError("IAM_PASSKEY_CHALLENGE_INVALID", appErrors.AuthUnauthenticated)
	}
	sum := sha256.Sum256([]byte(state.Challenge))
	challenge, err := s.store.ConsumeAuthChallenge(ctx, challengeID, hex.EncodeToString(sum[:]))
	if err != nil {
		return nil, nil, fmt.Errorf("consume WebAuthn challenge record: %w", err)
	}
	if challenge == nil || challenge.ChallengeType != expectedType || challenge.RPID == nil || *challenge.RPID != s.opts.WebAuthnRPID || challenge.Origin == nil {
		return nil, nil, appErrors.NewAuthError("IAM_PASSKEY_CHALLENGE_INVALID", appErrors.AuthUnauthenticated)
	}
	return &state, challenge, nil
}

func (s *Service) validateOrigin(origin string) error {
	origin = strings.TrimSpace(origin)
	if origin == "" {
		return appErrors.NewClientError("IAM_WEBAUTHN_ORIGIN_INVALID", nil)
	}
	for _, allowed := range s.opts.WebAuthnOrigins {
		if strings.EqualFold(origin, strings.TrimSpace(allowed)) {
			return nil
		}
	}
	return appErrors.NewClientError("IAM_WEBAUTHN_ORIGIN_INVALID", nil)
}

func ceremonyRequest(rpID, origin string, body []byte) *http.Request {
	request := httptest.NewRequest(http.MethodPost, "https://"+rpID+"/jrpc", bytes.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Origin", origin)
	return request
}

func ctxUserID(ctx *servicecontext.Context) string {
	if ctx == nil {
		return ""
	}
	return ctx.UserID()
}

func mustUserHandle(store repository.Store, ctx *servicecontext.Context, userID int64) []byte {
	user, err := store.FindUserByID(ctx, userID)
	if err != nil || user == nil {
		return nil
	}
	return user.WebAuthnUserHandle
}
