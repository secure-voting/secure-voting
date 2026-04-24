package adminsettings

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	ssvc "secure-voting/apps/backend/internal/adminsettings"
	"secure-voting/apps/backend/internal/httpserver/httputil"
	"secure-voting/apps/backend/internal/httpserver/middleware"
)

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

type fakeAdminSettingsService struct {
	getFn    func(ctx context.Context) (ssvc.Settings, error)
	updateFn func(ctx context.Context, in ssvc.UpdateInput) (ssvc.Settings, string, error)
}

func (f *fakeAdminSettingsService) Get(ctx context.Context) (ssvc.Settings, error) {
	if f.getFn != nil {
		return f.getFn(ctx)
	}
	return ssvc.Settings{}, nil
}

func (f *fakeAdminSettingsService) Update(ctx context.Context, in ssvc.UpdateInput) (ssvc.Settings, string, error) {
	if f.updateFn != nil {
		return f.updateFn(ctx, in)
	}
	return ssvc.Settings{}, "", nil
}

func TestGet_Unauthorized(t *testing.T) {
	svc := &fakeAdminSettingsService{}
	h := NewHandlers(svc)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/settings", nil)
	rr := httptest.NewRecorder()

	handler := middleware.RequireAuth(
		fakeTokenVerifier{ok: false},
		httputil.Wrap(h.Get),
	)

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d body=%s", rr.Code, rr.Body.String())
	}
}

func TestGet_OK(t *testing.T) {
	svc := &fakeAdminSettingsService{
		getFn: func(ctx context.Context) (ssvc.Settings, error) {
			return ssvc.Settings{
				PublicBaseURL:       func() *string { s := "https://vote.example.com"; return &s }(),
				TLSMode:             "lets_encrypt",
				TLSDomain:           func() *string { s := "vote.example.com"; return &s }(),
				TLSContactEmail:     func() *string { s := "admin@example.com"; return &s }(),
				BackupEnabled:       true,
				BackupSchedule:      func() *string { s := "daily 02:00"; return &s }(),
				BackupRetentionDays: func() *int { v := 7; return &v }(),
				DatabaseHost:        func() *string { s := "postgres-db"; return &s }(),
				DatabaseName:        func() *string { s := "secure_voting"; return &s }(),
				UpdatedAt:           "2026-04-16T10:00:00Z",
				HasUnsavedWarning:   true,
			}, nil
		},
	}
	h := NewHandlers(svc)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/settings", nil)
	req.Header.Set("Authorization", "Bearer token123")
	rr := httptest.NewRecorder()

	handler := middleware.RequireAuth(
		fakeTokenVerifier{
			userID: "admin-1",
			email:  "admin@example.com",
			role:   "admin",
			ok:     true,
		},
		httputil.Wrap(h.Get),
	)

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rr.Code, rr.Body.String())
	}

	var got ssvc.Settings
	if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
		t.Fatalf("bad json: %v body=%s", err, rr.Body.String())
	}
	if got.TLSMode != "lets_encrypt" {
		t.Fatalf("unexpected tls_mode=%q", got.TLSMode)
	}
	if got.PublicBaseURL == nil || *got.PublicBaseURL != "https://vote.example.com" {
		t.Fatalf("unexpected public_base_url=%#v", got.PublicBaseURL)
	}
	if !got.BackupEnabled {
		t.Fatal("expected backup_enabled=true")
	}
	if got.DatabaseName == nil || *got.DatabaseName != "secure_voting" {
		t.Fatalf("unexpected database_name=%#v", got.DatabaseName)
	}
}

func TestUpdate_Unauthorized(t *testing.T) {
	svc := &fakeAdminSettingsService{}
	h := NewHandlers(svc)

	req := httptest.NewRequest(
		http.MethodPut,
		"/api/v1/admin/settings",
		strings.NewReader(`{"tls_mode":"disabled","backup_enabled":false}`),
	)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	handler := middleware.RequireAuth(
		fakeTokenVerifier{ok: false},
		httputil.Wrap(h.Update),
	)

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d body=%s", rr.Code, rr.Body.String())
	}
}

func TestUpdate_InvalidJSON(t *testing.T) {
	svc := &fakeAdminSettingsService{}
	h := NewHandlers(svc)

	req := httptest.NewRequest(http.MethodPut, "/api/v1/admin/settings", strings.NewReader(`{"tls_mode":`))
	req.Header.Set("Authorization", "Bearer token123")
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	handler := middleware.RequireAuth(
		fakeTokenVerifier{
			userID: "admin-1",
			email:  "admin@example.com",
			role:   "admin",
			ok:     true,
		},
		httputil.Wrap(h.Update),
	)

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d body=%s", rr.Code, rr.Body.String())
	}
}

