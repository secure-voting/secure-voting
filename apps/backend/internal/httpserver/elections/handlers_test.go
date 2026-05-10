package elections

import (
	"secure-voting/apps/backend/internal/httpserver/httputil"

	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"secure-voting/apps/backend/internal/elections"
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

type fakeSvc struct {
	createFn           func(ctx context.Context, createdBy string, in elections.CreateElectionInput) (string, string, error)
	listFn             func(ctx context.Context, userID, email, role string) ([]elections.ElectionSummary, error)
	getFn              func(ctx context.Context, electionID, userID, email, role string) (elections.ElectionDetail, string, error)
	ballotMetaFn       func(ctx context.Context, electionID, userID, email, role string) (elections.BallotMeta, string, error)
	updateRulesFn      func(ctx context.Context, electionID, adminUserID string, in elections.UpdateRulesInput) (string, error)
	actionFn           func(ctx context.Context, electionID, adminUserID, action string) (string, error)
	createInvFn        func(ctx context.Context, electionID, adminUserID, email string) (elections.InviteCreated, string, error)
	listInvFn          func(ctx context.Context, electionID, adminUserID string) ([]elections.Invite, string, error)
	importCandidatesFn func(ctx context.Context, filename string, content []byte) ([]elections.CandidateNormalized, string, error)
}

func (f fakeSvc) Create(ctx context.Context, createdBy string, in elections.CreateElectionInput) (string, string, error) {
	return f.createFn(ctx, createdBy, in)
}

func (f fakeSvc) ListForUser(ctx context.Context, userID, email, role string) ([]elections.ElectionSummary, error) {
	return f.listFn(ctx, userID, email, role)
}

func (f fakeSvc) Get(ctx context.Context, electionID, userID, email, role string) (elections.ElectionDetail, string, error) {
	return f.getFn(ctx, electionID, userID, email, role)
}

func (f fakeSvc) GetBallotMeta(ctx context.Context, electionID, userID, email, role string) (elections.BallotMeta, string, error) {
	return f.ballotMetaFn(ctx, electionID, userID, email, role)
}

func (f fakeSvc) UpdateRules(ctx context.Context, electionID, adminUserID string, in elections.UpdateRulesInput) (string, error) {
	return f.updateRulesFn(ctx, electionID, adminUserID, in)
}

func (f fakeSvc) Action(ctx context.Context, electionID, adminUserID, action string) (string, error) {
	return f.actionFn(ctx, electionID, adminUserID, action)
}

func (f fakeSvc) CreateInvite(ctx context.Context, electionID, adminUserID, email string) (elections.InviteCreated, string, error) {
	return f.createInvFn(ctx, electionID, adminUserID, email)
}

func (f fakeSvc) ListInvites(ctx context.Context, electionID, adminUserID string) ([]elections.Invite, string, error) {
	return f.listInvFn(ctx, electionID, adminUserID)
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

func newBaseFakeSvc() fakeSvc {
	return fakeSvc{
		createFn: func(ctx context.Context, createdBy string, in elections.CreateElectionInput) (string, string, error) {
			return "", "", nil
		},
		listFn: func(ctx context.Context, userID, email, role string) ([]elections.ElectionSummary, error) {
			return nil, nil
		},
		getFn: func(ctx context.Context, electionID, userID, email, role string) (elections.ElectionDetail, string, error) {
			return elections.ElectionDetail{}, "", nil
		},
		ballotMetaFn: func(ctx context.Context, electionID, userID, email, role string) (elections.BallotMeta, string, error) {
			return elections.BallotMeta{}, "", nil
		},
		updateRulesFn: func(ctx context.Context, electionID, adminUserID string, in elections.UpdateRulesInput) (string, error) {
			return "", nil
		},
		actionFn: func(ctx context.Context, electionID, adminUserID, action string) (string, error) {
			return "", nil
		},
		createInvFn: func(ctx context.Context, electionID, adminUserID, email string) (elections.InviteCreated, string, error) {
			return elections.InviteCreated{}, "", nil
		},
		listInvFn: func(ctx context.Context, electionID, adminUserID string) ([]elections.Invite, string, error) {
			return nil, "", nil
		},
	}
}

func TestGet_InvalidID_MapsTo400(t *testing.T) {
	svc := fakeSvc{
		getFn: func(ctx context.Context, electionID, userID, email, role string) (elections.ElectionDetail, string, error) {
			return elections.ElectionDetail{}, "invalid_id", nil
		},
		listFn: func(ctx context.Context, userID, email, role string) ([]elections.ElectionSummary, error) {
			return nil, nil
		},
		createFn: func(ctx context.Context, createdBy string, in elections.CreateElectionInput) (string, string, error) {
			return "", "", nil
		},
		ballotMetaFn: func(ctx context.Context, electionID, userID, email, role string) (elections.BallotMeta, string, error) {
			return elections.BallotMeta{}, "", nil
		},
		updateRulesFn: func(ctx context.Context, electionID, adminUserID string, in elections.UpdateRulesInput) (string, error) {
			return "", nil
		},
		actionFn: func(ctx context.Context, electionID, adminUserID, action string) (string, error) { return "", nil },
		createInvFn: func(ctx context.Context, electionID, adminUserID, email string) (elections.InviteCreated, string, error) {
			return elections.InviteCreated{}, "", nil
		},
		listInvFn: func(ctx context.Context, electionID, adminUserID string) ([]elections.Invite, string, error) {
			return nil, "", nil
		},
	}

	h := NewHandlers(svc)
	ver := fakeVerifier{uid: "u1", email: "u1@example.com", role: "voter"}
	handler := middleware.RequireAuth(ver, httputil.Wrap(h.Get))

	req := httptest.NewRequest(http.MethodGet, "http://example/api/v1/elections/bad", nil)
	req.SetPathValue("id", "bad")
	req.Header.Set("Authorization", "Bearer t")

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d body=%s", rr.Code, rr.Body.String())
	}
	er := decodeErr(t, rr)
	if er.Error.Code != "bad_request" || er.Error.Message != "invalid id" {
		t.Fatalf("unexpected err: %+v", er)
	}
}

func TestGet_NotFound_MapsTo404(t *testing.T) {
	svc := fakeSvc{
		getFn: func(ctx context.Context, electionID, userID, email, role string) (elections.ElectionDetail, string, error) {
			return elections.ElectionDetail{}, "not_found", nil
		},
		listFn: func(ctx context.Context, userID, email, role string) ([]elections.ElectionSummary, error) {
			return nil, nil
		},
		createFn: func(ctx context.Context, createdBy string, in elections.CreateElectionInput) (string, string, error) {
			return "", "", nil
		},
		ballotMetaFn: func(ctx context.Context, electionID, userID, email, role string) (elections.BallotMeta, string, error) {
			return elections.BallotMeta{}, "", nil
		},
		updateRulesFn: func(ctx context.Context, electionID, adminUserID string, in elections.UpdateRulesInput) (string, error) {
			return "", nil
		},
		actionFn: func(ctx context.Context, electionID, adminUserID, action string) (string, error) { return "", nil },
		createInvFn: func(ctx context.Context, electionID, adminUserID, email string) (elections.InviteCreated, string, error) {
			return elections.InviteCreated{}, "", nil
		},
		listInvFn: func(ctx context.Context, electionID, adminUserID string) ([]elections.Invite, string, error) {
			return nil, "", nil
		},
	}

	h := NewHandlers(svc)
	ver := fakeVerifier{uid: "u1", email: "u1@example.com", role: "voter"}
	handler := middleware.RequireAuth(ver, httputil.Wrap(h.Get))

	req := httptest.NewRequest(http.MethodGet, "http://example/api/v1/elections/xxx", nil)
	req.SetPathValue("id", "xxx")
	req.Header.Set("Authorization", "Bearer t")

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d body=%s", rr.Code, rr.Body.String())
	}
}

