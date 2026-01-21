package middleware

import (
	"context"
	"net/http"
	"strings"

	"secure-voting/apps/backend/internal/httpserver/httputil"
)

type ctxKey string

const (
	ctxUserID  ctxKey = "user_id"
	ctxEmail   ctxKey = "email"
	ctxRole    ctxKey = "role"
	ctxToken   ctxKey = "token"
)

type TokenVerifier interface {
	VerifyAccessToken(ctx context.Context, rawToken string) (userID, email, role string, ok bool, err error)
}

func RequireAuth(v TokenVerifier, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw := strings.TrimSpace(r.Header.Get("Authorization"))
		if raw == "" || !strings.HasPrefix(raw, "Bearer ") {
			httputil.WriteError(w, http.StatusUnauthorized, "unauthorized", "missing bearer token")
			return
		}
		token := strings.TrimSpace(strings.TrimPrefix(raw, "Bearer "))
		if token == "" {
			httputil.WriteError(w, http.StatusUnauthorized, "unauthorized", "missing bearer token")
			return
		}

		uid, email, role, ok, err := v.VerifyAccessToken(r.Context(), token)
		if err != nil {
			httputil.WriteError(w, http.StatusInternalServerError, "internal_error", "token verification failed")
			return
		}
		if !ok {
			httputil.WriteError(w, http.StatusUnauthorized, "unauthorized", "invalid or expired token")
			return
		}

		ctx := context.WithValue(r.Context(), ctxUserID, uid)
		ctx = context.WithValue(ctx, ctxEmail, email)
		ctx = context.WithValue(ctx, ctxRole, role)
		ctx = context.WithValue(ctx, ctxToken, token)

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func UserIDFromContext(ctx context.Context) (string, bool) {
	v := ctx.Value(ctxUserID)
	s, ok := v.(string)
	return s, ok && s != ""
}

func EmailFromContext(ctx context.Context) (string, bool) {
	v := ctx.Value(ctxEmail)
	s, ok := v.(string)
	return s, ok && s != ""
}

func RoleFromContext(ctx context.Context) (string, bool) {
	v := ctx.Value(ctxRole)
	s, ok := v.(string)
	return s, ok && s != ""
}

func TokenFromContext(ctx context.Context) (string, bool) {
	v := ctx.Value(ctxToken)
	s, ok := v.(string)
	return s, ok && s != ""
}