func TestUpdate_CodeMapping(t *testing.T) {
	tests := []struct {
		name string
		code string
		want int
	}{
		{name: "invalid tls mode", code: "invalid_tls_mode", want: http.StatusBadRequest},
		{name: "invalid tls contact email", code: "invalid_tls_contact_email", want: http.StatusBadRequest},
		{name: "invalid backup retention days", code: "invalid_backup_retention_days", want: http.StatusBadRequest},
		{name: "unauthorized", code: "unauthorized", want: http.StatusUnauthorized},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := &fakeAdminSettingsService{
				updateFn: func(ctx context.Context, in ssvc.UpdateInput) (ssvc.Settings, string, error) {
					return ssvc.Settings{}, tt.code, nil
				},
			}
			h := NewHandlers(svc)

			req := httptest.NewRequest(
				http.MethodPut,
				"/api/v1/admin/settings",
				strings.NewReader(`{
					"public_base_url":"https://vote.example.com",
					"tls_mode":"lets_encrypt",
					"tls_domain":"vote.example.com",
					"tls_contact_email":"admin@example.com",
					"backup_enabled":true,
					"backup_schedule":"daily 02:00",
					"backup_retention_days":7,
					"database_host":"postgres-db",
					"database_name":"secure_voting"
				}`),
			)
			req.Header.Set("Authorization", "Bearer token123")
			req.Header.Set("Content-Type", "application/json")
			rr := httptest.NewRecorder()

			handler := middleware.RequireAuth(
				fakeTokenVerifier{
					userID: "admin-1",
					email:  "admin@example.com",
					role:   "admin",
					ok:     true,
				},
				httputil.Wrap(h.Update),
			)

			handler.ServeHTTP(rr, req)

			if rr.Code != tt.want {
				t.Fatalf("expected %d, got %d body=%s", tt.want, rr.Code, rr.Body.String())
			}
		})
	}
}

func TestUpdate_OK(t *testing.T) {
	svc := &fakeAdminSettingsService{
		updateFn: func(ctx context.Context, in ssvc.UpdateInput) (ssvc.Settings, string, error) {
			if in.ActorUserID != "admin-1" {
				t.Fatalf("unexpected actorUserID=%q", in.ActorUserID)
			}
			if in.TLSMode != "lets_encrypt" {
				t.Fatalf("unexpected tls_mode=%q", in.TLSMode)
			}
			if in.PublicBaseURL != "https://vote.example.com" {
				t.Fatalf("unexpected public_base_url=%q", in.PublicBaseURL)
			}
			if in.TLSDomain != "vote.example.com" {
				t.Fatalf("unexpected tls_domain=%q", in.TLSDomain)
			}
			if in.TLSContactEmail != "admin@example.com" {
				t.Fatalf("unexpected tls_contact_email=%q", in.TLSContactEmail)
			}
			if !in.BackupEnabled {
				t.Fatal("expected backup_enabled=true")
			}
			if in.BackupSchedule != "daily 02:00" {
				t.Fatalf("unexpected backup_schedule=%q", in.BackupSchedule)
			}
			if in.BackupRetentionDays == nil || *in.BackupRetentionDays != 7 {
				t.Fatalf("unexpected backup_retention_days=%#v", in.BackupRetentionDays)
			}
			if in.DatabaseHost != "postgres-db" {
				t.Fatalf("unexpected database_host=%q", in.DatabaseHost)
			}
			if in.DatabaseName != "secure_voting" {
				t.Fatalf("unexpected database_name=%q", in.DatabaseName)
			}

			return ssvc.Settings{
				PublicBaseURL:       func() *string { s := in.PublicBaseURL; return &s }(),
				TLSMode:             in.TLSMode,
				TLSDomain:           func() *string { s := in.TLSDomain; return &s }(),
				TLSContactEmail:     func() *string { s := in.TLSContactEmail; return &s }(),
				BackupEnabled:       in.BackupEnabled,
				BackupSchedule:      func() *string { s := in.BackupSchedule; return &s }(),
				BackupRetentionDays: in.BackupRetentionDays,
				DatabaseHost:        func() *string { s := in.DatabaseHost; return &s }(),
				DatabaseName:        func() *string { s := in.DatabaseName; return &s }(),
				UpdatedAt:           "2026-04-16T12:00:00Z",
				HasUnsavedWarning:   true,
			}, "", nil
		},
	}
	h := NewHandlers(svc)

	req := httptest.NewRequest(
		http.MethodPut,
		"/api/v1/admin/settings",
		strings.NewReader(`{
			"public_base_url":"https://vote.example.com",
			"tls_mode":"lets_encrypt",
			"tls_domain":"vote.example.com",
			"tls_contact_email":"admin@example.com",
			"backup_enabled":true,
			"backup_schedule":"daily 02:00",
			"backup_retention_days":7,
			"database_host":"postgres-db",
			"database_name":"secure_voting"
		}`),
	)
	req.Header.Set("Authorization", "Bearer token123")
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	handler := middleware.RequireAuth(
		fakeTokenVerifier{
			userID: "admin-1",
			email:  "admin@example.com",
			role:   "admin",
			ok:     true,
		},
		httputil.Wrap(h.Update),
	)

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rr.Code, rr.Body.String())
	}

	var got ssvc.Settings
	if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
		t.Fatalf("bad json: %v body=%s", err, rr.Body.String())
	}
	if got.TLSMode != "lets_encrypt" {
		t.Fatalf("unexpected tls_mode=%q", got.TLSMode)
	}
	if got.DatabaseName == nil || *got.DatabaseName != "secure_voting" {
		t.Fatalf("unexpected database_name=%#v", got.DatabaseName)
	}
	if got.BackupRetentionDays == nil || *got.BackupRetentionDays != 7 {
		t.Fatalf("unexpected backup_retention_days=%#v", got.BackupRetentionDays)
	}
}
