package auth

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	asvc "secure-voting/apps/backend/internal/auth"
	"secure-voting/apps/backend/internal/httpserver/httputil"
	"secure-voting/apps/backend/internal/httpserver/middleware"
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
	lastLogoutToken    string
	lastLogoutActor    *string

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
	f.lastLogoutToken = rawToken
	if actorUserID != nil {
		v := *actorUserID
		f.lastLogoutActor = &v
	} else {
		f.lastLogoutActor = nil
	}
	return f.logoutOK, f.logoutErr
}

func (f *fakeAuthService) ChangePassword(ctx context.Context, userID, currentPassword, newPassword string) (string, error) {
	if f.changePasswordFn != nil {
		return f.changePasswordFn(ctx, userID, currentPassword, newPassword)
	}
	return "", nil
}

type fakeTokenVerifier struct {
	userID string
	email  string
	role   string
	ok     bool
	err    error
}

func (f fakeTokenVerifier) VerifyAccessToken(ctx context.Context, rawToken string) (string, string, string, bool, error) {
	return f.userID, f.email, f.role, f.ok, f.err
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

	req := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/auth/register",
		strings.NewReader(`{"email":"voter1@example.com","password":"S3curePass_2026!","invite_code":"ABC"}`),
	)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	httputil.Wrap(h.Register).ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d, body=%s", rr.Code, rr.Body.String())
	}
	if svc.lastRegisterInvite != "ABC" {
		t.Fatalf("expected invite_code to be passed, got %q", svc.lastRegisterInvite)
	}
	if svc.lastRegisterRole != "" {
		t.Fatalf("expected empty public role, got %q", svc.lastRegisterRole)
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

	req := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/auth/register",
		strings.NewReader(`{"email":"voter1@example.com","password":"S3curePass_2026!"}`),
	)
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

	req := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/auth/login",
		strings.NewReader(`{"email":"voter1@example.com","password":"wrongpass"}`),
	)
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

	req := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/auth/login",
		strings.NewReader(`{"email":"voter1@example.com","password":"S3curePass_2026!","invite_code":"XYZ"}`),
	)
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

func TestMe_Unauthorized(t *testing.T) {
	svc := &fakeAuthService{}
	h := NewHandlers(svc)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
	rr := httptest.NewRecorder()

	httputil.Wrap(h.Me).ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d, body=%s", rr.Code, rr.Body.String())
	}
}

func TestMe_OK(t *testing.T) {
	svc := &fakeAuthService{}
	h := NewHandlers(svc)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
	req.Header.Set("Authorization", "Bearer token123")
	rr := httptest.NewRecorder()

	handler := middleware.RequireAuth(
		fakeTokenVerifier{
			userID: "u1",
			email:  "voter1@example.com",
			role:   "voter",
			ok:     true,
		},
		httputil.Wrap(h.Me),
	)

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d, body=%s", rr.Code, rr.Body.String())
	}

	var got asvc.User
	if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
		t.Fatalf("bad json: %v body=%s", err, rr.Body.String())
	}
	if got.ID != "u1" || got.Email != "voter1@example.com" || got.Role != "voter" {
		t.Fatalf("unexpected response: %+v", got)
	}
}

func TestLogout_Unauthorized(t *testing.T) {
	svc := &fakeAuthService{}
	h := NewHandlers(svc)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	httputil.Wrap(h.Logout).ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d, body=%s", rr.Code, rr.Body.String())
	}
}

func TestLogout_OK(t *testing.T) {
	svc := &fakeAuthService{logoutOK: true}
	h := NewHandlers(svc)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", strings.NewReader(`{}`))
	req.Header.Set("Authorization", "Bearer token123")
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	handler := middleware.RequireAuth(
		fakeTokenVerifier{
			userID: "u1",
			email:  "voter1@example.com",
			role:   "voter",
			ok:     true,
		},
		httputil.Wrap(h.Logout),
	)

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d, body=%s", rr.Code, rr.Body.String())
	}
	if svc.lastLogoutToken != "token123" {
		t.Fatalf("expected token123, got %q", svc.lastLogoutToken)
	}
	if svc.lastLogoutActor == nil || *svc.lastLogoutActor != "u1" {
		t.Fatalf("expected actor u1, got %#v", svc.lastLogoutActor)
	}
}

