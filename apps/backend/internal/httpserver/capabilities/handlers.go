package capabilities

import (
	"context"
	"errors"
	"net/http"

	"secure-voting/apps/backend/internal/apperr"
	capsvc "secure-voting/apps/backend/internal/capabilities"
	"secure-voting/apps/backend/internal/computeclient"
	"secure-voting/apps/backend/internal/httpserver/httputil"
)

type listTallyRulesFunc func(ctx context.Context) ([]computeclient.TallyRuleInfo, error)

type Handlers struct {
	listTallyRules listTallyRulesFunc
}

func NewHandlers(svc *capsvc.Service) *Handlers {
	h := &Handlers{}
	if svc != nil {
		h.listTallyRules = svc.ListTallyRules
	}
	return h
}

func (h *Handlers) ListTallyRules(w http.ResponseWriter, r *http.Request) error {
	if h.listTallyRules == nil {
		return apperr.Internal(errors.New("capabilities service is nil"), "list tally rules failed")
	}

	items, err := h.listTallyRules(r.Context())
	if err != nil {
		return apperr.Internal(err, "list tally rules failed")
	}

	httputil.WriteJSON(w, http.StatusOK, map[string]any{
		"items": items,
	})
	return nil
}