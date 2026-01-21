package jobs

import (
	"log"
	"net/http"
	"strconv"
	"strings"

	"secure-voting/apps/backend/internal/httpserver/httputil"
	"secure-voting/apps/backend/internal/httpserver/middleware"
	"secure-voting/apps/backend/internal/jobs"
)

type Handlers struct {
	svc *jobs.Service
}

func NewHandlers(svc *jobs.Service) *Handlers {
	return &Handlers{svc: svc}
}

func (h *Handlers) List(w http.ResponseWriter, r *http.Request) {
	role, _ := middleware.RoleFromContext(r.Context())
	uid, _ := middleware.UserIDFromContext(r.Context())

	q := r.URL.Query()
	var f jobs.ListFilter

	if s := strings.TrimSpace(q.Get("status")); s != "" {
		f.Status = &s
	}
	if k := strings.TrimSpace(q.Get("kind")); k != "" {
		f.Kind = &k
	}
	if v := strings.TrimSpace(q.Get("limit")); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			f.Limit = n
		}
	}
	if v := strings.TrimSpace(q.Get("offset")); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			f.Offset = n
		}
	}

	items, err := h.svc.List(r.Context(), role, uid, f)
	if err != nil {
		log.Printf("jobs.list error: %v", err)
		httputil.WriteError(w, http.StatusInternalServerError, "internal_error", "list jobs failed")
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
		log.Printf("jobs.get error: %v", err)
		httputil.WriteError(w, http.StatusInternalServerError, "internal_error", "get job failed")
		return
	}
	if code != "" {
		switch code {
		case "not_found":
			httputil.WriteError(w, http.StatusNotFound, "not_found", "job not found")
		default:
			httputil.WriteError(w, http.StatusBadRequest, "bad_request", code)
		}
		return
	}

	httputil.WriteJSON(w, http.StatusOK, item)
}
