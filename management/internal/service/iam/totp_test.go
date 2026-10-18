package iam

import (
	"encoding/base32"
	"strings"
	"testing"
	"time"
)

func TestTOTPValidatesWindowAndReturnsMatchedStep(t *testing.T) {
	key := []byte("12345678901234567890")
	secret := base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(key)
	// RFC 6238 SHA1 test vector: T=59 seconds, counter=1, six digits.
	if got := totpCodeForStep(key, 1); got != "287082" {
		t.Fatalf("RFC 6238 test vector mismatch: got=%s", got)
	}
	now := time.Unix(1_700_000_000, 0).UTC()
	step := now.Unix()/totpPeriod - 1
	code := totpCodeForStep(key, step)
	matched, ok := matchingTOTPStep(secret, code, now)
	if !ok || matched != step {
		t.Fatalf("previous-window TOTP was rejected: matched=%d ok=%v", matched, ok)
	}
	if _, ok := matchingTOTPStep(secret, "12345x", now); ok {
		t.Fatal("non-numeric TOTP code was accepted")
	}
	wrong := "0" + code[1:]
	if code[0] == '0' {
		wrong = "1" + code[1:]
	}
	if _, ok := matchingTOTPStep(secret, wrong, now); ok {
		t.Fatal("incorrect TOTP code was accepted")
	}
}

func TestTOTPSecretEncryptionBindsCiphertextToUserAndKeyVersion(t *testing.T) {
	key := []byte(strings.Repeat("k", 32))
	ciphertext, err := encryptTOTPSecret("JBSWY3DPEHPK3PXP", key, 42, "v1")
	if err != nil {
		t.Fatal(err)
	}
	got, err := decryptTOTPSecret(ciphertext, key, 42, "v1")
	if err != nil || got != "JBSWY3DPEHPK3PXP" {
		t.Fatalf("decrypt TOTP secret: got=%q err=%v", got, err)
	}
	if _, err := decryptTOTPSecret(ciphertext, key, 43, "v1"); err == nil {
		t.Fatal("ciphertext decrypted for a different user")
	}
	if _, err := decryptTOTPSecret(ciphertext, key, 42, "v2"); err == nil {
		t.Fatal("ciphertext decrypted with a different key version")
	}
}
