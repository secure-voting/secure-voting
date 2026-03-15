package results

import (
	"context"
	"net/http"
	"strings"

	"secure-voting/apps/backend/internal/httpserver/httputil"
	"secure-voting/apps/backend/internal/httpserver/middleware"
	"secure-voting/apps/backend/internal/results"
)

type getFunc func(ctx context.Context, electionID, userID, email, role string) (any, string, error)

type Handlers struct {
	get getFunc
}

func NewHandlers(svc *results.Service) *Handlers {
	return &Handlers{
		get: func(ctx context.Context, electionID, userID, email, role string) (any, string, error) {
			res, code, err := svc.Get(ctx, electionID, userID, email, role)
			return res, code, err
		},
	}
}

func (h *Handlers) Get(w http.ResponseWriter, r *http.Request) error {
	eid := strings.TrimSpace(r.PathValue("id"))

	role, _ := middleware.RoleFromContext(r.Context())
	uid, _ := middleware.UserIDFromContext(r.Context())
	email, _ := middleware.EmailFromContext(r.Context())

	res, code, err := h.get(r.Context(), eid, uid, email, role)
	if err != nil {
		return err
	}

	if code != "" {
		switch code {
		case "invalid_id":
			httputil.WriteError(w, http.StatusBadRequest, "invalid_id", "invalid election id")
			return nil
		case "not_found":
			httputil.WriteError(w, http.StatusNotFound, "not_found", "election not found")
			return nil
		case "not_published":
			httputil.WriteError(w, http.StatusForbidden, "not_published", "results not published")
			return nil
		default:
			httputil.WriteError(w, http.StatusBadRequest, code, code)
			return nil
		}
	}

	httputil.WriteJSON(w, http.StatusOK, res)
	return nil
}