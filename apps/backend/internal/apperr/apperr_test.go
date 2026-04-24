package apperr

import (
	"errors"
	"net/http"
	"testing"
)

func TestErrorString(t *testing.T) {
	var e *Error
	if e.Error() != "<nil>" {
		t.Fatalf("unexpected nil error string: %q", e.Error())
	}

	if (&Error{Message: "hello"}).Error() != "hello" {
		t.Fatal("unexpected message-only string")
	}

	if (&Error{Code: "bad", Message: "hello"}).Error() != "bad: hello" {
		t.Fatal("unexpected code+message string")
	}

	if (&Error{}).Error() != "error" {
		t.Fatal("unexpected empty string form")
	}
}

func TestUnwrap(t *testing.T) {
	base := errors.New("boom")
	e := &Error{Err: base}
	if !errors.Is(e, base) {
		t.Fatal("expected wrapped error")
	}
}

func TestConstructors(t *testing.T) {
	tests := []struct {
		name string
		err  error
		kind Kind
		code string
	}{
		{"invalid", Invalid("bad_input", "x"), KindInvalid, "bad_input"},
		{"unauthorized", Unauthorized("x"), KindUnauthorized, "unauthorized"},
		{"forbidden", Forbidden("x"), KindForbidden, "forbidden"},
		{"not_found", NotFound("x"), KindNotFound, "not_found"},
		{"conflict", Conflict("dup", "x"), KindConflict, "dup"},
		{"internal", Internal(errors.New("boom"), "x"), KindInternal, "internal_error"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var ae *Error
			if !errors.As(tt.err, &ae) {
				t.Fatalf("expected *Error for %s", tt.name)
			}
			if ae.Kind != tt.kind || ae.Code != tt.code {
				t.Fatalf("unexpected error: %#v", ae)
			}
		})
	}
}

func TestToHTTP(t *testing.T) {
	if status, code, msg := ToHTTP(nil); status != http.StatusOK || code != "" || msg != "" {
		t.Fatalf("unexpected nil mapping: %d %q %q", status, code, msg)
	}

	tests := []struct {
		name       string
		err        error
		wantStatus int
		wantCode   string
		wantMsg    string
	}{
		{"invalid custom", Invalid("x", "bad input"), http.StatusBadRequest, "bad_request", "bad input"},
		{"invalid default", &Error{Kind: KindInvalid}, http.StatusBadRequest, "bad_request", "invalid input"},
		{"unauthorized custom", Unauthorized("bad token"), http.StatusUnauthorized, "unauthorized", "bad token"},
		{"unauthorized default", &Error{Kind: KindUnauthorized}, http.StatusUnauthorized, "unauthorized", "invalid or expired token"},
		{"forbidden custom", Forbidden("no access"), http.StatusForbidden, "forbidden", "no access"},
		{"forbidden default", &Error{Kind: KindForbidden}, http.StatusForbidden, "forbidden", "insufficient permissions"},
		{"not_found custom", NotFound("gone"), http.StatusNotFound, "not_found", "gone"},
		{"not_found default", &Error{Kind: KindNotFound}, http.StatusNotFound, "not_found", "not found"},
		{"conflict custom", Conflict("dup", "duplicate"), http.StatusConflict, "conflict", "duplicate"},
		{"conflict default", &Error{Kind: KindConflict}, http.StatusConflict, "conflict", "conflict"},
		{"internal custom", Internal(errors.New("boom"), "oops"), http.StatusInternalServerError, "internal_error", "oops"},
		{"internal default", &Error{Kind: KindInternal}, http.StatusInternalServerError, "internal_error", "internal error"},
		{"unknown kind", &Error{Kind: 255}, http.StatusInternalServerError, "internal_error", "internal error"},
		{"plain error", errors.New("plain"), http.StatusInternalServerError, "internal_error", "internal error"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status, code, msg := ToHTTP(tt.err)
			if status != tt.wantStatus || code != tt.wantCode || msg != tt.wantMsg {
				t.Fatalf("got (%d,%q,%q), want (%d,%q,%q)", status, code, msg, tt.wantStatus, tt.wantCode, tt.wantMsg)
			}
		})
	}
}
