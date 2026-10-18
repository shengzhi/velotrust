package iam

import (
	"context"
	"encoding/base32"
	"encoding/base64"
	"strings"
	"testing"
	"time"

	servicecontext "velotrust/management/internal/context"
	model "velotrust/management/internal/model/iam"
	repository "velotrust/management/internal/repository/iam"
)

type testStore struct {
	repository.Store
	user                   *model.User
	blocked                bool
	loginSession           *model.UserSession
	loginLog               *model.LoginLog
	usernameThrottleHash   string
	ipThrottleHash         string
	revokedReason          string
	touched                bool
	loginFailureCount      int
	totpAuthenticator      *model.UserAuthenticator
	totpConsumeError       error
	totpConsumedStep       int64
	revokeRoleCalled       bool
	revokePermissionCalled bool
	setUserStatusCalled    bool
	resetPasswordCalled    bool
	changePasswordCalled   bool
	changedSessionID       string
	grantRoleCalled        bool
	grantRoleActorID       int64
	grantRoleTargetID      int64
	hasPermission          bool
	resourceCatalog        []model.Resource
	resourceCatalogCalled  bool
}

func (s *testStore) LoginBlocked(*servicecontext.Context, string, string) (bool, error) {
	return s.blocked, nil
}

func (s *testStore) FindTOTPAuthenticator(*servicecontext.Context, int64) (*model.UserAuthenticator, error) {
	return s.totpAuthenticator, nil
}

func (s *testStore) ConsumeTOTP(_ *servicecontext.Context, _ int64, step int64) error {
	s.totpConsumedStep = step
	return s.totpConsumeError
}

func (s *testStore) FindUserByUsername(*servicecontext.Context, string) (*model.User, error) {
	return s.user, nil
}

func (s *testStore) FindUserByID(*servicecontext.Context, int64) (*model.User, error) {
	return s.user, nil
}

func (s *testStore) CreateLoginSession(_ *servicecontext.Context, session *model.UserSession, log *model.LoginLog, _ string, usernameHash, ipHash string) error {
	s.loginSession, s.loginLog = session, log
	s.usernameThrottleHash, s.ipThrottleHash = usernameHash, ipHash
	return nil
}

func (s *testStore) RecordLoginFailure(_ *servicecontext.Context, _ *model.LoginLog, _, _ string, _, _, _ int) error {
	s.loginFailureCount++
	return nil
}

func (s *testStore) ResolveSession(*servicecontext.Context, string) (*model.UserSession, *model.User, error) {
	return s.loginSession, s.user, nil
}

func (s *testStore) TouchSession(*servicecontext.Context, string, string, string) error {
	s.touched = true
	return nil
}

func (s *testStore) RevokeSession(_ *servicecontext.Context, _, reason string) error {
	s.revokedReason = reason
	return nil
}

func (s *testStore) GrantRole(_ *servicecontext.Context, actorID, userID, _ int64, _ *model.AuditLog) error {
	s.grantRoleCalled = true
	s.grantRoleActorID, s.grantRoleTargetID = actorID, userID
	return nil
}

func (s *testStore) RevokeRole(*servicecontext.Context, int64, int64, int64, *model.AuditLog) error {
	s.revokeRoleCalled = true
	return nil
}

func (s *testStore) RevokePermission(*servicecontext.Context, int64, int64, int64, *model.AuditLog) error {
	s.revokePermissionCalled = true
	return nil
}

func (s *testStore) SetUserStatus(*servicecontext.Context, int64, int64, string, *model.AuditLog) error {
	s.setUserStatusCalled = true
	return nil
}

func (s *testStore) ResetUserPassword(*servicecontext.Context, int64, int64, string, string, int16, *model.AuditLog) error {
	s.resetPasswordCalled = true
	return nil
}

func (s *testStore) ChangeOwnPassword(_ *servicecontext.Context, _ int64, _, _, _ string, _ int16, sessionID string, _ *model.AuditLog) error {
	s.changePasswordCalled = true
	s.changedSessionID = sessionID
	return nil
}

func (s *testStore) HasPermission(*servicecontext.Context, int64, string) (bool, error) {
	return s.hasPermission, nil
}

func (s *testStore) ListResourceCatalog(*servicecontext.Context) ([]model.Resource, error) {
	s.resourceCatalogCalled = true
	return s.resourceCatalog, nil
}

