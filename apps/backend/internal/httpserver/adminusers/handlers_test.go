package adminusers

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

type fakeAdminUsersService struct {
	listFn       func(ctx context.Context, actorUserID string, limit, offset int) ([]asvc.AdminUser, string, error)
	updateRoleFn func(ctx context.Context, actorUserID, targetUserID, newRole string) (asvc.AdminUser, string, error)
}

func (f *fakeAdminUsersService) ListUsers(ctx context.Context, actorUserID string, limit, offset int) ([]asvc.AdminUser, string, error) {
	if f.listFn != nil {
		return f.listFn(ctx, actorUserID, limit, offset)
	}
	return nil, "", nil
}

func (f *fakeAdminUsersService) UpdateUserRole(ctx context.Context, actorUserID, targetUserID, newRole string) (asvc.AdminUser, string, error) {
	if f.updateRoleFn != nil {
		return f.updateRoleFn(ctx, actorUserID, targetUserID, newRole)
	}
	return asvc.AdminUser{}, "", nil
}

func TestList_Unauthorized(t *testing.T) {
	svc := &fakeAdminUsersService{}
	h := NewHandlers(svc)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/users", nil)
	rr := httptest.NewRecorder()

	handler := middleware.RequireAuth(
		fakeTokenVerifier{ok: false},
		httputil.Wrap(h.List),
	)

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d body=%s", rr.Code, rr.Body.String())
	}
}

func TestList_Forbidden(t *testing.T) {
	svc := &fakeAdminUsersService{
		listFn: func(ctx context.Context, actorUserID string, limit, offset int) ([]asvc.AdminUser, string, error) {
			return nil, "forbidden", nil
		},
	}
	h := NewHandlers(svc)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/users", nil)
	req.Header.Set("Authorization", "Bearer token123")
	rr := httptest.NewRecorder()

	handler := middleware.RequireAuth(
		fakeTokenVerifier{
			userID: "admin-1",
			email:  "admin@example.com",
			role:   "admin",
			ok:     true,
		},
		httputil.Wrap(h.List),
	)

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d body=%s", rr.Code, rr.Body.String())
	}
}

func TestList_OK(t *testing.T) {
	svc := &fakeAdminUsersService{
		listFn: func(ctx context.Context, actorUserID string, limit, offset int) ([]asvc.AdminUser, string, error) {
			if actorUserID != "admin-1" {
				t.Fatalf("unexpected actorUserID=%q", actorUserID)
			}
			if limit != 50 {
				t.Fatalf("unexpected limit=%d", limit)
			}
			if offset != 10 {
				t.Fatalf("unexpected offset=%d", offset)
			}

			return []asvc.AdminUser{
				{
					ID:        "u1",
					Email:     "user1@example.com",
					Role:      "voter",
					FullName:  func() *string { s := "Иван Иванов"; return &s }(),
					Phone:     func() *string { s := "+79990000000"; return &s }(),
					CreatedAt: "2026-04-16T10:00:00Z",
				},
				{
					ID:        "u2",
					Email:     "user2@example.com",
					Role:      "researcher",
					CreatedAt: "2026-04-16T11:00:00Z",
				},
			}, "", nil
		},
	}
	h := NewHandlers(svc)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/users?limit=50&offset=10", nil)
	req.Header.Set("Authorization", "Bearer token123")
	rr := httptest.NewRecorder()

	handler := middleware.RequireAuth(
		fakeTokenVerifier{
			userID: "admin-1",
			email:  "admin@example.com",
			role:   "admin",
			ok:     true,
		},
		httputil.Wrap(h.List),
	)

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rr.Code, rr.Body.String())
	}

	var body map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("bad json: %v body=%s", err, rr.Body.String())
	}

	itemsV, ok := body["items"]
	if !ok {
		t.Fatalf("missing items: %#v", body)
	}
	items, ok := itemsV.([]any)
	if !ok || len(items) != 2 {
		t.Fatalf("unexpected items=%#v", itemsV)
	}
}

