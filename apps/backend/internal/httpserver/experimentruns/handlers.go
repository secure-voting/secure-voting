package experimentruns

import (
	"log"
	"net/http"
	"strings"

	"secure-voting/apps/backend/internal/experimentruns"
	"secure-voting/apps/backend/internal/httpserver/httputil"
	"secure-voting/apps/backend/internal/httpserver/middleware"
)

type Handlers struct {
	svc *experimentruns.Service
}

func NewHandlers(svc *experimentruns.Service) *Handlers {
	return &Handlers{svc: svc}
}

func writeCodeError(w http.ResponseWriter, code, notFoundMessage string) {
	switch code {
	case "not_found":
		httputil.WriteError(w, http.StatusNotFound, "not_found", notFoundMessage)
	case "invalid_id", "invalid_experiment_id":
		httputil.WriteError(w, http.StatusBadRequest, "bad_request", code)
	default:
		httputil.WriteError(w, http.StatusBadRequest, "bad_request", code)
	}
}

func (h *Handlers) Batch(w http.ResponseWriter, r *http.Request) {
	role, _ := middleware.RoleFromContext(r.Context())
	uid, _ := middleware.UserIDFromContext(r.Context())

	var req experimentruns.BatchReq
	if err := httputil.DecodeJSON(r, &req); err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "bad_request", "invalid json body")
		return
	}

	items, code, err := h.svc.BatchCreate(r.Context(), uid, role, req)
	if err != nil {
		log.Printf("experimentruns.batch error: %v", err)
		httputil.WriteError(w, http.StatusInternalServerError, "internal_error", "batch create failed")
		return
	}
	if code != "" {
		writeCodeError(w, code, "experiment not found")
		return
	}

	httputil.WriteJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *Handlers) List(w http.ResponseWriter, r *http.Request) {
	role, _ := middleware.RoleFromContext(r.Context())
	uid, _ := middleware.UserIDFromContext(r.Context())
	experimentID := strings.TrimSpace(r.URL.Query().Get("experiment_id"))

	items, code, err := h.svc.List(r.Context(), role, uid, experimentID)
	if err != nil {
		log.Printf("experimentruns.list error: %v", err)
		httputil.WriteError(w, http.StatusInternalServerError, "internal_error", "list runs failed")
		return
	}
	if code != "" {
		writeCodeError(w, code, "experiment not found")
		return
	}

	httputil.WriteJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *Handlers) Get(w http.ResponseWriter, r *http.Request) {
	role, _ := middleware.RoleFromContext(r.Context())
	uid, _ := middleware.UserIDFromContext(r.Context())
	id := strings.TrimSpace(r.PathValue("id"))

	item, code, err := h.svc.Get(r.Context(), role, uid, id)
	if err != nil {
		log.Printf("experimentruns.get error: %v", err)
		httputil.WriteError(w, http.StatusInternalServerError, "internal_error", "get run failed")
		return
	}
	if code != "" {
		writeCodeError(w, code, "run not found")
		return
	}

	httputil.WriteJSON(w, http.StatusOK, item)
}

func (h *Handlers) Result(w http.ResponseWriter, r *http.Request) {
	role, _ := middleware.RoleFromContext(r.Context())
	uid, _ := middleware.UserIDFromContext(r.Context())
	id := strings.TrimSpace(r.PathValue("id"))

	res, code, err := h.svc.GetResult(r.Context(), role, uid, id)
	if err != nil {
		log.Printf("experimentruns.result error: %v", err)
		httputil.WriteError(w, http.StatusInternalServerError, "internal_error", "get result failed")
		return
	}
	if code != "" {
		writeCodeError(w, code, "result not found")
		return
	}

	httputil.WriteJSON(w, http.StatusOK, res)
}

func (h *Handlers) Download(w http.ResponseWriter, r *http.Request) {
	role, _ := middleware.RoleFromContext(r.Context())
	uid, _ := middleware.UserIDFromContext(r.Context())
	id := strings.TrimSpace(r.PathValue("id"))

	data, filename, mime, code, err := h.svc.DownloadResult(r.Context(), role, uid, id)
	if err != nil {
		log.Printf("experimentruns.download error: %v", err)
		httputil.WriteError(w, http.StatusInternalServerError, "internal_error", "download failed")
		return
	}
	if code != "" {
		writeCodeError(w, code, "result not found")
		return
	}

	httputil.WriteFile(w, filename, mime, data)
}