func TestPasswordHashVerifiesAndUsesUniqueSalt(t *testing.T) {
	salt1, hash1, err := hashPassword("correct horse battery staple")
	if err != nil {
		t.Fatal(err)
	}
	salt2, _, err := hashPassword("correct horse battery staple")
	if err != nil {
		t.Fatal(err)
	}
	if salt1 == salt2 {
		t.Fatal("password hashes reused a salt")
	}
	saltBytes, err := base64.RawStdEncoding.DecodeString(salt1)
	if err != nil || len(saltBytes) != passwordSaltLength {
		t.Fatalf("unexpected salt encoding/length: %d, %v", len(saltBytes), err)
	}
	version := PasswordHashVersion
	user := &model.User{PasswordSalt: &salt1, PasswordHash: &hash1, PasswordHashVersion: &version}
	if !verifyPassword(user, "correct horse battery staple") {
		t.Fatal("valid password was rejected")
	}
	if verifyPassword(user, "incorrect password") {
		t.Fatal("invalid password was accepted")
	}
	if verifyPassword(nil, "incorrect password") {
		t.Fatal("missing user was accepted")
	}
}

func TestOpaqueSessionTokenStoresOnlyDigest(t *testing.T) {
	token, digest, err := newToken()
	if err != nil {
		t.Fatal(err)
	}
	if token == digest || len(digest) != 64 || strings.Contains(token, digest) {
		t.Fatalf("token/digest format is unsafe: token=%q digest=%q", token, digest)
	}
	decoded, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil || len(decoded) != sessionTokenLength {
		t.Fatalf("unexpected opaque token encoding: %d, %v", len(decoded), err)
	}
	if hashToken(token) != digest {
		t.Fatal("returned token digest does not match token")
	}
}

func TestLoginPersistsOpaqueSessionAndThrottleResetTogether(t *testing.T) {
	salt, passwordHash, err := hashPassword("a long test password")
	if err != nil {
		t.Fatal(err)
	}
	version := PasswordHashVersion
	store := &testStore{user: &model.User{ID: 7, Username: "operator", DisplayName: "Operator", UserStatus: "active", PasswordSalt: &salt, PasswordHash: &passwordHash, PasswordHashVersion: &version}}
	service, err := New(store, Options{ThrottleHMACKey: []byte(strings.Repeat("k", 32))})
	if err != nil {
		t.Fatal(err)
	}
	grant, err := service.Login(context.Background(), " operator ", "a long test password", "192.0.2.8", "test-agent", "request-1")
	if err != nil {
		t.Fatal(err)
	}
	if grant.AccessToken == "" || grant.User.ID != 7 || grant.User.Username != "operator" {
		t.Fatalf("unexpected login response: %+v", grant)
	}
	if store.loginSession == nil || store.loginSession.TokenHash != hashToken(grant.AccessToken) {
		t.Fatal("database session did not store only the token digest")
	}
	if store.loginLog == nil || store.loginLog.Result != "success" || store.loginLog.SessionID == nil {
		t.Fatalf("login event was not persisted with the session: %+v", store.loginLog)
	}
	if store.usernameThrottleHash == "" || store.ipThrottleHash == "" {
		t.Fatal("successful login did not atomically clear its shared throttle buckets")
	}
	if grant.ExpiresAt.After(store.loginSession.ExpiresAt) || !grant.ExpiresAt.After(time.Now()) {
		t.Fatal("session idle expiry must be no later than absolute expiry")
	}
}

func TestLoginRecordsFailureWithoutCreatingSession(t *testing.T) {
	salt, passwordHash, err := hashPassword("correct password")
	if err != nil {
		t.Fatal(err)
	}
	version := PasswordHashVersion
	store := &testStore{user: &model.User{ID: 7, Username: "operator", UserStatus: "active", PasswordSalt: &salt, PasswordHash: &passwordHash, PasswordHashVersion: &version}}
	service, err := New(store, Options{ThrottleHMACKey: []byte(strings.Repeat("k", 32))})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.Login(context.Background(), "operator", "wrong password", "192.0.2.8", "", ""); err == nil {
		t.Fatal("invalid password was accepted")
	}
	if store.loginFailureCount != 1 || store.loginSession != nil {
		t.Fatalf("failed login should be logged and must not create a session: %+v", store)
	}
}