func TestUpdateRole_InvalidJSON(t *testing.T) {
	svc := &fakeAdminUsersService{}
	h := NewHandlers(svc)

	req := httptest.NewRequest(http.MethodPatch, "/api/v1/admin/users/u1/role", strings.NewReader(`{"role":`))
	req.Header.Set("Authorization", "Bearer token123")
	req.Header.Set("Content-Type", "application/json")
	req.SetPathValue("id", "u1")
	rr := httptest.NewRecorder()

	handler := middleware.RequireAuth(
		fakeTokenVerifier{
			userID: "admin-1",
			email:  "admin@example.com",
			role:   "admin",
			ok:     true,
		},
		httputil.Wrap(h.UpdateRole),
	)

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d body=%s", rr.Code, rr.Body.String())
	}
}

func TestUpdateRole_CodeMapping(t *testing.T) {
	tests := []struct {
		name string
		code string
		want int
	}{
		{name: "forbidden", code: "forbidden", want: http.StatusForbidden},
		{name: "invalid role", code: "invalid_role", want: http.StatusBadRequest},
		{name: "invalid id", code: "invalid_id", want: http.StatusBadRequest},
		{name: "self role change forbidden", code: "self_role_change_forbidden", want: http.StatusBadRequest},
		{name: "not found", code: "not_found", want: http.StatusNotFound},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := &fakeAdminUsersService{
				updateRoleFn: func(ctx context.Context, actorUserID, targetUserID, newRole string) (asvc.AdminUser, string, error) {
					return asvc.AdminUser{}, tt.code, nil
				},
			}
			h := NewHandlers(svc)

			req := httptest.NewRequest(
				http.MethodPatch,
				"/api/v1/admin/users/u2/role",
				strings.NewReader(`{"role":"researcher"}`),
			)
			req.Header.Set("Authorization", "Bearer token123")
			req.Header.Set("Content-Type", "application/json")
			req.SetPathValue("id", "u2")
			rr := httptest.NewRecorder()

			handler := middleware.RequireAuth(
				fakeTokenVerifier{
					userID: "admin-1",
					email:  "admin@example.com",
					role:   "admin",
					ok:     true,
				},
				httputil.Wrap(h.UpdateRole),
			)

			handler.ServeHTTP(rr, req)

			if rr.Code != tt.want {
				t.Fatalf("expected %d, got %d body=%s", tt.want, rr.Code, rr.Body.String())
			}
		})
	}
}

func TestUpdateRole_OK(t *testing.T) {
	svc := &fakeAdminUsersService{
		updateRoleFn: func(ctx context.Context, actorUserID, targetUserID, newRole string) (asvc.AdminUser, string, error) {
			if actorUserID != "admin-1" {
				t.Fatalf("unexpected actorUserID=%q", actorUserID)
			}
			if targetUserID != "u2" {
				t.Fatalf("unexpected targetUserID=%q", targetUserID)
			}
			if newRole != "researcher" {
				t.Fatalf("unexpected newRole=%q", newRole)
			}

			return asvc.AdminUser{
				ID:        "u2",
				Email:     "user2@example.com",
				Role:      "researcher",
				CreatedAt: "2026-04-16T11:00:00Z",
			}, "", nil
		},
	}
	h := NewHandlers(svc)

	req := httptest.NewRequest(
		http.MethodPatch,
		"/api/v1/admin/users/u2/role",
		strings.NewReader(`{"role":"researcher"}`),
	)
	req.Header.Set("Authorization", "Bearer token123")
	req.Header.Set("Content-Type", "application/json")
	req.SetPathValue("id", "u2")
	rr := httptest.NewRecorder()

	handler := middleware.RequireAuth(
		fakeTokenVerifier{
			userID: "admin-1",
			email:  "admin@example.com",
			role:   "admin",
			ok:     true,
		},
		httputil.Wrap(h.UpdateRole),
	)

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rr.Code, rr.Body.String())
	}

	var got asvc.AdminUser
	if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
		t.Fatalf("bad json: %v body=%s", err, rr.Body.String())
	}
	if got.ID != "u2" || got.Role != "researcher" {
		t.Fatalf("unexpected response: %+v", got)
	}
}
