package audit

import (
	"context"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	coreaudit "secure-voting/apps/backend/internal/audit"
	"secure-voting/apps/backend/internal/httpserver/httputil"
	"secure-voting/apps/backend/internal/httpserver/middleware"
)

type listFunc func(ctx context.Context, role, uid string, f coreaudit.ListFilter) (any, error)
type parseTimeFunc func(value string) (*time.Time, error)

type Handlers struct {
	list      listFunc
	parseTime parseTimeFunc
}

func NewHandlers(svc *coreaudit.Service) *Handlers {
	return &Handlers{
		list: func(ctx context.Context, role, uid string, f coreaudit.ListFilter) (any, error) {
			items, err := svc.List(ctx, role, uid, f)
			return items, err
		},
		parseTime: svc.ParseTimeRFC3339,
	}
}

func (h *Handlers) List(w http.ResponseWriter, r *http.Request) {
	role, _ := middleware.RoleFromContext(r.Context())
	uid, _ := middleware.UserIDFromContext(r.Context())

	q := r.URL.Query()
	var f coreaudit.ListFilter

	if et := strings.TrimSpace(q.Get("event_type")); et != "" {
		f.EventType = &et
	}
	if au := strings.TrimSpace(q.Get("actor_user_id")); au != "" {
		if _, ok := coreaudit.ParseUUIDOrEmpty(au); !ok {
			httputil.WriteError(w, http.StatusBadRequest, "bad_request", "invalid actor_user_id")
			return
		}
		f.ActorUserID = &au
	}

	if v := strings.TrimSpace(q.Get("since")); v != "" {
		t, err := h.parseTime(v)
		if err != nil {
			httputil.WriteError(w, http.StatusBadRequest, "bad_request", "invalid since")
			return
		}
		f.Since = t
	}
	if v := strings.TrimSpace(q.Get("until")); v != "" {
		t, err := h.parseTime(v)
		if err != nil {
			httputil.WriteError(w, http.StatusBadRequest, "bad_request", "invalid until")
			return
		}
		f.Until = t
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

	items, err := h.list(r.Context(), role, uid, f)
	if err != nil {
		log.Printf("audit.list error: %v", err)
		httputil.WriteError(w, http.StatusInternalServerError, "internal_error", "list audit log failed")
		return
	}
	httputil.WriteJSON(w, http.StatusOK, map[string]any{"items": items})
}
