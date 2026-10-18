package iam

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/base32"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const totpPeriod = int64(30)

func newTOTPSecret() (string, error) {
	raw := make([]byte, 20)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(raw), nil
}

func encryptTOTPSecret(secret string, key []byte, userID int64, version string) (string, error) {
	if len(key) < 32 || userID <= 0 || version == "" {
		return "", fmt.Errorf("TOTP encryption key must contain at least 32 bytes and have a key version")
	}
	derived := sha256.Sum256(key)
	block, err := aes.NewCipher(derived[:])
	if err != nil {
		return "", err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", err
	}
	sealed := aead.Seal(nonce, nonce, []byte(secret), totpAAD(userID, version))
	return base64.RawStdEncoding.EncodeToString(sealed), nil
}

func decryptTOTPSecret(encoded string, key []byte, userID int64, version string) (string, error) {
	if len(key) < 32 || userID <= 0 || version == "" {
		return "", fmt.Errorf("TOTP encryption key is unavailable")
	}
	sealed, err := base64.RawStdEncoding.DecodeString(encoded)
	if err != nil {
		return "", fmt.Errorf("decode encrypted TOTP secret: %w", err)
	}
	derived := sha256.Sum256(key)
	block, err := aes.NewCipher(derived[:])
	if err != nil {
		return "", err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	if len(sealed) < aead.NonceSize()+aead.Overhead() {
		return "", fmt.Errorf("encrypted TOTP secret is truncated")
	}
	nonce, ciphertext := sealed[:aead.NonceSize()], sealed[aead.NonceSize():]
	plaintext, err := aead.Open(nil, nonce, ciphertext, totpAAD(userID, version))
	if err != nil {
		return "", fmt.Errorf("decrypt TOTP secret: %w", err)
	}
	return string(plaintext), nil
}

func totpAAD(userID int64, version string) []byte {
	return []byte(fmt.Sprintf("velotrust/iam/totp/%d/%s", userID, version))
}

func totpURI(secret, issuer, username string) string {
	label := url.PathEscape(issuer + ":" + username)
	query := url.Values{}
	query.Set("secret", secret)
	query.Set("issuer", issuer)
	query.Set("algorithm", "SHA1")
	query.Set("digits", "6")
	query.Set("period", strconv.FormatInt(totpPeriod, 10))
	return "otpauth://totp/" + label + "?" + query.Encode()
}

// matchingTOTPStep returns the accepted time step so persistence can reject
// replayed codes, including codes accepted from the adjacent skew window.
func matchingTOTPStep(secret, code string, now time.Time) (int64, bool) {
	code = strings.TrimSpace(code)
	if len(code) != 6 {
		return 0, false
	}
	for _, r := range code {
		if r < '0' || r > '9' {
			return 0, false
		}
	}
	key, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(strings.ToUpper(strings.TrimSpace(secret)))
	if err != nil || len(key) == 0 {
		return 0, false
	}
	current := now.Unix() / totpPeriod
	for step := current - 1; step <= current+1; step++ {
		if step < 0 {
			continue
		}
		expected := totpCodeForStep(key, step)
		if hmac.Equal([]byte(expected), []byte(code)) {
			return step, true
		}
	}
	return 0, false
}

func totpCodeForStep(key []byte, step int64) string {
	var counter [8]byte
	binary.BigEndian.PutUint64(counter[:], uint64(step))
	mac := hmac.New(sha1.New, key)
	_, _ = mac.Write(counter[:])
	sum := mac.Sum(nil)
	offset := sum[len(sum)-1] & 0x0f
	value := (uint32(sum[offset])&0x7f)<<24 | uint32(sum[offset+1])<<16 | uint32(sum[offset+2])<<8 | uint32(sum[offset+3])
	return fmt.Sprintf("%06d", value%1_000_000)
}