func TestUpdateRules_InvalidStatus_MapsTo409(t *testing.T) {
	svc := fakeSvc{
		updateRulesFn: func(ctx context.Context, electionID, adminUserID string, in elections.UpdateRulesInput) (string, error) {
			return "invalid_status", nil
		},
		getFn: func(ctx context.Context, electionID, userID, email, role string) (elections.ElectionDetail, string, error) {
			return elections.ElectionDetail{}, "", nil
		},
		listFn: func(ctx context.Context, userID, email, role string) ([]elections.ElectionSummary, error) {
			return nil, nil
		},
		createFn: func(ctx context.Context, createdBy string, in elections.CreateElectionInput) (string, string, error) {
			return "", "", nil
		},
		ballotMetaFn: func(ctx context.Context, electionID, userID, email, role string) (elections.BallotMeta, string, error) {
			return elections.BallotMeta{}, "", nil
		},
		actionFn: func(ctx context.Context, electionID, adminUserID, action string) (string, error) { return "", nil },
		createInvFn: func(ctx context.Context, electionID, adminUserID, email string) (elections.InviteCreated, string, error) {
			return elections.InviteCreated{}, "", nil
		},
		listInvFn: func(ctx context.Context, electionID, adminUserID string) ([]elections.Invite, string, error) {
			return nil, "", nil
		},
	}

	h := NewHandlers(svc)
	ver := fakeVerifier{uid: "admin1", email: "a@example.com", role: "admin"}
	handler := middleware.RequireAuth(ver, middleware.RequireRole("admin", httputil.Wrap(h.UpdateRules)))

	req := httptest.NewRequest(http.MethodPut, "http://example/api/v1/elections/e1/rules", bytes.NewReader([]byte(`{}`)))
	req.SetPathValue("id", "e1")
	req.Header.Set("Authorization", "Bearer t")
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d body=%s", rr.Code, rr.Body.String())
	}
	er := decodeErr(t, rr)
	if er.Error.Code != "conflict" {
		t.Fatalf("unexpected err: %+v", er)
	}
}

