package middleware

import (
	"net/http"

	"secure-voting/apps/backend/internal/httpserver/httputil"
)

func RequireRole(role string, next http.Handler) http.Handler {
	return RequireAnyRole([]string{role}, next)
}

func RequireAnyRole(roles []string, next http.Handler) http.Handler {
	allowed := make(map[string]struct{}, len(roles))
	for _, r := range roles {
		if r == "" {
			continue
		}
		allowed[r] = struct{}{}
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		curRole, ok := RoleFromContext(r.Context())
		if !ok {
			httputil.WriteError(w, http.StatusUnauthorized, "unauthorized", "invalid or expired token")
			return
		}
		if _, ok := allowed[curRole]; !ok {
			httputil.WriteError(w, http.StatusForbidden, "forbidden", "insufficient permissions")
			return
		}
		next.ServeHTTP(w, r)
	})
}
