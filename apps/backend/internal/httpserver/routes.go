package httpserver

import (
	"net/http"
)

// Routes wires HTTP handlers.
// Keep handlers small; put business logic into internal packages later.
func Routes() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		// Health endpoint must be fast and side-effect free.
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	})

	return mux
}
