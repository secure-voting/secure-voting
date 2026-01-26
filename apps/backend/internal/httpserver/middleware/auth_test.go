package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"secure-voting/apps/backend/internal/httpserver/httputil"
)

type fakeVerifier struct {
	ok   bool
	role string
}

func (f fakeVerifier) VerifyAccessToken(ctx context.Context, rawToken string) (userID, email, role string, ok bool, err error) {
	if !f.ok {
		return "", "", "", false, nil
	}
	return "u1", "a@b.com", f.role, true, nil
}

func TestRequireAuth_SetsContext(t *testing.T) {
	v := fakeVerifier{ok: true, role: "voter"}

	h := RequireAuth(v, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		uid, _ := UserIDFromContext(r.Context())
		if uid != "u1" {
			httputil.WriteError(w, http.StatusInternalServerError, "internal_error", "bad uid")
			return
		}
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	req.Header.Set("Authorization", "Bearer token123")
	rr := httptest.NewRecorder()

	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rr.Code, rr.Body.String())
	}
}

func TestRequireRole_Forbid(t *testing.T) {
	v := fakeVerifier{ok: true, role: "voter"}

	protected := RequireAuth(v, RequireRole("admin", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})))

	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	req.Header.Set("Authorization", "Bearer token123")
	rr := httptest.NewRecorder()

	protected.ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d body=%s", rr.Code, rr.Body.String())
	}
}

