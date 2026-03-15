package apperr

import (
	"errors"
	"fmt"
	"net/http"
)

type Kind uint8

const (
	KindInvalid Kind = iota + 1
	KindUnauthorized
	KindForbidden
	KindNotFound
	KindConflict
	KindInternal
)

type Error struct {
	Kind    Kind
	Code    string
	Message string
	Err     error
}

func (e *Error) Error() string {
	if e == nil {
		return "<nil>"
	}
	msg := e.Message
	if msg == "" {
		msg = "error"
	}
	if e.Code == "" {
		return msg
	}
	return fmt.Sprintf("%s: %s", e.Code, msg)
}

func (e *Error) Unwrap() error { return e.Err }

func Invalid(code, message string) error {
	return &Error{Kind: KindInvalid, Code: code, Message: message}
}

func Unauthorized(message string) error {
	return &Error{Kind: KindUnauthorized, Code: "unauthorized", Message: message}
}

func Forbidden(message string) error {
	return &Error{Kind: KindForbidden, Code: "forbidden", Message: message}
}

func NotFound(message string) error {
	return &Error{Kind: KindNotFound, Code: "not_found", Message: message}
}

func Conflict(code, message string) error {
	return &Error{Kind: KindConflict, Code: code, Message: message}
}

func Internal(err error, message string) error {
	return &Error{Kind: KindInternal, Code: "internal_error", Message: message, Err: err}
}

// ToHTTP maps internal errors to (status, api_code, message) for your existing JSON schema:
// {"error":{"code":"...","message":"..."}}
func ToHTTP(err error) (int, string, string) {
	if err == nil {
		return http.StatusOK, "", ""
	}

	var ae *Error
	if errors.As(err, &ae) {
		switch ae.Kind {
		case KindInvalid:
			msg := ae.Message
			if msg == "" {
				msg = "invalid input"
			}
			return http.StatusBadRequest, "bad_request", msg
		case KindUnauthorized:
			msg := ae.Message
			if msg == "" {
				msg = "invalid or expired token"
			}
			return http.StatusUnauthorized, "unauthorized", msg
		case KindForbidden:
			msg := ae.Message
			if msg == "" {
				msg = "insufficient permissions"
			}
			return http.StatusForbidden, "forbidden", msg
		case KindNotFound:
			msg := ae.Message
			if msg == "" {
				msg = "not found"
			}
			return http.StatusNotFound, "not_found", msg
		case KindConflict:
			msg := ae.Message
			if msg == "" {
				msg = "conflict"
			}
			return http.StatusConflict, "conflict", msg
		case KindInternal:
			msg := ae.Message
			if msg == "" {
				msg = "internal error"
			}
			return http.StatusInternalServerError, "internal_error", msg
		default:
			return http.StatusInternalServerError, "internal_error", "internal error"
		}
	}

	return http.StatusInternalServerError, "internal_error", "internal error"
}
