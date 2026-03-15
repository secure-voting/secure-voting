package audit

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	coreaudit "secure-voting/apps/backend/internal/audit"
	"secure-voting/apps/backend/internal/httpserver/middleware"
)

type fakeVerifier struct {
	uid   string
	email string
	role  string
}

func (f fakeVerifier) VerifyAccessToken(ctx context.Context, rawToken string) (userID, email, role string, ok bool, err error) {
	return f.uid, f.email, f.role, true, nil
}

type apiErrResp struct {
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

func decodeErr(t *testing.T, rr *httptest.ResponseRecorder) apiErrResp {
	t.Helper()

	var er apiErrResp
	if err := json.Unmarshal(rr.Body.Bytes(), &er); err != nil {
		t.Fatalf("failed to decode error response: %v; body=%s", err, rr.Body.String())
	}
	return er
}

func timePtr(t time.Time) *time.Time {
	return &t
}

func TestList_Success_ReturnsItemsArray(t *testing.T) {
	h := &Handlers{
		list: func(ctx context.Context, role, uid string, f coreaudit.ListFilter) (any, error) {
			return []map[string]any{
				{"id": 1, "event_type": "election_created"},
				{"id": 2, "event_type": "results_published"},
			}, nil
		},
		parseTime: func(value string) (*time.Time, error) {
			v := time.Date(2026, 3, 12, 20, 0, 0, 0, time.UTC)
			return timePtr(v), nil
		},
	}

	ver := fakeVerifier{uid: "u1", email: "u1@example.com", role: "admin"}
	handler := middleware.RequireAuth(ver, http.HandlerFunc(h.List))

	req := httptest.NewRequest(
		http.MethodGet,
		"http://example/api/v1/audit-log?event_type=election_created&actor_user_id=11111111-1111-1111-1111-111111111111&since=2026-03-12T20:00:00Z&until=2026-03-12T21:00:00Z&limit=10&offset=5",
		nil,
	)
	req.Header.Set("Authorization", "Bearer t")

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rr.Code, rr.Body.String())
	}

	var resp struct {
		Items []map[string]any `json:"items"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v; body=%s", err, rr.Body.String())
	}
	if len(resp.Items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(resp.Items))
	}
}

func TestList_InvalidActorUserID_MapsTo400(t *testing.T) {
	h := &Handlers{
		list: func(ctx context.Context, role, uid string, f coreaudit.ListFilter) (any, error) {
			return nil, nil
		},
		parseTime: func(value string) (*time.Time, error) {
			v := time.Now().UTC()
			return timePtr(v), nil
		},
	}

	ver := fakeVerifier{uid: "u1", email: "u1@example.com", role: "admin"}
	handler := middleware.RequireAuth(ver, http.HandlerFunc(h.List))

	req := httptest.NewRequest(
		http.MethodGet,
		"http://example/api/v1/audit-log?actor_user_id=bad-uuid",
		nil,
	)
	req.Header.Set("Authorization", "Bearer t")

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d body=%s", rr.Code, rr.Body.String())
	}

	er := decodeErr(t, rr)
	if er.Error.Code != "bad_request" || er.Error.Message != "invalid actor_user_id" {
		t.Fatalf("unexpected error response: %+v", er)
	}
}

func TestList_InvalidSince_MapsTo400(t *testing.T) {
	h := &Handlers{
		list: func(ctx context.Context, role, uid string, f coreaudit.ListFilter) (any, error) {
			return nil, nil
		},
		parseTime: func(value string) (*time.Time, error) {
			return nil, errors.New("bad time")
		},
	}

	ver := fakeVerifier{uid: "u1", email: "u1@example.com", role: "admin"}
	handler := middleware.RequireAuth(ver, http.HandlerFunc(h.List))

	req := httptest.NewRequest(
		http.MethodGet,
		"http://example/api/v1/audit-log?since=not-a-time",
		nil,
	)
	req.Header.Set("Authorization", "Bearer t")

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d body=%s", rr.Code, rr.Body.String())
	}

	er := decodeErr(t, rr)
	if er.Error.Code != "bad_request" || er.Error.Message != "invalid since" {
		t.Fatalf("unexpected error response: %+v", er)
	}
}

func TestList_InternalError_MapsTo500(t *testing.T) {
	h := &Handlers{
		list: func(ctx context.Context, role, uid string, f coreaudit.ListFilter) (any, error) {
			return nil, errors.New("boom")
		},
		parseTime: func(value string) (*time.Time, error) {
			v := time.Now().UTC()
			return timePtr(v), nil
		},
	}

	ver := fakeVerifier{uid: "u1", email: "u1@example.com", role: "admin"}
	handler := middleware.RequireAuth(ver, http.HandlerFunc(h.List))

	req := httptest.NewRequest(http.MethodGet, "http://example/api/v1/audit-log", nil)
	req.Header.Set("Authorization", "Bearer t")

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d body=%s", rr.Code, rr.Body.String())
	}

	er := decodeErr(t, rr)
	if er.Error.Code != "internal_error" || er.Error.Message != "list audit log failed" {
		t.Fatalf("unexpected error response: %+v", er)
	}
}