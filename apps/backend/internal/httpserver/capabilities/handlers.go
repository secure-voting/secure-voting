package capabilities

import (
	"net/http"

	"secure-voting/apps/backend/internal/apperr"
	capsvc "secure-voting/apps/backend/internal/capabilities"
	"secure-voting/apps/backend/internal/httpserver/httputil"
)

type Handlers struct {
	svc *capsvc.Service
}

func NewHandlers(svc *capsvc.Service) *Handlers {
	return &Handlers{svc: svc}
}

func (h *Handlers) ListTallyRules(w http.ResponseWriter, r *http.Request) error {
	items, err := h.svc.ListTallyRules(r.Context())
	if err != nil {
		return apperr.Internal(err, "list tally rules failed")
	}

	httputil.WriteJSON(w, http.StatusOK, map[string]any{
		"items": items,
	})
	return nil
}