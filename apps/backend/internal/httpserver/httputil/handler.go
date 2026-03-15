package httputil

import (
	"errors"
	"log"
	"net/http"

	"secure-voting/apps/backend/internal/apperr"
)

type HandlerFunc func(http.ResponseWriter, *http.Request) error

func Wrap(h HandlerFunc) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := h(w, r); err != nil {
			// логируем только серверные (internal) ошибки по умолчанию
			if isInternal(err) {
				log.Printf("http %s %s error: %v", r.Method, r.URL.Path, err)
			}
			WriteErrorFromErr(w, err)
		}
	})
}

func WriteErrorFromErr(w http.ResponseWriter, err error) {
	status, code, msg := apperr.ToHTTP(err)
	WriteError(w, status, code, msg)
}

func isInternal(err error) bool {
	var ae *apperr.Error
	if errors.As(err, &ae) {
		return ae.Kind == apperr.KindInternal
	}
	return true
}