func TestAction_InvalidTransition_MapsTo409(t *testing.T) {
	svc := fakeSvc{
		actionFn: func(ctx context.Context, electionID, adminUserID, action string) (string, error) {
			return "invalid_transition", nil
		},
		getFn: func(ctx context.Context, electionID, userID, email, role string) (elections.ElectionDetail, string, error) {
			return elections.ElectionDetail{}, "", nil
		},
		listFn: func(ctx context.Context, userID, email, role string) ([]elections.ElectionSummary, error) {
			return nil, nil
		},
		createFn: func(ctx context.Context, createdBy string, in elections.CreateElectionInput) (string, string, error) {
			return "", "", nil
		},
		ballotMetaFn: func(ctx context.Context, electionID, userID, email, role string) (elections.BallotMeta, string, error) {
			return elections.BallotMeta{}, "", nil
		},
		updateRulesFn: func(ctx context.Context, electionID, adminUserID string, in elections.UpdateRulesInput) (string, error) {
			return "", nil
		},
		createInvFn: func(ctx context.Context, electionID, adminUserID, email string) (elections.InviteCreated, string, error) {
			return elections.InviteCreated{}, "", nil
		},
		listInvFn: func(ctx context.Context, electionID, adminUserID string) ([]elections.Invite, string, error) {
			return nil, "", nil
		},
	}

	h := NewHandlers(svc)
	ver := fakeVerifier{uid: "admin1", email: "a@example.com", role: "admin"}
	handler := middleware.RequireAuth(ver, middleware.RequireRole("admin", httputil.Wrap(h.Action)))

	req := httptest.NewRequest(http.MethodPost, "http://example/api/v1/elections/e1/actions/open", nil)
	req.SetPathValue("id", "e1")
	req.SetPathValue("action", "open")
	req.Header.Set("Authorization", "Bearer t")

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d body=%s", rr.Code, rr.Body.String())
	}
}

func TestCreateInvite_EmailAlreadyInvited_MapsTo409(t *testing.T) {
	svc := fakeSvc{
		createInvFn: func(ctx context.Context, electionID, adminUserID, email string) (elections.InviteCreated, string, error) {
			return elections.InviteCreated{}, "email_already_invited", nil
		},
		getFn: func(ctx context.Context, electionID, userID, email, role string) (elections.ElectionDetail, string, error) {
			return elections.ElectionDetail{}, "", nil
		},
		listFn: func(ctx context.Context, userID, email, role string) ([]elections.ElectionSummary, error) {
			return nil, nil
		},
		createFn: func(ctx context.Context, createdBy string, in elections.CreateElectionInput) (string, string, error) {
			return "", "", nil
		},
		ballotMetaFn: func(ctx context.Context, electionID, userID, email, role string) (elections.BallotMeta, string, error) {
			return elections.BallotMeta{}, "", nil
		},
		updateRulesFn: func(ctx context.Context, electionID, adminUserID string, in elections.UpdateRulesInput) (string, error) {
			return "", nil
		},
		actionFn: func(ctx context.Context, electionID, adminUserID, action string) (string, error) { return "", nil },
		listInvFn: func(ctx context.Context, electionID, adminUserID string) ([]elections.Invite, string, error) {
			return nil, "", nil
		},
	}

	h := NewHandlers(svc)
	ver := fakeVerifier{uid: "admin1", email: "a@example.com", role: "admin"}
	handler := middleware.RequireAuth(ver, middleware.RequireRole("admin", httputil.Wrap(h.CreateInvite)))

	req := httptest.NewRequest(http.MethodPost, "http://example/api/v1/elections/e1/invites", bytes.NewReader([]byte(`{"email":"x@example.com"}`)))
	req.SetPathValue("id", "e1")
	req.Header.Set("Authorization", "Bearer t")
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d body=%s", rr.Code, rr.Body.String())
	}
}

