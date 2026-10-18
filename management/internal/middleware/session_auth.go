package middleware

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"

	appIAM "velotrust/management/internal/service/iam"
	"velotrust/pkg/jrpc"
)

// SessionIdentityResolver resolves opaque bearer tokens against the IAM
// database on every request. It intentionally has no JWT or cache fallback.
func SessionIdentityResolver(sessions *appIAM.Service) jrpc.IdentityResolver {
	return func(ctx *jrpc.Context) (jrpc.Identity, error) {
		if sessions == nil || ctx == nil || ctx.HttpRequest == nil {
			return nil, nil
		}
		header := strings.TrimSpace(ctx.HttpRequest.Header.Get("Authorization"))
		if header == "" {
			return nil, nil
		}
		kind, token, ok := strings.Cut(header, " ")
		if !ok || !strings.EqualFold(kind, "Bearer") || strings.TrimSpace(token) == "" {
			return nil, errors.New("invalid bearer authorization header")
		}
		session, user, err := sessions.ResolveBearer(ctx.HttpRequest.Context(), strings.TrimSpace(token))
		if err != nil {
			return nil, err
		}
		if session == nil || user == nil {
			return nil, errors.New("invalid IAM session")
		}
		id := strconv.FormatInt(user.ID, 10)
		identity := jrpc.NewDefaultIdentity(id, "", user.Username, session.SessionID, "user")
		identity.SetUserInfo(map[string]any{
			"display_name":         user.DisplayName,
			"auth_level":           session.AuthLevel,
			"must_change_password": user.MustChangePassword,
		})
		return &identity, nil
	}
}

type PermissionChecker interface {
	HasPermission(context.Context, int64, string) (bool, error)
}

type IAMMethodPolicy struct {
	Authenticated bool
	Permission    string
	Public        bool
}

// RequireIAMMethodPolicy gives every registered IAM method an explicit
// access declaration. Any unmapped method is denied by default.
func RequireIAMMethodPolicy(checker PermissionChecker, policies map[string]IAMMethodPolicy) jrpc.HandlerFunc {
	return func(ctx *jrpc.Context) {
		if ctx == nil {
			return
		}
		policy, ok := policies[ctx.ServiceMethod()]
		if !ok {
			ctx.AbortWithError(http.StatusForbidden, errors.New("IAM method has no access policy"))
			return
		}
		if policy.Public {
			ctx.Next()
			return
		}
		if !policy.Authenticated || ctx.Identity == nil {
			ctx.AbortWithError(http.StatusUnauthorized, errors.New("IAM authentication is required"))
			return
		}
		if userInfo, ok := ctx.Identity.UserInfo().(map[string]any); ok {
			if mustChange, _ := userInfo["must_change_password"].(bool); mustChange && ctx.ServiceMethod() != "ChangePassword" && ctx.ServiceMethod() != "GetCurrentUser" && ctx.ServiceMethod() != "Logout" {
				ctx.AbortWithError(http.StatusForbidden, errors.New("password change is required before using this IAM method"))
				return
			}
		}
		if policy.Permission != "" {
			if checker == nil {
				ctx.AbortWithError(http.StatusInternalServerError, errors.New("IAM permission checker is unavailable"))
				return
			}
			userID, err := strconv.ParseInt(ctx.Identity.ID(), 10, 64)
			if err != nil || userID <= 0 {
				ctx.AbortWithError(http.StatusForbidden, errors.New("IAM identity is invalid"))
				return
			}
			allowed, err := checker.HasPermission(ctx.HttpRequest.Context(), userID, policy.Permission)
			if err != nil {
				ctx.AbortWithError(http.StatusInternalServerError, err)
				return
			}
			if !allowed {
				ctx.AbortWithError(http.StatusForbidden, errors.New("IAM permission denied"))
				return
			}
		}
		ctx.Next()
	}
}
