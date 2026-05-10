package notifications

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"secure-voting/apps/backend/internal/apperr"
	"secure-voting/apps/backend/internal/httpserver/httputil"
	"secure-voting/apps/backend/internal/httpserver/middleware"
	nsvc "secure-voting/apps/backend/internal/notifications"
)

type fakeNotificationsService struct {
	listFn        func(ctx context.Context, userID string, limit, offset int) ([]nsvc.Item, string, error)
	createFn      func(ctx context.Context, in nsvc.CreateInput) (nsvc.Item, string, error)
	markReadFn    func(ctx context.Context, userID, notificationID string) (string, error)
	markAllReadFn func(ctx context.Context, userID string) (string, error)
	deleteFn      func(ctx context.Context, userID, notificationID string) (string, error)
	clearAllFn    func(ctx context.Context, userID string) (string, error)
	seedIfEmptyFn func(ctx context.Context, userID string) error
}

func (f *fakeNotificationsService) List(ctx context.Context, userID string, limit, offset int) ([]nsvc.Item, string, error) {
	if f.listFn != nil {
		return f.listFn(ctx, userID, limit, offset)
	}
	return nil, "", nil
}

func (f *fakeNotificationsService) Create(ctx context.Context, in nsvc.CreateInput) (nsvc.Item, string, error) {
	if f.createFn != nil {
		return f.createFn(ctx, in)
	}
	return nsvc.Item{}, "", nil
}

func (f *fakeNotificationsService) MarkRead(ctx context.Context, userID, notificationID string) (string, error) {
	if f.markReadFn != nil {
		return f.markReadFn(ctx, userID, notificationID)
	}
	return "", nil
}

func (f *fakeNotificationsService) MarkAllRead(ctx context.Context, userID string) (string, error) {
	if f.markAllReadFn != nil {
		return f.markAllReadFn(ctx, userID)
	}
	return "", nil
}

func (f *fakeNotificationsService) Delete(ctx context.Context, userID, notificationID string) (string, error) {
	if f.deleteFn != nil {
		return f.deleteFn(ctx, userID, notificationID)
	}
	return "", nil
}

func (f *fakeNotificationsService) ClearAll(ctx context.Context, userID string) (string, error) {
	if f.clearAllFn != nil {
		return f.clearAllFn(ctx, userID)
	}
	return "", nil
}

func (f *fakeNotificationsService) SeedIfEmpty(ctx context.Context, userID string) error {
	if f.seedIfEmptyFn != nil {
		return f.seedIfEmptyFn(ctx, userID)
	}
	return nil
}

type fakeVerifier struct{}

func (fakeVerifier) VerifyAccessToken(ctx context.Context, rawToken string) (string, string, string, bool, error) {
	if rawToken == "ok-token" {
		return "u1", "u1@example.com", "voter", true, nil
	}
	return "", "", "", false, nil
}

func authedRequest(method, url string, body string) *http.Request {
	req := httptest.NewRequest(method, url, strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer ok-token")
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	return req
}

func decodeMap(t *testing.T, rr *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var out map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode body: %v body=%s", err, rr.Body.String())
	}
	return out
}

func TestMapCode(t *testing.T) {
	cases := []struct {
		code string
		want string
	}{
		{"unauthorized", "unauthorized"},
		{"invalid_id", "bad_request"},
		{"invalid_title", "bad_request"},
		{"invalid_message", "bad_request"},
		{"invalid_details", "bad_request"},
		{"invalid_action_label", "bad_request"},
		{"invalid_action_to", "bad_request"},
		{"not_found", "not_found"},
		{"other", "bad_request"},
	}

	for _, tc := range cases {
		t.Run(tc.code, func(t *testing.T) {
			err := mapCode(tc.code)
			status, code, _ := writeErrCompat(err)
			_ = status
			if code != tc.want {
				t.Fatalf("expected code=%q got=%q", tc.want, code)
			}
		})
	}
}

func TestList_Unauthorized(t *testing.T) {
	h := NewHandlers(&fakeNotificationsService{})
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/notifications", nil)

	handler := middleware.RequireAuth(fakeVerifier{}, httputil.Wrap(h.List))
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 got %d body=%s", rr.Code, rr.Body.String())
	}
}

func TestList_OK(t *testing.T) {
	h := NewHandlers(&fakeNotificationsService{
		seedIfEmptyFn: func(ctx context.Context, userID string) error {
			if userID != "u1" {
				t.Fatalf("unexpected userID: %q", userID)
			}
			return nil
		},
		listFn: func(ctx context.Context, userID string, limit, offset int) ([]nsvc.Item, string, error) {
			if userID != "u1" {
				t.Fatalf("unexpected userID: %q", userID)
			}
			if limit != 25 || offset != 5 {
				t.Fatalf("unexpected paging: limit=%d offset=%d", limit, offset)
			}
			return []nsvc.Item{{
				ID:        "n1",
				Title:     "title",
				Message:   "message",
				Kind:      "info",
				CreatedAt: "2026-01-01T00:00:00Z",
			}}, "", nil
		},
	})

	rr := httptest.NewRecorder()
	req := authedRequest(http.MethodGet, "/api/v1/notifications?limit=25&offset=5", "")
	handler := middleware.RequireAuth(fakeVerifier{}, httputil.Wrap(h.List))
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 got %d body=%s", rr.Code, rr.Body.String())
	}
	body := decodeMap(t, rr)
	items, ok := body["items"].([]any)
	if !ok || len(items) != 1 {
		t.Fatalf("unexpected items: %#v", body["items"])
	}
}

