package elections

import (
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
	createFn      func(ctx context.Context, createdBy string, in elections.CreateElectionInput) (string, string, error)
	listFn        func(ctx context.Context, userID, email, role string) ([]elections.ElectionSummary, error)
	getFn         func(ctx context.Context, electionID, userID, email, role string) (elections.ElectionDetail, string, error)
	ballotMetaFn  func(ctx context.Context, electionID, userID, email, role string) (elections.BallotMeta, string, error)
	updateRulesFn func(ctx context.Context, electionID, adminUserID string, in elections.UpdateRulesInput) (string, error)
	actionFn      func(ctx context.Context, electionID, adminUserID, action string) (string, error)
	createInvFn   func(ctx context.Context, electionID, adminUserID, email string) (elections.InviteCreated, string, error)
	listInvFn     func(ctx context.Context, electionID, adminUserID string) ([]elections.Invite, string, error)
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
	handler := middleware.RequireAuth(ver, http.HandlerFunc(h.Get))

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
	handler := middleware.RequireAuth(ver, http.HandlerFunc(h.Get))

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
	handler := middleware.RequireAuth(ver, middleware.RequireRole("admin", http.HandlerFunc(h.UpdateRules)))

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
	handler := middleware.RequireAuth(ver, middleware.RequireRole("admin", http.HandlerFunc(h.Action)))

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
	handler := middleware.RequireAuth(ver, middleware.RequireRole("admin", http.HandlerFunc(h.CreateInvite)))

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
	handler := middleware.RequireAuth(ver, middleware.RequireRole("admin", http.HandlerFunc(h.ListInvites)))

	req := httptest.NewRequest(http.MethodGet, "http://example/api/v1/elections/e1/invites", nil)
	req.SetPathValue("id", "e1")
	req.Header.Set("Authorization", "Bearer t")

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d body=%s", rr.Code, rr.Body.String())
	}
}