func TestLoginRequiresAndConsumesTOTPWhenAuthenticatorIsActive(t *testing.T) {
	salt, passwordHash, err := hashPassword("a long test password")
	if err != nil {
		t.Fatal(err)
	}
	version := PasswordHashVersion
	secret := "JBSWY3DPEHPK3PXP"
	ciphertext, err := encryptTOTPSecret(secret, []byte(strings.Repeat("e", 32)), 7, "v1")
	if err != nil {
		t.Fatal(err)
	}
	store := &testStore{
		user:              &model.User{ID: 7, Username: "operator", UserStatus: "active", PasswordSalt: &salt, PasswordHash: &passwordHash, PasswordHashVersion: &version},
		totpAuthenticator: &model.UserAuthenticator{UserID: 7, SecretCiphertext: ciphertext, EncryptionKeyVersion: "v1", Status: "active"},
	}
	service, err := New(store, Options{ThrottleHMACKey: []byte(strings.Repeat("k", 32)), TOTPEncryptionKey: []byte(strings.Repeat("e", 32))})
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	secretBytes, _ := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(secret)
	code := totpCodeForStep(secretBytes, now.Unix()/totpPeriod)
	grant, err := service.LoginWithTOTP(context.Background(), "operator", "a long test password", code, "192.0.2.8", "test-agent", "request-mfa")
	if err != nil {
		t.Fatal(err)
	}
	if store.loginSession == nil || store.loginSession.AuthLevel != "password_mfa" || store.loginSession.MFAAuthenticatedAt == nil || store.totpConsumedStep == 0 {
		t.Fatalf("TOTP login was not recorded as MFA: session=%+v consumed=%d", store.loginSession, store.totpConsumedStep)
	}
	if grant.User.ID != 7 || store.loginLog == nil || !store.loginLog.MFAUsed {
		t.Fatal("successful MFA login metadata was not returned or recorded")
	}
	store.loginSession = nil
	if _, err := service.LoginWithTOTP(context.Background(), "operator", "a long test password", "bad", "192.0.2.8", "", ""); err == nil || store.loginSession != nil {
		t.Fatal("invalid TOTP code was accepted or created a session")
	}
}

func TestResolveBearerRevokesIdleSession(t *testing.T) {
	now := time.Now().UTC()
	store := &testStore{
		user:         &model.User{ID: 7, Username: "operator", UserStatus: "active"},
		loginSession: &model.UserSession{SessionID: "session-id", UserID: 7, TokenHash: hashToken(strings.Repeat("a", 43)), IssuedAt: now.Add(-2 * time.Hour), ExpiresAt: now.Add(time.Hour), Status: "active"},
	}
	service, err := New(store, Options{SessionIdleTTL: time.Minute, SessionAbsoluteTTL: 3 * time.Hour, ThrottleHMACKey: []byte(strings.Repeat("k", 32))})
	if err != nil {
		t.Fatal(err)
	}
	_, _, err = service.ResolveBearer(context.Background(), strings.Repeat("a", 43))
	if err == nil || store.revokedReason != "idle_timeout" || store.touched {
		t.Fatalf("idle session was not rejected and revoked: err=%v store=%+v", err, store)
	}
}

func TestGrantRoleRejectsSelfAssignmentBeforeStoreCall(t *testing.T) {
	store := &testStore{}
	service, err := New(store, Options{ThrottleHMACKey: []byte(strings.Repeat("k", 32))})
	if err != nil {
		t.Fatal(err)
	}
	ctx := servicecontext.New(context.Background(), servicecontext.Principal{ID: "7", Name: "operator"}, servicecontext.RequestMeta{})
	if err := service.GrantRole(ctx, 7, 1); err == nil {
		t.Fatal("self role assignment was accepted")
	}
	if store.grantRoleCalled {
		t.Fatal("self role assignment unexpectedly reached the persistence layer")
	}
}

func TestGrantRoleRequiresPermissionAtServiceBoundary(t *testing.T) {
	store := &testStore{}
	service, err := New(store, Options{ThrottleHMACKey: []byte(strings.Repeat("k", 32))})
	if err != nil {
		t.Fatal(err)
	}
	ctx := servicecontext.New(context.Background(), servicecontext.Principal{ID: "7", Name: "operator"}, servicecontext.RequestMeta{})
	if err := service.GrantRole(ctx, 8, 3); err == nil {
		t.Fatal("role assignment without iam.role.grant permission was accepted")
	}
	if store.grantRoleCalled {
		t.Fatal("unauthorized role assignment reached persistence")
	}
	store.hasPermission = true
	if err := service.GrantRole(ctx, 8, 3); err != nil {
		t.Fatal(err)
	}
	if !store.grantRoleCalled || store.grantRoleActorID != 7 || store.grantRoleTargetID != 8 {
		t.Fatalf("authorized role grant did not preserve operator and target IDs: %+v", store)
	}
}