func TestListInvites_NotFound_MapsTo404(t *testing.T) {
	svc := fakeSvc{
		listInvFn: func(ctx context.Context, electionID, adminUserID string) ([]elections.Invite, string, error) {
			return nil, "not_found", nil
		},
		getFn: func(ctx context.Context, electionID, userID, email, role string) (elections.ElectionDetail, string, error) {
			return elections.ElectionDetail{}, "", nil
		},
		listFn: func(ctx context.Context, userID, email, role string) ([]elections.ElectionSummary, error) {
			return nil, nil
		},
		createFn: func(ctx context.Context, createdBy string, in elections.CreateElectionInput) (string, string, error) {
			return "", "", nil
		},
		ballotMetaFn: func(ctx context.Context, electionID, userID, email, role string) (elections.BallotMeta, string, error) {
			return elections.BallotMeta{}, "", nil
		},
		updateRulesFn: func(ctx context.Context, electionID, adminUserID string, in elections.UpdateRulesInput) (string, error) {
			return "", nil
		},
		actionFn: func(ctx context.Context, electionID, adminUserID, action string) (string, error) { return "", nil },
		createInvFn: func(ctx context.Context, electionID, adminUserID, email string) (elections.InviteCreated, string, error) {
			return elections.InviteCreated{}, "", nil
		},
	}

	h := NewHandlers(svc)
	ver := fakeVerifier{uid: "admin1", email: "a@example.com", role: "admin"}
	handler := middleware.RequireAuth(ver, middleware.RequireRole("admin", httputil.Wrap(h.ListInvites)))

	req := httptest.NewRequest(http.MethodGet, "http://example/api/v1/elections/e1/invites", nil)
	req.SetPathValue("id", "e1")
	req.Header.Set("Authorization", "Bearer t")

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d body=%s", rr.Code, rr.Body.String())
	}
}

func TestCreate_Success_ReturnsID(t *testing.T) {
	svc := newBaseFakeSvc()
	svc.createFn = func(ctx context.Context, createdBy string, in elections.CreateElectionInput) (string, string, error) {
		return "11111111-1111-1111-1111-111111111111", "", nil
	}

	h := NewHandlers(svc)
	ver := fakeVerifier{uid: "admin1", email: "a@example.com", role: "admin"}
	handler := middleware.RequireAuth(ver, middleware.RequireRole("admin", httputil.Wrap(h.Create)))

	req := httptest.NewRequest(
		http.MethodPost,
		"http://example/api/v1/elections",
		bytes.NewReader([]byte(`{
			"title":"test election",
			"start_at":"2026-03-20T10:00:00Z",
			"end_at":"2026-03-21T10:00:00Z",
			"tally_rule":"plurality",
			"ballot_format":"ranking",
			"access_mode":"open",
			"candidates":[{"name":"A"},{"name":"B"}]
		}`)),
	)
	req.Header.Set("Authorization", "Bearer t")
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rr.Code, rr.Body.String())
	}

	var resp struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v; body=%s", err, rr.Body.String())
	}
	if resp.ID != "11111111-1111-1111-1111-111111111111" {
		t.Fatalf("unexpected id: %q", resp.ID)
	}
}

