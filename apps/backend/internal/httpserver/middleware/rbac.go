package middleware

import (
	"net/http"

	"secure-voting/apps/backend/internal/httpserver/httputil"
)

func RequireRole(role string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rv, ok := RoleFromContext(r.Context())
		if !ok || rv != role {
			httputil.WriteError(w, http.StatusForbidden, "forbidden", "insufficient permissions")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func RequireAnyRole(roles []string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rv, ok := RoleFromContext(r.Context())
		if !ok {
			httputil.WriteError(w, http.StatusForbidden, "forbidden", "insufficient permissions")
			return
		}
		for _, want := range roles {
			if rv == want {
				next.ServeHTTP(w, r)
				return
			}
		}
		httputil.WriteError(w, http.StatusForbidden, "forbidden", "insufficient permissions")
	})
}
