package experimentruns

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	domain "secure-voting/apps/backend/internal/experimentruns"
)

func TestNewHandlers(t *testing.T) {
	h := NewHandlers(nil)
	if h == nil {
		t.Fatal("expected handlers")
	}
}

func TestWriteCodeError_NotFound(t *testing.T) {
	w := httptest.NewRecorder()
	writeCodeError(w, "not_found", "run not found")
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestWriteCodeError_BadRequest(t *testing.T) {
	w := httptest.NewRecorder()
	writeCodeError(w, "invalid_id", "run not found")
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestBatch_BadJSON(t *testing.T) {
	h := NewHandlers(nil)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/experiment-runs/batch", strings.NewReader("{"))
	w := httptest.NewRecorder()

	h.Batch(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d body=%s", w.Code, w.Body.String())
	}
}

func TestBatch_Success(t *testing.T) {
	orig := batchRunsFn
	defer func() { batchRunsFn = orig }()

	batchRunsFn = func(_ *domain.Service, _ context.Context, _, _ string, req domain.BatchReq) ([]domain.BatchItem, string, error) {
		if req.ExperimentID != "exp-1" || len(req.DatasetIDs) != 1 || req.DatasetIDs[0] != "ds-1" {
			t.Fatalf("unexpected req: %#v", req)
		}
		return []domain.BatchItem{{RunID: "run-1", JobID: "job-1"}}, "", nil
	}

	h := NewHandlers(nil)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/experiment-runs/batch", bytes.NewBufferString(`{"experiment_id":"exp-1","dataset_ids":["ds-1"]}`))
	w := httptest.NewRecorder()

	h.Batch(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "run-1") {
		t.Fatalf("expected run-1 in body, got %s", w.Body.String())
	}
}

func TestBatch_CodeError(t *testing.T) {
	orig := batchRunsFn
	defer func() { batchRunsFn = orig }()

	batchRunsFn = func(_ *domain.Service, _ context.Context, _, _ string, _ domain.BatchReq) ([]domain.BatchItem, string, error) {
		return nil, "invalid_experiment_id", nil
	}

	h := NewHandlers(nil)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/experiment-runs/batch", bytes.NewBufferString(`{"experiment_id":"bad","dataset_ids":["ds-1"]}`))
	w := httptest.NewRecorder()

	h.Batch(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d body=%s", w.Code, w.Body.String())
	}
}

func TestBatch_InternalError(t *testing.T) {
	orig := batchRunsFn
	defer func() { batchRunsFn = orig }()

	batchRunsFn = func(_ *domain.Service, _ context.Context, _, _ string, _ domain.BatchReq) ([]domain.BatchItem, string, error) {
		return nil, "", errors.New("boom")
	}

	h := NewHandlers(nil)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/experiment-runs/batch", bytes.NewBufferString(`{"experiment_id":"exp-1","dataset_ids":["ds-1"]}`))
	w := httptest.NewRecorder()

	h.Batch(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d body=%s", w.Code, w.Body.String())
	}
}

func TestList_Success(t *testing.T) {
	orig := listRunsFn
	defer func() { listRunsFn = orig }()

	listRunsFn = func(_ *domain.Service, _ context.Context, _, _, experimentID string) ([]domain.Run, string, error) {
		if experimentID != "exp-1" {
			t.Fatalf("unexpected experimentID: %q", experimentID)
		}
		return []domain.Run{{ID: "run-1", ExperimentID: "exp-1", Status: "done"}}, "", nil
	}

	h := NewHandlers(nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/experiment-runs?experiment_id=exp-1", nil)
	w := httptest.NewRecorder()

	h.List(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "run-1") {
		t.Fatalf("expected run-1 in body, got %s", w.Body.String())
	}
}

func TestList_CodeError(t *testing.T) {
	orig := listRunsFn
	defer func() { listRunsFn = orig }()

	listRunsFn = func(_ *domain.Service, _ context.Context, _, _, _ string) ([]domain.Run, string, error) {
		return nil, "not_found", nil
	}

	h := NewHandlers(nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/experiment-runs?experiment_id=exp-1", nil)
	w := httptest.NewRecorder()

	h.List(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d body=%s", w.Code, w.Body.String())
	}
}

func TestGet_Success(t *testing.T) {
	orig := getRunFn
	defer func() { getRunFn = orig }()

	getRunFn = func(_ *domain.Service, _ context.Context, _, _, id string) (domain.Run, string, error) {
		return domain.Run{ID: id, ExperimentID: "exp-1", Status: "done"}, "", nil
	}

	h := NewHandlers(nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/experiment-runs/run-1", nil)
	req.SetPathValue("id", "run-1")
	w := httptest.NewRecorder()

	h.Get(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", w.Code, w.Body.String())
	}
}

func TestGet_NotFound(t *testing.T) {
	orig := getRunFn
	defer func() { getRunFn = orig }()

	getRunFn = func(_ *domain.Service, _ context.Context, _, _, _ string) (domain.Run, string, error) {
		return domain.Run{}, "not_found", nil
	}

	h := NewHandlers(nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/experiment-runs/run-1", nil)
	req.SetPathValue("id", "run-1")
	w := httptest.NewRecorder()

	h.Get(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d body=%s", w.Code, w.Body.String())
	}
}

func TestResult_Success(t *testing.T) {
	orig := getRunResultFn
	defer func() { getRunResultFn = orig }()

	getRunResultFn = func(_ *domain.Service, _ context.Context, _, _, id string) (domain.Result, string, error) {
		return domain.Result{RunID: id, Winners: []any{"c1"}}, "", nil
	}

	h := NewHandlers(nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/experiment-runs/run-1/result", nil)
	req.SetPathValue("id", "run-1")
	w := httptest.NewRecorder()

	h.Result(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "c1") {
		t.Fatalf("expected c1 in body, got %s", w.Body.String())
	}
}

func TestDownload_Success(t *testing.T) {
	orig := downloadRunResultFn
	defer func() { downloadRunResultFn = orig }()

	downloadRunResultFn = func(_ *domain.Service, _ context.Context, _, _, _ string) ([]byte, string, string, string, error) {
		return []byte(`{"ok":true}`), "result.json", "application/json", "", nil
	}

	h := NewHandlers(nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/experiment-runs/run-1/download", nil)
	req.SetPathValue("id", "run-1")
	w := httptest.NewRecorder()

	h.Download(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", w.Code, w.Body.String())
	}
	if ct := w.Header().Get("Content-Type"); !strings.Contains(ct, "application/json") {
		t.Fatalf("unexpected content type: %q", ct)
	}
	if !strings.Contains(w.Body.String(), `"ok":true`) {
		t.Fatalf("unexpected body: %s", w.Body.String())
	}
}

func TestDownload_CodeError(t *testing.T) {
	orig := downloadRunResultFn
	defer func() { downloadRunResultFn = orig }()

	downloadRunResultFn = func(_ *domain.Service, _ context.Context, _, _, _ string) ([]byte, string, string, string, error) {
		return nil, "", "", "not_found", nil
	}

	h := NewHandlers(nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/experiment-runs/run-1/download", nil)
	req.SetPathValue("id", "run-1")
	w := httptest.NewRecorder()

	h.Download(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d body=%s", w.Code, w.Body.String())
	}
}

func TestBatch_ResponseJSONShape(t *testing.T) {
	orig := batchRunsFn
	defer func() { batchRunsFn = orig }()

	batchRunsFn = func(_ *domain.Service, _ context.Context, _, _ string, _ domain.BatchReq) ([]domain.BatchItem, string, error) {
		return []domain.BatchItem{{RunID: "run-1", JobID: "job-1"}}, "", nil
	}

	h := NewHandlers(nil)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/experiment-runs/batch", bytes.NewBufferString(`{"experiment_id":"exp-1","dataset_ids":["ds-1"]}`))
	w := httptest.NewRecorder()

	h.Batch(w, req)

	var out map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if _, ok := out["items"]; !ok {
		t.Fatalf("expected items in response, got %v", out)
	}
}
