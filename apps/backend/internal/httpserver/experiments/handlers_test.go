package experiments

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	domain "secure-voting/apps/backend/internal/experiments"
)

func TestNewHandlers(t *testing.T) {
	h := NewHandlers(nil)
	if h == nil {
		t.Fatal("expected handlers")
	}
}

func TestCreate_BadJSON(t *testing.T) {
	h := NewHandlers(nil)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/experiments", strings.NewReader("{"))
	w := httptest.NewRecorder()

	h.Create(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d body=%s", w.Code, w.Body.String())
	}
}

func TestCreate_Success(t *testing.T) {
	orig := createExperimentFn
	defer func() { createExperimentFn = orig }()

	createExperimentFn = func(_ *domain.Service, _ context.Context, _ string, req domain.CreateReq) (string, string, error) {
		if req.Type != "algo" {
			t.Fatalf("unexpected req type: %q", req.Type)
		}
		return "exp-1", "", nil
	}

	h := NewHandlers(nil)

	body := bytes.NewBufferString(`{"type":"algo","params":{"ballot_format":"ranking","tally_rule":"plurality","committee_size":1}}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/experiments", body)
	w := httptest.NewRecorder()

	h.Create(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", w.Code, w.Body.String())
	}

	var out map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if out["id"] != "exp-1" {
		t.Fatalf("unexpected id: %#v", out["id"])
	}
}

func TestCreate_CodeError(t *testing.T) {
	orig := createExperimentFn
	defer func() { createExperimentFn = orig }()

	createExperimentFn = func(_ *domain.Service, _ context.Context, _ string, _ domain.CreateReq) (string, string, error) {
		return "", "invalid_type", nil
	}

	h := NewHandlers(nil)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/experiments", bytes.NewBufferString(`{"type":"bad"}`))
	w := httptest.NewRecorder()

	h.Create(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d body=%s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "invalid_type") {
		t.Fatalf("expected invalid_type in body, got %s", w.Body.String())
	}
}

func TestCreate_InternalError(t *testing.T) {
	orig := createExperimentFn
	defer func() { createExperimentFn = orig }()

	createExperimentFn = func(_ *domain.Service, _ context.Context, _ string, _ domain.CreateReq) (string, string, error) {
		return "", "", errors.New("boom")
	}

	h := NewHandlers(nil)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/experiments", bytes.NewBufferString(`{"type":"algo"}`))
	w := httptest.NewRecorder()

	h.Create(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d body=%s", w.Code, w.Body.String())
	}
}

func TestList_Success(t *testing.T) {
	orig := listExperimentsFn
	defer func() { listExperimentsFn = orig }()

	listExperimentsFn = func(_ *domain.Service, _ context.Context, _, _ string, p domain.ListParams) ([]domain.Experiment, error) {
		if p.Limit != 10 || p.Offset != 5 || p.Type != "algo" || p.Status != "draft" {
			t.Fatalf("unexpected params: %#v", p)
		}
		return []domain.Experiment{
			{ID: "exp-1", Type: "algo", Status: "draft"},
		}, nil
	}

	h := NewHandlers(nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/experiments?type=algo&status=draft&limit=10&offset=5", nil)
	w := httptest.NewRecorder()

	h.List(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "exp-1") {
		t.Fatalf("expected exp-1 in body, got %s", w.Body.String())
	}
}

func TestList_InternalError(t *testing.T) {
	orig := listExperimentsFn
	defer func() { listExperimentsFn = orig }()

	listExperimentsFn = func(_ *domain.Service, _ context.Context, _, _ string, _ domain.ListParams) ([]domain.Experiment, error) {
		return nil, errors.New("boom")
	}

	h := NewHandlers(nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/experiments", nil)
	w := httptest.NewRecorder()

	h.List(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d body=%s", w.Code, w.Body.String())
	}
}

func TestGet_NotFound(t *testing.T) {
	orig := getExperimentFn
	defer func() { getExperimentFn = orig }()

	getExperimentFn = func(_ *domain.Service, _ context.Context, _, _, _ string) (domain.Experiment, string, error) {
		return domain.Experiment{}, "not_found", nil
	}

	h := NewHandlers(nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/experiments/exp-1", nil)
	req.SetPathValue("id", "exp-1")
	w := httptest.NewRecorder()

	h.Get(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d body=%s", w.Code, w.Body.String())
	}
}

func TestGet_BadRequestCode(t *testing.T) {
	orig := getExperimentFn
	defer func() { getExperimentFn = orig }()

	getExperimentFn = func(_ *domain.Service, _ context.Context, _, _, _ string) (domain.Experiment, string, error) {
		return domain.Experiment{}, "invalid_id", nil
	}

	h := NewHandlers(nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/experiments/bad", nil)
	req.SetPathValue("id", "bad")
	w := httptest.NewRecorder()

	h.Get(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d body=%s", w.Code, w.Body.String())
	}
}

func TestGet_Success(t *testing.T) {
	orig := getExperimentFn
	defer func() { getExperimentFn = orig }()

	getExperimentFn = func(_ *domain.Service, _ context.Context, _, _, id string) (domain.Experiment, string, error) {
		return domain.Experiment{ID: id, Type: "algo", Status: "draft"}, "", nil
	}

	h := NewHandlers(nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/experiments/exp-1", nil)
	req.SetPathValue("id", "exp-1")
	w := httptest.NewRecorder()

	h.Get(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "exp-1") {
		t.Fatalf("expected exp-1 in body, got %s", w.Body.String())
	}
}
