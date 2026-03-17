package experimentruns

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestWriteCodeError_InvalidID(t *testing.T) {
	rr := httptest.NewRecorder()

	writeCodeError(rr, "invalid_id", "result not found")

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestWriteCodeError_InvalidExperimentID(t *testing.T) {
	rr := httptest.NewRecorder()

	writeCodeError(rr, "invalid_experiment_id", "experiment not found")

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestWriteCodeError_NotFound(t *testing.T) {
	rr := httptest.NewRecorder()

	writeCodeError(rr, "not_found", "result not found")

	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected %d, got %d", http.StatusNotFound, rr.Code)
	}
}

func TestWriteCodeError_UnknownCodeFallsBackToBadRequest(t *testing.T) {
	rr := httptest.NewRecorder()

	writeCodeError(rr, "some_other_code", "result not found")

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected %d, got %d", http.StatusBadRequest, rr.Code)
	}
}
