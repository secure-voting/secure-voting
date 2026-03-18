package datasets

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	cfgpkg "secure-voting/apps/backend/internal/config"
	domain "secure-voting/apps/backend/internal/datasets"
)

func restoreDatasetHandlerHooks() func() {
	oldList := listDatasetsFn
	oldGet := getDatasetFn
	oldDownload := downloadDatasetFn
	oldImport := importDatasetFn
	oldGenerate := generateDatasetFn

	return func() {
		listDatasetsFn = oldList
		getDatasetFn = oldGet
		downloadDatasetFn = oldDownload
		importDatasetFn = oldImport
		generateDatasetFn = oldGenerate
	}
}

func newTestHandlers() *Handlers {
	return NewHandlers(nil, cfgpkg.Config{MaxUploadBytes: 1 << 20})
}

func newMultipartRequest(t *testing.T, fields map[string]string, fileField, fileName string, fileContent []byte) *http.Request {
	t.Helper()

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)

	for k, v := range fields {
		if err := writer.WriteField(k, v); err != nil {
			t.Fatalf("write field %s: %v", k, err)
		}
	}

	if fileField != "" {
		part, err := writer.CreateFormFile(fileField, fileName)
		if err != nil {
			t.Fatalf("create form file: %v", err)
		}
		if _, err := part.Write(fileContent); err != nil {
			t.Fatalf("write file content: %v", err)
		}
	}

	if err := writer.Close(); err != nil {
		t.Fatalf("close multipart writer: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/datasets/import", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	return req
}

func TestList_Success(t *testing.T) {
	defer restoreDatasetHandlerHooks()()

	listDatasetsFn = func(_ *domain.Service, _ context.Context) ([]domain.ListItem, error) {
		return []domain.ListItem{{ID: "ds-1", Name: "dataset-1"}}, nil
	}

	h := newTestHandlers()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/datasets", nil)
	w := httptest.NewRecorder()

	h.List(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "dataset-1") {
		t.Fatalf("expected dataset name in body, got %s", w.Body.String())
	}
}

func TestList_InternalError(t *testing.T) {
	defer restoreDatasetHandlerHooks()()

	listDatasetsFn = func(_ *domain.Service, _ context.Context) ([]domain.ListItem, error) {
		return nil, errors.New("boom")
	}

	h := newTestHandlers()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/datasets", nil)
	w := httptest.NewRecorder()

	h.List(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d body=%s", w.Code, w.Body.String())
	}
}

func TestGet_Success(t *testing.T) {
	defer restoreDatasetHandlerHooks()()

	getDatasetFn = func(_ *domain.Service, _ context.Context, id string) (domain.Dataset, string, error) {
		return domain.Dataset{ID: id, Name: "dataset-1", Format: "ranking"}, "", nil
	}

	h := newTestHandlers()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/datasets/ds-1", nil)
	req.SetPathValue("id", "ds-1")
	w := httptest.NewRecorder()

	h.Get(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "dataset-1") {
		t.Fatalf("expected dataset in body, got %s", w.Body.String())
	}
}

func TestGet_NotFound(t *testing.T) {
	defer restoreDatasetHandlerHooks()()

	getDatasetFn = func(_ *domain.Service, _ context.Context, _ string) (domain.Dataset, string, error) {
		return domain.Dataset{}, "not_found", nil
	}

	h := newTestHandlers()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/datasets/ds-1", nil)
	req.SetPathValue("id", "ds-1")
	w := httptest.NewRecorder()

	h.Get(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d body=%s", w.Code, w.Body.String())
	}
}

func TestDownload_Success(t *testing.T) {
	defer restoreDatasetHandlerHooks()()

	downloadDatasetFn = func(_ *domain.Service, _ context.Context, _ string) ([]byte, string, string, string, error) {
		return []byte(`{"ok":true}`), "dataset.json", "application/json", "", nil
	}

	h := newTestHandlers()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/datasets/ds-1/download", nil)
	req.SetPathValue("id", "ds-1")
	w := httptest.NewRecorder()

	h.Download(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", w.Code, w.Body.String())
	}
	if ct := w.Header().Get("Content-Type"); !strings.Contains(ct, "application/json") {
		t.Fatalf("unexpected content type: %q", ct)
	}
}

func TestDownload_NotFound(t *testing.T) {
	defer restoreDatasetHandlerHooks()()

	downloadDatasetFn = func(_ *domain.Service, _ context.Context, _ string) ([]byte, string, string, string, error) {
		return nil, "", "", "not_found", nil
	}

	h := newTestHandlers()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/datasets/ds-1/download", nil)
	req.SetPathValue("id", "ds-1")
	w := httptest.NewRecorder()

	h.Download(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d body=%s", w.Code, w.Body.String())
	}
}

func TestGenerate_BadJSON(t *testing.T) {
	h := newTestHandlers()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/datasets/generate", strings.NewReader("{"))
	w := httptest.NewRecorder()

	h.Generate(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d body=%s", w.Code, w.Body.String())
	}
}

func TestGenerate_Success(t *testing.T) {
	defer restoreDatasetHandlerHooks()()

	generateDatasetFn = func(_ *domain.Service, _ context.Context, req domain.GenerateReq) (string, string, error) {
		if req.Name != "ds-1" || req.Format != "ranking" || req.Voters != 10 {
			t.Fatalf("unexpected req: %#v", req)
		}
		return "generated-id", "", nil
	}

	h := newTestHandlers()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/datasets/generate", strings.NewReader(`{
		"name":"ds-1",
		"format":"ranking",
		"candidates":[{"id":"c1","name":"Alice"}],
		"voters":10
	}`))
	w := httptest.NewRecorder()

	h.Generate(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", w.Code, w.Body.String())
	}

	var out map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if out["id"] != "generated-id" {
		t.Fatalf("unexpected id: %#v", out["id"])
	}
}

func TestGenerate_CodeError(t *testing.T) {
	defer restoreDatasetHandlerHooks()()

	generateDatasetFn = func(_ *domain.Service, _ context.Context, _ domain.GenerateReq) (string, string, error) {
		return "", "invalid_format", nil
	}

	h := newTestHandlers()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/datasets/generate", strings.NewReader(`{
		"name":"ds-1",
		"format":"bad",
		"candidates":[{"id":"c1","name":"Alice"}],
		"voters":10
	}`))
	w := httptest.NewRecorder()

	h.Generate(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d body=%s", w.Code, w.Body.String())
	}
}

func TestImport_MissingFile(t *testing.T) {
	h := newTestHandlers()
	req := newMultipartRequest(t, map[string]string{
		"name":   "ds-1",
		"format": "ranking",
	}, "", "", nil)
	w := httptest.NewRecorder()

	h.Import(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d body=%s", w.Code, w.Body.String())
	}
}

func TestImport_Success(t *testing.T) {
	defer restoreDatasetHandlerHooks()()

	importDatasetFn = func(_ *domain.Service, _ context.Context, meta domain.ImportMeta, fh *multipart.FileHeader, f multipart.File) (string, string, error) {
		if meta.Name != "ds-1" || meta.Description != "desc" || meta.Format != "ranking" {
			t.Fatalf("unexpected meta: %#v", meta)
		}
		if fh == nil || fh.Filename != "dataset.json" {
			t.Fatalf("unexpected file header: %#v", fh)
		}
		b, err := io.ReadAll(f)
		if err != nil {
			t.Fatalf("read multipart file: %v", err)
		}
		if !strings.Contains(string(b), `"dataset"`) {
			t.Fatalf("unexpected file content: %s", string(b))
		}
		return "imported-id", "", nil
	}

	h := newTestHandlers()
	req := newMultipartRequest(t, map[string]string{
		"name":        "ds-1",
		"description": "desc",
		"format":      "ranking",
	}, "file", "dataset.json", []byte(`{"dataset":{"format":"ranking"}}`))
	w := httptest.NewRecorder()

	h.Import(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "imported-id") {
		t.Fatalf("expected imported-id in body, got %s", w.Body.String())
	}
}

func TestGetFileHeader_Fallback(t *testing.T) {
	req := newMultipartRequest(t, map[string]string{}, "file", "dataset.json", []byte(`abc`))
	if err := req.ParseMultipartForm(1 << 20); err != nil {
		t.Fatalf("parse multipart: %v", err)
	}

	fh, err := getFileHeader(req, "file")
	if err != nil {
		t.Fatalf("getFileHeader: %v", err)
	}
	if fh == nil || fh.Filename != "dataset.json" {
		t.Fatalf("unexpected file header: %#v", fh)
	}
}
