package results

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"secure-voting/apps/backend/internal/httpserver/httputil"
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

func TestGet_Success_ReturnsResultJSON(t *testing.T) {
	h := &Handlers{
		get: func(ctx context.Context, electionID, role, userID, email string) (any, string, error) {
			return map[string]any{
				"election_id":  "11111111-1111-1111-1111-111111111111",
				"version":      1,
				"method":       "plurality",
				"winners":      []string{"c1"},
				"params":       map[string]any{"committee_size": 1},
				"metrics":      map[string]any{"ballots": 10},
				"protocol":     []any{},
				"published_at": "2026-03-12T20:30:00Z",
			}, "", nil
		},
	}

	ver := fakeVerifier{uid: "u1", email: "u1@example.com", role: "admin"}
	handler := middleware.RequireAuth(ver, httputil.Wrap(h.Get))

	req := httptest.NewRequest(http.MethodGet, "http://example/api/v1/elections/e1/results", nil)
	req.SetPathValue("id", "e1")
	req.Header.Set("Authorization", "Bearer t")

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rr.Code, rr.Body.String())
	}

	var resp struct {
		ElectionID  string   `json:"election_id"`
		Version     int      `json:"version"`
		Method      string   `json:"method"`
		Winners     []string `json:"winners"`
		PublishedAt string   `json:"published_at"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v; body=%s", err, rr.Body.String())
	}

	if resp.ElectionID != "11111111-1111-1111-1111-111111111111" {
		t.Fatalf("unexpected election_id: %q", resp.ElectionID)
	}
	if resp.Version != 1 {
		t.Fatalf("unexpected version: %d", resp.Version)
	}
	if resp.Method != "plurality" {
		t.Fatalf("unexpected method: %q", resp.Method)
	}
	if len(resp.Winners) != 1 || resp.Winners[0] != "c1" {
		t.Fatalf("unexpected winners: %+v", resp.Winners)
	}
	if resp.PublishedAt != "2026-03-12T20:30:00Z" {
		t.Fatalf("unexpected published_at: %q", resp.PublishedAt)
	}
}

func TestGet_InvalidID_MapsTo400(t *testing.T) {
	h := &Handlers{
		get: func(ctx context.Context, electionID, role, userID, email string) (any, string, error) {
			return nil, "invalid_id", nil
		},
	}

	ver := fakeVerifier{uid: "u1", email: "u1@example.com", role: "voter"}
	handler := middleware.RequireAuth(ver, httputil.Wrap(h.Get))

	req := httptest.NewRequest(http.MethodGet, "http://example/api/v1/elections/bad/results", nil)
	req.SetPathValue("id", "bad")
	req.Header.Set("Authorization", "Bearer t")

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d body=%s", rr.Code, rr.Body.String())
	}

	er := decodeErr(t, rr)
	if er.Error.Code != "invalid_id" {
		t.Fatalf("unexpected error code: %+v", er)
	}
	if er.Error.Message != "invalid election id" {
		t.Fatalf("unexpected error message: %+v", er)
	}
}

func TestGet_NotFound_MapsTo404(t *testing.T) {
	h := &Handlers{
		get: func(ctx context.Context, electionID, role, userID, email string) (any, string, error) {
			return nil, "not_found", nil
		},
	}

	ver := fakeVerifier{uid: "u1", email: "u1@example.com", role: "voter"}
	handler := middleware.RequireAuth(ver, httputil.Wrap(h.Get))

	req := httptest.NewRequest(http.MethodGet, "http://example/api/v1/elections/missing/results", nil)
	req.SetPathValue("id", "missing")
	req.Header.Set("Authorization", "Bearer t")

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d body=%s", rr.Code, rr.Body.String())
	}

	er := decodeErr(t, rr)
	if er.Error.Code != "not_found" {
		t.Fatalf("unexpected error code: %+v", er)
	}
	if er.Error.Message != "election not found" {
		t.Fatalf("unexpected error message: %+v", er)
	}
}

func TestGet_NotPublished_MapsTo403(t *testing.T) {
	h := &Handlers{
		get: func(ctx context.Context, electionID, role, userID, email string) (any, string, error) {
			return nil, "not_published", nil
		},
	}

	ver := fakeVerifier{uid: "u1", email: "u1@example.com", role: "voter"}
	handler := middleware.RequireAuth(ver, httputil.Wrap(h.Get))

	req := httptest.NewRequest(http.MethodGet, "http://example/api/v1/elections/e1/results", nil)
	req.SetPathValue("id", "e1")
	req.Header.Set("Authorization", "Bearer t")

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d body=%s", rr.Code, rr.Body.String())
	}

	er := decodeErr(t, rr)
	if er.Error.Code != "not_published" {
		t.Fatalf("unexpected error code: %+v", er)
	}
	if er.Error.Message != "results not published" {
		t.Fatalf("unexpected error message: %+v", er)
	}
}

func TestGet_PassesAuthContextInServiceOrder(t *testing.T) {
	var gotElectionID string
	var gotRole string
	var gotUserID string
	var gotEmail string

	h := &Handlers{
		get: func(ctx context.Context, electionID, role, userID, email string) (any, string, error) {
			gotElectionID = electionID
			gotRole = role
			gotUserID = userID
			gotEmail = email

			return map[string]any{
				"election_id":  electionID,
				"version":      1,
				"method":       "score",
				"winners":      []string{"c1"},
				"published_at": "2026-05-03T10:39:00Z",
			}, "", nil
		},
	}

	ver := fakeVerifier{
		uid:   "user-123",
		email: "user@example.com",
		role:  "voter",
	}

	handler := middleware.RequireAuth(ver, httputil.Wrap(h.Get))

	req := httptest.NewRequest(http.MethodGet, "http://example/api/v1/elections/election-1/results", nil)
	req.SetPathValue("id", "election-1")
	req.Header.Set("Authorization", "Bearer t")

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rr.Code, rr.Body.String())
	}

	if gotElectionID != "election-1" {
		t.Fatalf("unexpected election id: %q", gotElectionID)
	}
	if gotRole != "voter" {
		t.Fatalf("unexpected role: %q", gotRole)
	}
	if gotUserID != "user-123" {
		t.Fatalf("unexpected user id: %q", gotUserID)
	}
	if gotEmail != "user@example.com" {
		t.Fatalf("unexpected email: %q", gotEmail)
	}
}
