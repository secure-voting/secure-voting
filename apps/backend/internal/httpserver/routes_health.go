package httpserver

import (
	"net/http"
	"strings"
	"time"

	"secure-voting/apps/backend/internal/httpserver/httputil"
)

type systemComponentStatus struct {
	OK      bool           `json:"ok"`
	Status  string         `json:"status"`
	Details map[string]any `json:"details,omitempty"`
}

type systemStatusResponse struct {
	Backend   systemComponentStatus `json:"backend"`
	Compute   systemComponentStatus `json:"compute"`
	CheckedAt string                `json:"checked_at"`
}

func registerHealth(r *routeCtx) {
	r.mux.HandleFunc("GET /health", func(w http.ResponseWriter, _ *http.Request) {
		httputil.WriteJSON(w, http.StatusOK, map[string]any{
			"ok": true,
		})
	})

	r.mux.Handle("GET /api/v1/system/status", r.RequireRole("admin", func(w http.ResponseWriter, _ *http.Request) error {
		computeState := "unavailable"
		computeOK := false

		if r.computeClient != nil {
			computeState = strings.ToLower(r.computeClient.ConnectivityState())
			computeOK = r.computeClient.Ready()
		}

		httputil.WriteJSON(w, http.StatusOK, systemStatusResponse{
			Backend: systemComponentStatus{
				OK:     true,
				Status: "ready",
				Details: map[string]any{
					"http_addr": r.cfg.HTTPAddr,
				},
			},
			Compute: systemComponentStatus{
				OK:     computeOK,
				Status: computeState,
				Details: map[string]any{
					"addr": r.cfg.ComputeGRPCAddr,
					"tls":  r.cfg.ComputeTLS,
				},
			},
			CheckedAt: time.Now().UTC().Format(time.RFC3339),
		})

		return nil
	}))
}