func TestResourceCatalogRequiresRoleReadPermission(t *testing.T) {
	store := &testStore{resourceCatalog: []model.Resource{{ID: 1, ResourceCode: "iam.user"}}}
	service, err := New(store, Options{ThrottleHMACKey: []byte(strings.Repeat("k", 32))})
	if err != nil {
		t.Fatal(err)
	}
	ctx := servicecontext.New(context.Background(), servicecontext.Principal{ID: "7", Name: "operator"}, servicecontext.RequestMeta{})
	if _, err := service.ListResourceCatalog(ctx); err == nil {
		t.Fatal("resource catalog was returned without iam.role.read")
	}
	if store.resourceCatalogCalled {
		t.Fatal("unauthorized resource catalog read reached the store")
	}

	store.hasPermission = true
	resources, err := service.ListResourceCatalog(ctx)
	if err != nil {
		t.Fatalf("authorized resource catalog read failed: %v", err)
	}
	if !store.resourceCatalogCalled || len(resources) != 1 || resources[0].ResourceCode != "iam.user" {
		t.Fatalf("unexpected resource catalog result: called=%v resources=%+v", store.resourceCatalogCalled, resources)
	}
}

func TestIAMMutationsRequirePermissionsAtServiceBoundary(t *testing.T) {
	store := &testStore{}
	service, err := New(store, Options{ThrottleHMACKey: []byte(strings.Repeat("k", 32))})
	if err != nil {
		t.Fatal(err)
	}
	ctx := servicecontext.New(context.Background(), servicecontext.Principal{ID: "7", Name: "operator"}, servicecontext.RequestMeta{})
	mutations := []struct {
		name   string
		call   func() error
		called func() bool
	}{
		{"disable user", func() error { return service.SetUserStatus(ctx, 8, "disabled") }, func() bool { return store.setUserStatusCalled }},
		{"revoke role", func() error { return service.RevokeRole(ctx, 8, 3) }, func() bool { return store.revokeRoleCalled }},
		{"revoke permission", func() error { return service.RevokePermission(ctx, 3, 4) }, func() bool { return store.revokePermissionCalled }},
	}
	for _, mutation := range mutations {
		if err := mutation.call(); err == nil {
			t.Errorf("%s without permission succeeded", mutation.name)
		}
		if mutation.called() {
			t.Errorf("%s without permission reached the store", mutation.name)
		}
	}
}

func TestPasswordResetRequiresPermissionAndRejectsSelfReset(t *testing.T) {
	store := &testStore{}
	service, err := New(store, Options{ThrottleHMACKey: []byte(strings.Repeat("k", 32))})
	if err != nil {
		t.Fatal(err)
	}
	ctx := servicecontext.New(context.Background(), servicecontext.Principal{ID: "7", Name: "operator"}, servicecontext.RequestMeta{})
	if err := service.ResetUserPassword(ctx, 8, "temporary password 123"); err == nil || store.resetPasswordCalled {
		t.Fatal("password reset without iam.user.write permission reached the repository")
	}
	store.hasPermission = true
	if err := service.ResetUserPassword(ctx, 7, "temporary password 123"); err == nil || store.resetPasswordCalled {
		t.Fatal("operator was allowed to reset their own password through the admin path")
	}
	if err := service.ResetUserPassword(ctx, 8, "short"); err == nil || store.resetPasswordCalled {
		t.Fatal("weak reset password was accepted")
	}
	if err := service.ResetUserPassword(ctx, 8, "temporary password 123"); err != nil {
		t.Fatal(err)
	}
	if !store.resetPasswordCalled {
		t.Fatal("authorized reset did not reach the repository")
	}
}

func TestOwnPasswordChangeChecksOldPasswordAndKeepsCurrentSession(t *testing.T) {
	salt, hash, err := hashPassword("old password value")
	if err != nil {
		t.Fatal(err)
	}
	version := PasswordHashVersion
	store := &testStore{user: &model.User{ID: 7, Username: "operator", UserStatus: "active", PasswordSalt: &salt, PasswordHash: &hash, PasswordHashVersion: &version}}
	service, err := New(store, Options{ThrottleHMACKey: []byte(strings.Repeat("k", 32))})
	if err != nil {
		t.Fatal(err)
	}
	ctx := servicecontext.New(context.Background(), servicecontext.Principal{ID: "7", Name: "operator", EntityID: "session-current"}, servicecontext.RequestMeta{})
	if err := service.ChangeOwnPassword(ctx, "wrong old password", "new password value 123"); err == nil || store.changePasswordCalled {
		t.Fatal("password change accepted an incorrect current password")
	}
	if err := service.ChangeOwnPassword(ctx, "old password value", "new password value 123"); err != nil {
		t.Fatal(err)
	}
	if !store.changePasswordCalled || store.changedSessionID != "session-current" {
		t.Fatal("password change did not preserve the current session identity")
	}
}