func TestList_SeedError(t *testing.T) {
	h := NewHandlers(&fakeNotificationsService{
		seedIfEmptyFn: func(ctx context.Context, userID string) error {
			return errors.New("boom")
		},
	})

	rr := httptest.NewRecorder()
	req := authedRequest(http.MethodGet, "/api/v1/notifications", "")
	handler := middleware.RequireAuth(fakeVerifier{}, httputil.Wrap(h.List))
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 got %d body=%s", rr.Code, rr.Body.String())
	}
}

func TestCreate_InvalidJSON(t *testing.T) {
	h := NewHandlers(&fakeNotificationsService{})
	rr := httptest.NewRecorder()
	req := authedRequest(http.MethodPost, "/api/v1/notifications", `{bad json`)
	handler := middleware.RequireAuth(fakeVerifier{}, httputil.Wrap(h.Create))
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 got %d body=%s", rr.Code, rr.Body.String())
	}
}

func TestCreate_OK(t *testing.T) {
	h := NewHandlers(&fakeNotificationsService{
		createFn: func(ctx context.Context, in nsvc.CreateInput) (nsvc.Item, string, error) {
			if in.UserID != "u1" || in.Title != "Title" || in.Message != "Body" || in.Kind != "warning" {
				t.Fatalf("unexpected input: %#v", in)
			}
			return nsvc.Item{
				ID:        "n1",
				Title:     in.Title,
				Message:   in.Message,
				Kind:      in.Kind,
				CreatedAt: "2026-01-01T00:00:00Z",
			}, "", nil
		},
	})

	rr := httptest.NewRecorder()
	req := authedRequest(http.MethodPost, "/api/v1/notifications", `{"title":"Title","message":"Body","kind":"warning"}`)
	handler := middleware.RequireAuth(fakeVerifier{}, httputil.Wrap(h.Create))
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 got %d body=%s", rr.Code, rr.Body.String())
	}
	body := decodeMap(t, rr)
	if body["id"] != "n1" {
		t.Fatalf("unexpected body: %#v", body)
	}
}

func TestMarkRead_OK(t *testing.T) {
	h := NewHandlers(&fakeNotificationsService{
		markReadFn: func(ctx context.Context, userID, notificationID string) (string, error) {
			if userID != "u1" || notificationID != "n1" {
				t.Fatalf("unexpected args: userID=%q notificationID=%q", userID, notificationID)
			}
			return "", nil
		},
	})

	rr := httptest.NewRecorder()
	req := authedRequest(http.MethodPost, "/api/v1/notifications/n1/read", "")
	req.SetPathValue("id", "n1")
	handler := middleware.RequireAuth(fakeVerifier{}, httputil.Wrap(h.MarkRead))
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 got %d body=%s", rr.Code, rr.Body.String())
	}
}

func TestMarkAllRead_OK(t *testing.T) {
	h := NewHandlers(&fakeNotificationsService{
		markAllReadFn: func(ctx context.Context, userID string) (string, error) {
			if userID != "u1" {
				t.Fatalf("unexpected userID: %q", userID)
			}
			return "", nil
		},
	})

	rr := httptest.NewRecorder()
	req := authedRequest(http.MethodPost, "/api/v1/notifications/read-all", "")
	handler := middleware.RequireAuth(fakeVerifier{}, httputil.Wrap(h.MarkAllRead))
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 got %d body=%s", rr.Code, rr.Body.String())
	}
}

func TestDelete_OK(t *testing.T) {
	h := NewHandlers(&fakeNotificationsService{
		deleteFn: func(ctx context.Context, userID, notificationID string) (string, error) {
			if userID != "u1" || notificationID != "n1" {
				t.Fatalf("unexpected args: userID=%q notificationID=%q", userID, notificationID)
			}
			return "", nil
		},
	})

	rr := httptest.NewRecorder()
	req := authedRequest(http.MethodDelete, "/api/v1/notifications/n1", "")
	req.SetPathValue("id", "n1")
	handler := middleware.RequireAuth(fakeVerifier{}, httputil.Wrap(h.Delete))
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 got %d body=%s", rr.Code, rr.Body.String())
	}
}

func TestClearAll_OK(t *testing.T) {
	h := NewHandlers(&fakeNotificationsService{
		clearAllFn: func(ctx context.Context, userID string) (string, error) {
			if userID != "u1" {
				t.Fatalf("unexpected userID: %q", userID)
			}
			return "", nil
		},
	})

	rr := httptest.NewRecorder()
	req := authedRequest(http.MethodDelete, "/api/v1/notifications", "")
	handler := middleware.RequireAuth(fakeVerifier{}, httputil.Wrap(h.ClearAll))
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 got %d body=%s", rr.Code, rr.Body.String())
	}
}

func writeErrCompat(err error) (int, string, string) {
	return apperr.ToHTTP(err)
}
