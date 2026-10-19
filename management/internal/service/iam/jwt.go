package iam

import (
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const (
	iamJWTIssuer   = "velotrust-management"
	iamJWTAudience = "velotrust-management-api"
	maxJWTLength   = 4096
)

func issueAccessToken(signingKey []byte, userID int64, sessionID string, issuedAt, expiresAt time.Time) (string, error) {
	if len(signingKey) < 32 || userID <= 0 || sessionID == "" || !expiresAt.After(issuedAt) {
		return "", errors.New("invalid IAM JWT signing input")
	}
	claims := jwt.RegisteredClaims{
		Issuer:    iamJWTIssuer,
		Subject:   strconv.FormatInt(userID, 10),
		Audience:  jwt.ClaimStrings{iamJWTAudience},
		ExpiresAt: jwt.NewNumericDate(expiresAt),
		IssuedAt:  jwt.NewNumericDate(issuedAt),
		ID:        sessionID,
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(signingKey)
}

func parseAccessToken(signingKey []byte, raw string) (*jwt.RegisteredClaims, error) {
	if len(signingKey) < 32 || len(raw) == 0 || len(raw) > maxJWTLength {
		return nil, errors.New("invalid IAM JWT")
	}
	claims := new(jwt.RegisteredClaims)
	token, err := jwt.ParseWithClaims(raw, claims, func(token *jwt.Token) (any, error) {
		if token.Method.Alg() != jwt.SigningMethodHS256.Alg() {
			return nil, fmt.Errorf("unexpected IAM JWT signing algorithm %q", token.Method.Alg())
		}
		return signingKey, nil
	},
		jwt.WithIssuer(iamJWTIssuer),
		jwt.WithAudience(iamJWTAudience),
		jwt.WithExpirationRequired(),
		jwt.WithIssuedAt(),
	)
	if err != nil {
		return nil, fmt.Errorf("validate IAM JWT: %w", err)
	}
	if token == nil || !token.Valid || claims.ID == "" || claims.Subject == "" || claims.IssuedAt == nil {
		return nil, errors.New("IAM JWT is missing required claims")
	}
	return claims, nil
}

func parseAccessTokenSubject(subject string) (int64, error) {
	userID, err := strconv.ParseInt(subject, 10, 64)
	if err != nil || userID <= 0 {
		return 0, errors.New("invalid IAM JWT subject")
	}
	return userID, nil
}