func TestList_EmptySlice_ReturnsItemsArray(t *testing.T) {
	svc := newBaseFakeSvc()
	svc.listFn = func(ctx context.Context, userID, email, role string) ([]elections.ElectionSummary, error) {
		return []elections.ElectionSummary{}, nil
	}

	h := NewHandlers(svc)
	ver := fakeVerifier{uid: "u1", email: "u1@example.com", role: "voter"}
	handler := middleware.RequireAuth(ver, httputil.Wrap(h.List))

	req := httptest.NewRequest(http.MethodGet, "http://example/api/v1/elections", nil)
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
		t.Fatalf("expected empty items, got len=%d", len(resp.Items))
	}
}

func TestUpdateRules_Success_ReturnsOkTrue(t *testing.T) {
	svc := newBaseFakeSvc()
	svc.updateRulesFn = func(ctx context.Context, electionID, adminUserID string, in elections.UpdateRulesInput) (string, error) {
		return "", nil
	}

	h := NewHandlers(svc)
	ver := fakeVerifier{uid: "admin1", email: "a@example.com", role: "admin"}
	handler := middleware.RequireAuth(ver, middleware.RequireRole("admin", httputil.Wrap(h.UpdateRules)))

	req := httptest.NewRequest(http.MethodPut, "http://example/api/v1/elections/e1/rules", bytes.NewReader([]byte(`{
		"tally_rule":"plurality",
		"ballot_format":"ranking"
	}`)))
	req.SetPathValue("id", "e1")
	req.Header.Set("Authorization", "Bearer t")
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rr.Code, rr.Body.String())
	}

	var resp struct {
		Ok bool `json:"ok"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v; body=%s", err, rr.Body.String())
	}
	if !resp.Ok {
		t.Fatalf("expected ok=true")
	}
}

func TestAction_Success_ReturnsOkTrue(t *testing.T) {
	svc := newBaseFakeSvc()
	svc.actionFn = func(ctx context.Context, electionID, adminUserID, action string) (string, error) {
		return "", nil
	}

	h := NewHandlers(svc)
	ver := fakeVerifier{uid: "admin1", email: "a@example.com", role: "admin"}
	handler := middleware.RequireAuth(ver, middleware.RequireRole("admin", httputil.Wrap(h.Action)))

	req := httptest.NewRequest(http.MethodPost, "http://example/api/v1/elections/e1/actions/open", nil)
	req.SetPathValue("id", "e1")
	req.SetPathValue("action", "open")
	req.Header.Set("Authorization", "Bearer t")

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rr.Code, rr.Body.String())
	}

	var resp struct {
		Ok bool `json:"ok"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v; body=%s", err, rr.Body.String())
	}
	if !resp.Ok {
		t.Fatalf("expected ok=true")
	}
}

