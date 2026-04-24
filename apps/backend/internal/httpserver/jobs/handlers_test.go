package jobs

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"secure-voting/apps/backend/internal/httpserver/middleware"
	corejobs "secure-voting/apps/backend/internal/jobs"
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

func TestList_Success_ReturnsItemsArray(t *testing.T) {
	h := &Handlers{
		list: func(ctx context.Context, role, uid string, f corejobs.ListFilter) (any, error) {
			return []map[string]any{
				{"id": "j1", "kind": "experiment_run", "status": "queued"},
				{"id": "j2", "kind": "tally", "status": "done"},
			}, nil
		},
		get: nil,
	}

	ver := fakeVerifier{uid: "u1", email: "u1@example.com", role: "researcher"}
	handler := middleware.RequireAuth(ver, http.HandlerFunc(h.List))

	req := httptest.NewRequest(http.MethodGet, "http://example/api/v1/jobs?status=queued&kind=experiment_run&limit=10&offset=5", nil)
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

func TestList_EmptySlice_ReturnsItemsArray(t *testing.T) {
	h := &Handlers{
		list: func(ctx context.Context, role, uid string, f corejobs.ListFilter) (any, error) {
			return []map[string]any{}, nil
		},
		get: nil,
	}

	ver := fakeVerifier{uid: "u1", email: "u1@example.com", role: "researcher"}
	handler := middleware.RequireAuth(ver, http.HandlerFunc(h.List))

	req := httptest.NewRequest(http.MethodGet, "http://example/api/v1/jobs", nil)
	req.Header.Set("Authorization", "Bearer t")

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rr.Code, rr.Body.String())
	}

	var resp struct {
		Items []json.RawMessage `json:"items"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v; body=%s", err, rr.Body.String())
	}
	if resp.Items == nil {
		t.Fatalf("items is nil, want empty array")
	}
	if len(resp.Items) != 0 {
		t.Fatalf("expected empty items, got %d", len(resp.Items))
	}
}

func TestGet_Success_ReturnsJob(t *testing.T) {
	h := &Handlers{
		list: nil,
		get: func(ctx context.Context, role, uid, id string) (any, string, error) {
			return map[string]any{
				"id":       "job-1",
				"kind":     "experiment_run",
				"status":   "done",
				"progress": 100,
			}, "", nil
		},
	}

	ver := fakeVerifier{uid: "u1", email: "u1@example.com", role: "researcher"}
	handler := middleware.RequireAuth(ver, http.HandlerFunc(h.Get))

	req := httptest.NewRequest(http.MethodGet, "http://example/api/v1/jobs/job-1", nil)
	req.SetPathValue("id", "job-1")
	req.Header.Set("Authorization", "Bearer t")

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rr.Code, rr.Body.String())
	}

	var resp struct {
		ID       string `json:"id"`
		Kind     string `json:"kind"`
		Status   string `json:"status"`
		Progress int    `json:"progress"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v; body=%s", err, rr.Body.String())
	}
	if resp.ID != "job-1" || resp.Kind != "experiment_run" || resp.Status != "done" || resp.Progress != 100 {
		t.Fatalf("unexpected response: %+v", resp)
	}
}

func TestGet_NotFound_MapsTo404(t *testing.T) {
	h := &Handlers{
		list: nil,
		get: func(ctx context.Context, role, uid, id string) (any, string, error) {
			return nil, "not_found", nil
		},
	}

	ver := fakeVerifier{uid: "u1", email: "u1@example.com", role: "researcher"}
	handler := middleware.RequireAuth(ver, http.HandlerFunc(h.Get))

	req := httptest.NewRequest(http.MethodGet, "http://example/api/v1/jobs/missing", nil)
	req.SetPathValue("id", "missing")
	req.Header.Set("Authorization", "Bearer t")

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d body=%s", rr.Code, rr.Body.String())
	}

	er := decodeErr(t, rr)
	if er.Error.Code != "not_found" || er.Error.Message != "job not found" {
		t.Fatalf("unexpected error response: %+v", er)
	}
}
