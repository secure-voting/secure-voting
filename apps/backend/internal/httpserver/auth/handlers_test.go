package auth

import (
	"secure-voting/apps/backend/internal/httpserver/httputil"

	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	asvc "secure-voting/apps/backend/internal/auth"
)

type fakeAuthService struct {
	registerRes  asvc.AuthResult
	registerCode string
	registerErr  error

	loginRes  asvc.AuthResult
	loginCode string
	loginErr  error

	logoutOK  bool
	logoutErr error

	lastRegisterInvite string
	lastRegisterRole   string
	lastLoginInvite    string

	changePasswordFn func(ctx context.Context, userID, currentPassword, newPassword string) (string, error)
}

func (f *fakeAuthService) Register(ctx context.Context, email, password, role, inviteCode string) (asvc.AuthResult, string, error) {
	f.lastRegisterInvite = inviteCode
	f.lastRegisterRole = role
	return f.registerRes, f.registerCode, f.registerErr
}

func (f *fakeAuthService) Login(ctx context.Context, email, password, inviteCode string) (asvc.AuthResult, string, error) {
	f.lastLoginInvite = inviteCode
	return f.loginRes, f.loginCode, f.loginErr
}

func (f *fakeAuthService) Logout(ctx context.Context, rawToken string, actorUserID *string) (bool, error) {
	return f.logoutOK, f.logoutErr
}

func TestRegister_OK(t *testing.T) {
	svc := &fakeAuthService{
		registerRes: asvc.AuthResult{
			AccessToken: "token123",
			ExpiresAt:   "2026-02-01T00:00:00Z",
			User: asvc.User{
				ID:    "u1",
				Email: "voter1@example.com",
				Role:  "voter",
			},
		},
	}
	h := NewHandlers(svc)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/register",
		strings.NewReader(`{"email":"voter1@example.com","password":"S3curePass_2026!","invite_code":"ABC"}`))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	httputil.Wrap(h.Register).ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d, body=%s", rr.Code, rr.Body.String())
	}
	if svc.lastRegisterInvite != "ABC" {
		t.Fatalf("expected invite_code to be passed, got %q", svc.lastRegisterInvite)
	}

	var got asvc.AuthResult
	if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
		t.Fatalf("bad json: %v body=%s", err, rr.Body.String())
	}
	if got.AccessToken != "token123" || got.User.Email != "voter1@example.com" {
		t.Fatalf("unexpected response: %+v", got)
	}
}

func TestRegister_BadJSON(t *testing.T) {
	svc := &fakeAuthService{}
	h := NewHandlers(svc)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", strings.NewReader(`{"email":`))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	httputil.Wrap(h.Register).ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d, body=%s", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), `"bad_request"`) {
		t.Fatalf("expected bad_request error, body=%s", rr.Body.String())
	}
}

func TestRegister_EmailTaken(t *testing.T) {
	svc := &fakeAuthService{registerCode: "email_taken"}
	h := NewHandlers(svc)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/register",
		strings.NewReader(`{"email":"voter1@example.com","password":"S3curePass_2026!"}`))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	httputil.Wrap(h.Register).ServeHTTP(rr, req)

	if rr.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d, body=%s", rr.Code, rr.Body.String())
	}
}

func TestLogin_Unauthorized(t *testing.T) {
	svc := &fakeAuthService{loginCode: "invalid_credentials"}
	h := NewHandlers(svc)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login",
		strings.NewReader(`{"email":"voter1@example.com","password":"wrongpass"}`))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	httputil.Wrap(h.Login).ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d, body=%s", rr.Code, rr.Body.String())
	}
	if svc.lastLoginInvite != "" {
		t.Fatalf("expected empty invite_code, got %q", svc.lastLoginInvite)
	}
}

func TestLogin_InvitePassed(t *testing.T) {
	svc := &fakeAuthService{
		loginRes: asvc.AuthResult{
			AccessToken: "token999",
			ExpiresAt:   "2026-02-01T00:00:00Z",
			User: asvc.User{
				ID:    "u1",
				Email: "voter1@example.com",
				Role:  "voter",
			},
		},
	}
	h := NewHandlers(svc)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login",
		strings.NewReader(`{"email":"voter1@example.com","password":"S3curePass_2026!","invite_code":"XYZ"}`))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	httputil.Wrap(h.Login).ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d, body=%s", rr.Code, rr.Body.String())
	}
	if svc.lastLoginInvite != "XYZ" {
		t.Fatalf("expected invite_code to be passed, got %q", svc.lastLoginInvite)
	}
}

func (s *fakeAuthService) ChangePassword(ctx context.Context, userID, currentPassword, newPassword string) (string, error) {
	if s.changePasswordFn != nil {
		return s.changePasswordFn(ctx, userID, currentPassword, newPassword)
	}
	return "", nil
}
