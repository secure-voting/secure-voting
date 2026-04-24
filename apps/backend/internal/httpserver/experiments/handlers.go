package experiments

import (
	"context"
	"log"
	"net/http"
	"strconv"
	"strings"

	"secure-voting/apps/backend/internal/experiments"
	"secure-voting/apps/backend/internal/httpserver/httputil"
	"secure-voting/apps/backend/internal/httpserver/middleware"
)

var createExperimentFn = func(svc *experiments.Service, ctx context.Context, uid string, req experiments.CreateReq) (string, string, error) {
	return svc.Create(ctx, uid, req)
}

var listExperimentsFn = func(svc *experiments.Service, ctx context.Context, role, uid string, p experiments.ListParams) ([]experiments.Experiment, error) {
	return svc.List(ctx, role, uid, p)
}

var getExperimentFn = func(svc *experiments.Service, ctx context.Context, role, uid, id string) (experiments.Experiment, string, error) {
	return svc.Get(ctx, role, uid, id)
}

type Handlers struct {
	svc *experiments.Service
}

func NewHandlers(svc *experiments.Service) *Handlers {
	return &Handlers{svc: svc}
}

func (h *Handlers) Create(w http.ResponseWriter, r *http.Request) {
	uid, _ := middleware.UserIDFromContext(r.Context())

	var req experiments.CreateReq
	if err := httputil.DecodeJSON(r, &req); err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "bad_request", "invalid json body")
		return
	}

	id, code, err := createExperimentFn(h.svc, r.Context(), uid, req)
	if err != nil {
		log.Printf("experiments.create error: %v", err)
		httputil.WriteError(w, http.StatusInternalServerError, "internal_error", "create experiment failed")
		return
	}
	if code != "" {
		httputil.WriteError(w, http.StatusBadRequest, "bad_request", code)
		return
	}

	httputil.WriteJSON(w, http.StatusOK, map[string]any{"id": id})
}

func (h *Handlers) List(w http.ResponseWriter, r *http.Request) {
	role, _ := middleware.RoleFromContext(r.Context())
	uid, _ := middleware.UserIDFromContext(r.Context())

	q := r.URL.Query()
	limit, _ := strconv.Atoi(q.Get("limit"))
	offset, _ := strconv.Atoi(q.Get("offset"))

	items, err := listExperimentsFn(h.svc, r.Context(), role, uid, experiments.ListParams{
		Type:   strings.TrimSpace(q.Get("type")),
		Status: strings.TrimSpace(q.Get("status")),
		Limit:  limit,
		Offset: offset,
	})
	if err != nil {
		log.Printf("experiments.list error: %v", err)
		httputil.WriteError(w, http.StatusInternalServerError, "internal_error", "list experiments failed")
		return
	}

	httputil.WriteJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *Handlers) Get(w http.ResponseWriter, r *http.Request) {
	role, _ := middleware.RoleFromContext(r.Context())
	uid, _ := middleware.UserIDFromContext(r.Context())

	id := strings.TrimSpace(r.PathValue("id"))
	e, code, err := getExperimentFn(h.svc, r.Context(), role, uid, id)
	if err != nil {
		log.Printf("experiments.get error: %v", err)
		httputil.WriteError(w, http.StatusInternalServerError, "internal_error", "get experiment failed")
		return
	}
	if code != "" {
		if code == "not_found" {
			httputil.WriteError(w, http.StatusNotFound, "not_found", "experiment not found")
		} else {
			httputil.WriteError(w, http.StatusBadRequest, "bad_request", code)
		}
		return
	}

	httputil.WriteJSON(w, http.StatusOK, e)
}