func TestCreateInvite_Success_ReturnsInvite(t *testing.T) {
	svc := newBaseFakeSvc()
	svc.createInvFn = func(ctx context.Context, electionID, adminUserID, email string) (elections.InviteCreated, string, error) {
		return elections.InviteCreated{
			InviteID:   "22222222-2222-2222-2222-222222222222",
			Email:      "x@example.com",
			InviteCode: "CODE123",
			Status:     "created",
			CreatedAt:  "2026-03-12T10:00:00Z",
		}, "", nil
	}

	h := NewHandlers(svc)
	ver := fakeVerifier{uid: "admin1", email: "a@example.com", role: "admin"}
	handler := middleware.RequireAuth(ver, middleware.RequireRole("admin", httputil.Wrap(h.CreateInvite)))

	req := httptest.NewRequest(
		http.MethodPost,
		"http://example/api/v1/elections/e1/invites",
		bytes.NewReader([]byte(`{"email":"x@example.com"}`)),
	)
	req.SetPathValue("id", "e1")
	req.Header.Set("Authorization", "Bearer t")
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rr.Code, rr.Body.String())
	}

	var resp struct {
		InviteID   string `json:"invite_id"`
		Email      string `json:"email"`
		InviteCode string `json:"invite_code"`
		Status     string `json:"status"`
		CreatedAt  string `json:"created_at"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v; body=%s", err, rr.Body.String())
	}

	if resp.InviteID != "22222222-2222-2222-2222-222222222222" {
		t.Fatalf("unexpected invite_id: %q", resp.InviteID)
	}
	if resp.Email != "x@example.com" {
		t.Fatalf("unexpected email: %q", resp.Email)
	}
	if resp.InviteCode != "CODE123" {
		t.Fatalf("unexpected invite_code: %q", resp.InviteCode)
	}
	if resp.Status != "created" {
		t.Fatalf("unexpected status: %q", resp.Status)
	}
	if resp.CreatedAt != "2026-03-12T10:00:00Z" {
		t.Fatalf("unexpected created_at: %q", resp.CreatedAt)
	}
}

func strPtr(v string) *string {
	return &v
}

func TestListInvites_Success_ReturnsItemsArray(t *testing.T) {
	svc := newBaseFakeSvc()
	svc.listInvFn = func(ctx context.Context, electionID, adminUserID string) ([]elections.Invite, string, error) {
		return []elections.Invite{
			{
				ID:        "33333333-3333-3333-3333-333333333333",
				Email:     "first@example.com",
				Status:    "created",
				CreatedAt: "2026-03-12T10:00:00Z",
			},
			{
				ID:         "44444444-4444-4444-4444-444444444444",
				Email:      "second@example.com",
				Status:     "accepted",
				CreatedAt:  "2026-03-12T11:00:00Z",
				AcceptedAt: strPtr("2026-03-12T11:30:00Z"),
			},
		}, "", nil
	}

	h := NewHandlers(svc)
	ver := fakeVerifier{uid: "admin1", email: "a@example.com", role: "admin"}
	handler := middleware.RequireAuth(ver, middleware.RequireRole("admin", httputil.Wrap(h.ListInvites)))

	req := httptest.NewRequest(http.MethodGet, "http://example/api/v1/elections/e1/invites", nil)
	req.SetPathValue("id", "e1")
	req.Header.Set("Authorization", "Bearer t")

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rr.Code, rr.Body.String())
	}

	var resp struct {
		Items []struct {
			ID         string `json:"id"`
			Email      string `json:"email"`
			Status     string `json:"status"`
			SentAt     string `json:"sent_at"`
			AcceptedAt string `json:"accepted_at"`
			CreatedAt  string `json:"created_at"`
		} `json:"items"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v; body=%s", err, rr.Body.String())
	}

	if resp.Items == nil {
		t.Fatalf("items is nil, want array")
	}
	if len(resp.Items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(resp.Items))
	}

	if resp.Items[0].ID != "33333333-3333-3333-3333-333333333333" {
		t.Fatalf("unexpected first id: %q", resp.Items[0].ID)
	}
	if resp.Items[0].Email != "first@example.com" {
		t.Fatalf("unexpected first email: %q", resp.Items[0].Email)
	}
	if resp.Items[0].Status != "created" {
		t.Fatalf("unexpected first status: %q", resp.Items[0].Status)
	}

	if resp.Items[1].ID != "44444444-4444-4444-4444-444444444444" {
		t.Fatalf("unexpected second id: %q", resp.Items[1].ID)
	}
	if resp.Items[1].Status != "accepted" {
		t.Fatalf("unexpected second status: %q", resp.Items[1].Status)
	}
	if resp.Items[1].AcceptedAt != "2026-03-12T11:30:00Z" {
		t.Fatalf("unexpected second accepted_at: %q", resp.Items[1].AcceptedAt)
	}
}

func TestListInvites_EmptySlice_ReturnsItemsArray(t *testing.T) {
	svc := newBaseFakeSvc()
	svc.listInvFn = func(ctx context.Context, electionID, adminUserID string) ([]elections.Invite, string, error) {
		return []elections.Invite{}, "", nil
	}

	h := NewHandlers(svc)
	ver := fakeVerifier{uid: "admin1", email: "a@example.com", role: "admin"}
	handler := middleware.RequireAuth(ver, middleware.RequireRole("admin", httputil.Wrap(h.ListInvites)))

	req := httptest.NewRequest(http.MethodGet, "http://example/api/v1/elections/e1/invites", nil)
	req.SetPathValue("id", "e1")
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
		t.Fatalf("expected empty items, got len=%d", len(resp.Items))
	}
}

func (s fakeSvc) ImportCandidates(ctx context.Context, filename string, content []byte) ([]elections.CandidateNormalized, string, error) {
	if s.importCandidatesFn != nil {
		return s.importCandidatesFn(ctx, filename, content)
	}
	return nil, "", nil
}

func (s fakeSvc) ImportInvites(ctx context.Context, electionID, adminUserID, filename string, content []byte) (elections.InviteImportResult, string, error) {
	return elections.InviteImportResult{}, "", nil
}
