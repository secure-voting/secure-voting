package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRequireTrustedCIDRs_DisabledWhenEmpty(t *testing.T) {
	h := RequireTrustedCIDRs(nil, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	req.RemoteAddr = "198.51.100.10:12345"
	rr := httptest.NewRecorder()

	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rr.Code, rr.Body.String())
	}
}

func TestRequireTrustedCIDRs_AllowsTrustedIP(t *testing.T) {
	h := RequireTrustedCIDRs([]string{"203.0.113.0/24"}, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	req.Header.Set("X-Forwarded-For", "203.0.113.10")
	rr := httptest.NewRecorder()

	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rr.Code, rr.Body.String())
	}
}

func TestRequireTrustedCIDRs_RejectsUntrustedIP(t *testing.T) {
	h := RequireTrustedCIDRs([]string{"203.0.113.0/24"}, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	req.Header.Set("X-Forwarded-For", "198.51.100.10")
	rr := httptest.NewRecorder()

	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d body=%s", rr.Code, rr.Body.String())
	}
}

func TestRequireTrustedCIDRs_InvalidCIDRPanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic for invalid CIDR")
		}
	}()

	_ = RequireTrustedCIDRs([]string{"bad-cidr"}, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
}
