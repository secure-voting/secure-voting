package experimentruns

import (
	"context"
	"log"
	"net/http"
	"strings"

	"secure-voting/apps/backend/internal/experimentruns"
	"secure-voting/apps/backend/internal/httpserver/httputil"
	"secure-voting/apps/backend/internal/httpserver/middleware"
)

var batchRunsFn = func(svc *experimentruns.Service, ctx context.Context, uid, role string, req experimentruns.BatchReq) ([]experimentruns.BatchItem, string, error) {
	return svc.BatchCreate(ctx, uid, role, req)
}

var listRunsFn = func(svc *experimentruns.Service, ctx context.Context, role, uid, experimentID string) ([]experimentruns.Run, string, error) {
	return svc.List(ctx, role, uid, experimentID)
}

var getRunFn = func(svc *experimentruns.Service, ctx context.Context, role, uid, id string) (experimentruns.Run, string, error) {
	return svc.Get(ctx, role, uid, id)
}

var getRunResultFn = func(svc *experimentruns.Service, ctx context.Context, role, uid, id string) (experimentruns.Result, string, error) {
	return svc.GetResult(ctx, role, uid, id)
}

var downloadRunResultFn = func(svc *experimentruns.Service, ctx context.Context, role, uid, id string) ([]byte, string, string, string, error) {
	return svc.DownloadResult(ctx, role, uid, id)
}

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

	items, code, err := batchRunsFn(h.svc, r.Context(), uid, role, req)
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

	items, code, err := listRunsFn(h.svc, r.Context(), role, uid, experimentID)
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

	item, code, err := getRunFn(h.svc, r.Context(), role, uid, id)
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

	res, code, err := getRunResultFn(h.svc, r.Context(), role, uid, id)
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

	data, filename, mime, code, err := downloadRunResultFn(h.svc, r.Context(), role, uid, id)
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
