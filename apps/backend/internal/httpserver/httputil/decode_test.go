package httputil

import (
	"errors"
	"net/http/httptest"
	"strings"
	"testing"
)

type decodeReq struct {
	Name string `json:"name"`
}

func TestDecodeJSON_Success(t *testing.T) {
	req := httptest.NewRequest("POST", "/", strings.NewReader(`{"name":"alice"}`))

	var dst decodeReq
	if err := DecodeJSON(req, &dst); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dst.Name != "alice" {
		t.Fatalf("unexpected name: %q", dst.Name)
	}
}

func TestDecodeJSON_UnknownField(t *testing.T) {
	req := httptest.NewRequest("POST", "/", strings.NewReader(`{"name":"alice","extra":1}`))

	var dst decodeReq
	if err := DecodeJSON(req, &dst); err == nil {
		t.Fatal("expected error for unknown field")
	}
}

func TestDecodeJSON_ExtraJSON(t *testing.T) {
	req := httptest.NewRequest("POST", "/", strings.NewReader(`{"name":"alice"} {"name":"bob"}`))

	var dst decodeReq
	err := DecodeJSON(req, &dst)
	if !errors.Is(err, errExtraJSON) {
		t.Fatalf("expected errExtraJSON, got %v", err)
	}
}

func TestDecodeJSON_InvalidJSON(t *testing.T) {
	req := httptest.NewRequest("POST", "/", strings.NewReader(`{`))

	var dst decodeReq
	if err := DecodeJSON(req, &dst); err == nil {
		t.Fatal("expected invalid json error")
	}
}
