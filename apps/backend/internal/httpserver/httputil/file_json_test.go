package httputil

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestWriteFile_DefaultContentType(t *testing.T) {
	w := httptest.NewRecorder()

	WriteFile(w, "report.txt", "", []byte("hello"))

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/octet-stream" {
		t.Fatalf("unexpected content type: %q", ct)
	}
	if cd := w.Header().Get("Content-Disposition"); !strings.Contains(cd, `filename="report.txt"`) {
		t.Fatalf("unexpected content disposition: %q", cd)
	}
	if body := w.Body.String(); body != "hello" {
		t.Fatalf("unexpected body: %q", body)
	}
}

func TestWriteJSON(t *testing.T) {
	w := httptest.NewRecorder()

	WriteJSON(w, http.StatusCreated, map[string]any{"ok": true})

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); !strings.Contains(ct, "application/json") {
		t.Fatalf("unexpected content type: %q", ct)
	}
	if !strings.Contains(w.Body.String(), `"ok":true`) {
		t.Fatalf("unexpected body: %s", w.Body.String())
	}
}

func TestWriteError(t *testing.T) {
	w := httptest.NewRecorder()

	WriteError(w, http.StatusBadRequest, "bad_request", "broken")

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), `"code":"bad_request"`) {
		t.Fatalf("unexpected body: %s", w.Body.String())
	}
	if !strings.Contains(w.Body.String(), `"message":"broken"`) {
		t.Fatalf("unexpected body: %s", w.Body.String())
	}
}
