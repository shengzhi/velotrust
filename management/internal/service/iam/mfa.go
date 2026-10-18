package iam

import (
	"fmt"
	"strings"
	"time"

	model "velotrust/management/internal/model/iam"
	core "velotrust/management/internal/service"
	appErrors "velotrust/pkg/errors"
)

type TOTPSetup struct {
	Secret     string `json:"secret"`
	OTPAuthURI string `json:"otpauth_uri"`
}

func (s *Service) SetupTOTP(ctx *core.Context, password string) (*TOTPSetup, error) {
	userID, err := parseUserID(ctxUserID(ctx))
	if err != nil {
		return nil, appErrors.NewAuthError("IAM_UNAUTHENTICATED", appErrors.AuthUnauthenticated)
	}
	user, err := s.store.FindUserByID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("load TOTP setup user: %w", err)
	}
	if user == nil || user.UserStatus != "active" || !verifyPassword(user, password) {
		return nil, appErrors.NewAuthError("IAM_REAUTHENTICATION_REQUIRED", appErrors.AuthUnauthenticated)
	}
	key, version, err := s.totpKey()
	if err != nil {
		return nil, err
	}
	current, err := s.store.FindTOTPAuthenticator(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("check existing TOTP authenticator: %w", err)
	}
	if current != nil && current.Status == "active" {
		return nil, appErrors.NewClientError("IAM_TOTP_ALREADY_ACTIVE", nil)
	}
	secret, err := newTOTPSecret()
	if err != nil {
		return nil, fmt.Errorf("generate TOTP secret: %w", err)
	}
	ciphertext, err := encryptTOTPSecret(secret, key, userID, version)
	if err != nil {
		return nil, fmt.Errorf("encrypt TOTP secret: %w", err)
	}
	actor, actorID := actorFromContext(ctx)
	audit := s.newAudit(ctx, actorID, actor, "iam.totp.setup", "authenticator", fmt.Sprintf("%d:totp", userID), "success", nil)
	row := &model.UserAuthenticator{UserID: userID, AuthenticatorType: "totp", SecretCiphertext: ciphertext, EncryptionKeyVersion: version, Status: "pending", CreatedAt: time.Now().UTC()}
	if err := s.store.SavePendingTOTP(ctx, row, audit); err != nil {
		return nil, fmt.Errorf("save pending TOTP authenticator: %w", err)
	}
	return &TOTPSetup{Secret: secret, OTPAuthURI: totpURI(secret, "VeloTrust", user.Username)}, nil
}

func (s *Service) ConfirmTOTP(ctx *core.Context, code string) error {
	userID, err := parseUserID(ctxUserID(ctx))
	if err != nil {
		return appErrors.NewAuthError("IAM_UNAUTHENTICATED", appErrors.AuthUnauthenticated)
	}
	row, err := s.store.FindTOTPAuthenticator(ctx, userID)
	if err != nil {
		return fmt.Errorf("load pending TOTP authenticator: %w", err)
	}
	if row == nil || row.Status != "pending" {
		return appErrors.NewClientError("IAM_TOTP_SETUP_NOT_FOUND", nil)
	}
	key, _, err := s.totpKey()
	if err != nil {
		return err
	}
	secret, err := decryptTOTPSecret(row.SecretCiphertext, key, userID, row.EncryptionKeyVersion)
	if err != nil {
		return fmt.Errorf("decrypt pending TOTP secret: %w", err)
	}
	step, ok := matchingTOTPStep(secret, code, time.Now().UTC())
	if !ok {
		return appErrors.NewAuthError("IAM_TOTP_INVALID", appErrors.AuthUnauthenticated)
	}
	actor, actorID := actorFromContext(ctx)
	audit := s.newAudit(ctx, actorID, actor, "iam.totp.activate", "authenticator", fmt.Sprintf("%d:totp", userID), "success", nil)
	if err := s.store.ActivateTOTP(ctx, userID, step, audit); err != nil {
		return fmt.Errorf("activate TOTP authenticator: %w", err)
	}
	return nil
}

func (s *Service) DisableTOTP(ctx *core.Context, password, code string) error {
	userID, err := parseUserID(ctxUserID(ctx))
	if err != nil {
		return appErrors.NewAuthError("IAM_UNAUTHENTICATED", appErrors.AuthUnauthenticated)
	}
	user, err := s.store.FindUserByID(ctx, userID)
	if err != nil {
		return fmt.Errorf("load TOTP disable user: %w", err)
	}
	if user == nil || user.UserStatus != "active" || !verifyPassword(user, password) {
		return appErrors.NewAuthError("IAM_REAUTHENTICATION_REQUIRED", appErrors.AuthUnauthenticated)
	}
	row, err := s.store.FindTOTPAuthenticator(ctx, userID)
	if err != nil {
		return fmt.Errorf("load active TOTP authenticator: %w", err)
	}
	if row == nil || row.Status != "active" {
		return appErrors.NewClientError("IAM_TOTP_NOT_ACTIVE", nil)
	}
	key, _, err := s.totpKey()
	if err != nil {
		return err
	}
	secret, err := decryptTOTPSecret(row.SecretCiphertext, key, userID, row.EncryptionKeyVersion)
	if err != nil {
		return fmt.Errorf("decrypt active TOTP secret: %w", err)
	}
	step, ok := matchingTOTPStep(secret, code, time.Now().UTC())
	if !ok {
		return appErrors.NewAuthError("IAM_TOTP_INVALID", appErrors.AuthUnauthenticated)
	}
	actor, actorID := actorFromContext(ctx)
	audit := s.newAudit(ctx, actorID, actor, "iam.totp.disable", "authenticator", fmt.Sprintf("%d:totp", userID), "success", nil)
	return s.store.DisableTOTP(ctx, userID, step, audit)
}

func (s *Service) totpKey() ([]byte, string, error) {
	if len(s.opts.TOTPEncryptionKey) < 32 {
		return nil, "", fmt.Errorf("TOTP is unavailable: auth.totp_encryption_key must contain at least 32 bytes")
	}
	version := strings.TrimSpace(s.opts.TOTPKeyVersion)
	if version == "" {
		version = "v1"
	}
	if len(version) > 64 {
		return nil, "", fmt.Errorf("auth.totp_key_version exceeds the schema limit")
	}
	return s.opts.TOTPEncryptionKey, version, nil
}
