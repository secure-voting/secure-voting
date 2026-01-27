package results

import (
	"log"
	"net/http"
	"strings"

	"secure-voting/apps/backend/internal/httpserver/httputil"
	"secure-voting/apps/backend/internal/httpserver/middleware"
	"secure-voting/apps/backend/internal/results"
)

type Handlers struct {
	svc *results.Service
}

func NewHandlers(svc *results.Service) *Handlers {
	return &Handlers{svc: svc}
}

func (h *Handlers) Get(w http.ResponseWriter, r *http.Request) {
	eid := strings.TrimSpace(r.PathValue("id"))

	role, _ := middleware.RoleFromContext(r.Context())
	uid, _ := middleware.UserIDFromContext(r.Context())
	email, _ := middleware.EmailFromContext(r.Context())

	res, code, err := h.svc.Get(r.Context(), eid, role, uid, email)
	if err != nil {
		log.Printf("results.get error: %v", err)
		httputil.WriteError(w, http.StatusInternalServerError, "internal_error", "get results failed")
		return
	}
	if code != "" {
		switch code {
		case "invalid_id":
			httputil.WriteError(w, http.StatusBadRequest, "bad_request", "invalid id")
		case "not_found":
			httputil.WriteError(w, http.StatusNotFound, "not_found", "election not found")
		case "not_published":
			httputil.WriteError(w, http.StatusForbidden, "forbidden", "results not published")
		case "no_results":
			httputil.WriteError(w, http.StatusNotFound, "not_found", "no results yet")
		default:
			httputil.WriteError(w, http.StatusBadRequest, "bad_request", code)
		}
		return
	}

	httputil.WriteJSON(w, http.StatusOK, res)
}
