package ballots

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"secure-voting/apps/backend/internal/ballots"
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
	submitFn func(ctx context.Context, electionID, userID, email, idemKey string, req ballots.SubmitReq) (ballots.SubmitResp, string, error)
	myFn     func(ctx context.Context, electionID, userID, email string) (ballots.MyBallotResp, string, error)
}

func (f fakeSvc) Submit(ctx context.Context, electionID, userID, email, idemKey string, req ballots.SubmitReq) (ballots.SubmitResp, string, error) {
	return f.submitFn(ctx, electionID, userID, email, idemKey, req)
}

func (f fakeSvc) MyBallot(ctx context.Context, electionID, userID, email string) (ballots.MyBallotResp, string, error) {
	return f.myFn(ctx, electionID, userID, email)
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

func TestSubmit_Success(t *testing.T) {
	svc := fakeSvc{
		submitFn: func(ctx context.Context, electionID, userID, email, idemKey string, req ballots.SubmitReq) (ballots.SubmitResp, string, error) {
			if electionID != "e1" || userID != "u1" || email != "voter1@example.com" {
				t.Fatalf("unexpected args: electionID=%s userID=%s email=%s", electionID, userID, email)
			}
			if idemKey != "k1" {
				t.Fatalf("expected idemKey=k1, got %s", idemKey)
			}
			return ballots.SubmitResp{Ok: true, BallotID: "b1", Status: "accepted"}, "", nil
		},
		myFn: func(ctx context.Context, electionID, userID, email string) (ballots.MyBallotResp, string, error) {
			return ballots.MyBallotResp{}, "", nil
		},
	}

	h := NewHandlers(svc)
	ver := fakeVerifier{uid: "u1", email: "voter1@example.com", role: "voter"}

	handler := middleware.RequireAuth(ver, middleware.RequireRole("voter", http.HandlerFunc(h.Submit)))

	body := []byte(`{"approval_set":["c1"]}`)
	req := httptest.NewRequest(http.MethodPost, "http://example/api/v1/elections/e1/ballots/submit", bytes.NewReader(body))
	req.SetPathValue("id", "e1")
	req.Header.Set("Authorization", "Bearer testtoken")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Idempotency-Key", "k1")

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rr.Code, rr.Body.String())
	}
	var got ballots.SubmitResp
	if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode ok resp: %v", err)
	}
	if !got.Ok || got.BallotID != "b1" || got.Status != "accepted" {
		t.Fatalf("unexpected response: %+v", got)
	}
}

func TestSubmit_MissingIdempotencyKey_MapsTo400(t *testing.T) {
	svc := fakeSvc{
		submitFn: func(ctx context.Context, electionID, userID, email, idemKey string, req ballots.SubmitReq) (ballots.SubmitResp, string, error) {
			if idemKey != "" {
				t.Fatalf("expected empty idemKey, got %s", idemKey)
			}
			return ballots.SubmitResp{}, "missing_idempotency_key", nil
		},
		myFn: func(ctx context.Context, electionID, userID, email string) (ballots.MyBallotResp, string, error) {
			return ballots.MyBallotResp{}, "", nil
		},
	}

	h := NewHandlers(svc)
	ver := fakeVerifier{uid: "u1", email: "voter1@example.com", role: "voter"}
	handler := middleware.RequireAuth(ver, middleware.RequireRole("voter", http.HandlerFunc(h.Submit)))

	req := httptest.NewRequest(http.MethodPost, "http://example/api/v1/elections/e1/ballots/submit", bytes.NewReader([]byte(`{"approval_set":["c1"]}`)))
	req.SetPathValue("id", "e1")
	req.Header.Set("Authorization", "Bearer testtoken")
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d body=%s", rr.Code, rr.Body.String())
	}
	er := decodeErr(t, rr)
	if er.Error.Code != "bad_request" {
		t.Fatalf("expected bad_request, got %s", er.Error.Code)
	}
}

func TestSubmit_NotFound_MapsTo404(t *testing.T) {
	svc := fakeSvc{
		submitFn: func(ctx context.Context, electionID, userID, email, idemKey string, req ballots.SubmitReq) (ballots.SubmitResp, string, error) {
			return ballots.SubmitResp{}, "not_found", nil
		},
		myFn: func(ctx context.Context, electionID, userID, email string) (ballots.MyBallotResp, string, error) {
			return ballots.MyBallotResp{}, "", nil
		},
	}

	h := NewHandlers(svc)
	ver := fakeVerifier{uid: "u1", email: "voter1@example.com", role: "voter"}
	handler := middleware.RequireAuth(ver, middleware.RequireRole("voter", http.HandlerFunc(h.Submit)))

	req := httptest.NewRequest(http.MethodPost, "http://example/api/v1/elections/e1/ballots/submit", bytes.NewReader([]byte(`{"approval_set":["c1"]}`)))
	req.SetPathValue("id", "e1")
	req.Header.Set("Authorization", "Bearer testtoken")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Idempotency-Key", "k1")

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d body=%s", rr.Code, rr.Body.String())
	}
}

func TestMe_Success(t *testing.T) {
	svc := fakeSvc{
		submitFn: func(ctx context.Context, electionID, userID, email, idemKey string, req ballots.SubmitReq) (ballots.SubmitResp, string, error) {
			return ballots.SubmitResp{}, "", nil
		},
		myFn: func(ctx context.Context, electionID, userID, email string) (ballots.MyBallotResp, string, error) {
			if electionID != "e1" || userID != "u1" || email != "voter1@example.com" {
				t.Fatalf("unexpected args: %s %s %s", electionID, userID, email)
			}
			return ballots.MyBallotResp{Status: "none"}, "", nil
		},
	}

	h := NewHandlers(svc)
	ver := fakeVerifier{uid: "u1", email: "voter1@example.com", role: "voter"}
	handler := middleware.RequireAuth(ver, middleware.RequireRole("voter", http.HandlerFunc(h.Me)))

	req := httptest.NewRequest(http.MethodGet, "http://example/api/v1/elections/e1/ballots/me", nil)
	req.SetPathValue("id", "e1")
	req.Header.Set("Authorization", "Bearer testtoken")

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rr.Code, rr.Body.String())
	}
	var got ballots.MyBallotResp
	if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode resp: %v", err)
	}
	if got.Status != "none" {
		t.Fatalf("unexpected resp: %+v", got)
	}
}