func TestChangePassword_Unauthorized(t *testing.T) {
	svc := &fakeAuthService{}
	h := NewHandlers(svc)

	req := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/auth/change-password",
		strings.NewReader(`{"current_password":"old-pass-123","new_password":"new-pass-456"}`),
	)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	httputil.Wrap(h.ChangePassword).ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d, body=%s", rr.Code, rr.Body.String())
	}
}

func TestChangePassword_InvalidJSON(t *testing.T) {
	svc := &fakeAuthService{}
	h := NewHandlers(svc)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/change-password", strings.NewReader(`{"current_password":`))
	req.Header.Set("Authorization", "Bearer token123")
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	handler := middleware.RequireAuth(
		fakeTokenVerifier{
			userID: "u1",
			email:  "voter1@example.com",
			role:   "voter",
			ok:     true,
		},
		httputil.Wrap(h.ChangePassword),
	)

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d, body=%s", rr.Code, rr.Body.String())
	}
}

func TestChangePassword_CodeMapping(t *testing.T) {
	tests := []struct {
		name string
		code string
		want int
	}{
		{name: "invalid current password", code: "invalid_current_password", want: http.StatusBadRequest},
		{name: "invalid password", code: "invalid_password", want: http.StatusBadRequest},
		{name: "password unchanged", code: "password_unchanged", want: http.StatusBadRequest},
		{name: "unauthorized", code: "unauthorized", want: http.StatusUnauthorized},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := &fakeAuthService{
				changePasswordFn: func(ctx context.Context, userID, currentPassword, newPassword string) (string, error) {
					return tt.code, nil
				},
			}
			h := NewHandlers(svc)

			req := httptest.NewRequest(
				http.MethodPost,
				"/api/v1/auth/change-password",
				strings.NewReader(`{"current_password":"old-pass-123","new_password":"new-pass-456"}`),
			)
			req.Header.Set("Authorization", "Bearer token123")
			req.Header.Set("Content-Type", "application/json")
			rr := httptest.NewRecorder()

			handler := middleware.RequireAuth(
				fakeTokenVerifier{
					userID: "u1",
					email:  "voter1@example.com",
					role:   "voter",
					ok:     true,
				},
				httputil.Wrap(h.ChangePassword),
			)

			handler.ServeHTTP(rr, req)

			if rr.Code != tt.want {
				t.Fatalf("expected %d, got %d, body=%s", tt.want, rr.Code, rr.Body.String())
			}
		})
	}
}

func TestChangePassword_OK(t *testing.T) {
	svc := &fakeAuthService{
		changePasswordFn: func(ctx context.Context, userID, currentPassword, newPassword string) (string, error) {
			if userID != "u1" {
				t.Fatalf("unexpected userID: %q", userID)
			}
			if currentPassword != "old-pass-123" {
				t.Fatalf("unexpected current password: %q", currentPassword)
			}
			if newPassword != "new-pass-456" {
				t.Fatalf("unexpected new password: %q", newPassword)
			}
			return "", nil
		},
	}
	h := NewHandlers(svc)

	req := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/auth/change-password",
		strings.NewReader(`{"current_password":"old-pass-123","new_password":"new-pass-456"}`),
	)
	req.Header.Set("Authorization", "Bearer token123")
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	handler := middleware.RequireAuth(
		fakeTokenVerifier{
			userID: "u1",
			email:  "voter1@example.com",
			role:   "voter",
			ok:     true,
		},
		httputil.Wrap(h.ChangePassword),
	)

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d, body=%s", rr.Code, rr.Body.String())
	}
}