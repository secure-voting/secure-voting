package datasets

import (
	"bytes"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"secure-voting/apps/backend/internal/config"
)

func TestImport_PayloadTooLarge_Returns413(t *testing.T) {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)

	part, err := writer.CreateFormFile("file", "big.json")
	if err != nil {
		t.Fatalf("create form file: %v", err)
	}

	largePayload := strings.Repeat("a", 128)
	if _, err := part.Write([]byte(largePayload)); err != nil {
		t.Fatalf("write form file: %v", err)
	}

	if err := writer.Close(); err != nil {
		t.Fatalf("close multipart writer: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/datasets/import", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	rr := httptest.NewRecorder()

	h := NewHandlers(nil, config.Config{
		MaxUploadBytes: 32,
	})

	h.Import(rr, req)

	if rr.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("expected %d, got %d; body=%s", http.StatusRequestEntityTooLarge, rr.Code, rr.Body.String())
	}
}