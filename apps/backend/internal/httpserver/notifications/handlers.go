package notifications

import (
	"context"
	"net/http"
	"strconv"

	"secure-voting/apps/backend/internal/apperr"
	"secure-voting/apps/backend/internal/httpserver/httputil"
	"secure-voting/apps/backend/internal/httpserver/middleware"
	nsvc "secure-voting/apps/backend/internal/notifications"
)

type NotificationsService interface {
	List(ctx context.Context, userID string, limit, offset int) ([]nsvc.Item, string, error)
	Create(ctx context.Context, in nsvc.CreateInput) (nsvc.Item, string, error)
	MarkRead(ctx context.Context, userID, notificationID string) (string, error)
	MarkAllRead(ctx context.Context, userID string) (string, error)
	Delete(ctx context.Context, userID, notificationID string) (string, error)
	ClearAll(ctx context.Context, userID string) (string, error)
	SeedIfEmpty(ctx context.Context, userID string) error
}

type Handlers struct {
	svc NotificationsService
}

func NewHandlers(svc NotificationsService) *Handlers {
	return &Handlers{svc: svc}
}

func mapCode(code string) error {
	switch code {
	case "unauthorized":
		return apperr.Unauthorized("invalid or expired token")
	case "invalid_id":
		return apperr.Invalid(code, "invalid id")
	case "invalid_title":
		return apperr.Invalid(code, "invalid title")
	case "invalid_message":
		return apperr.Invalid(code, "invalid message")
	case "invalid_details":
		return apperr.Invalid(code, "invalid details")
	case "invalid_action_label":
		return apperr.Invalid(code, "invalid action_label")
	case "invalid_action_to":
		return apperr.Invalid(code, "invalid action_to")
	case "not_found":
		return apperr.NotFound("not found")
	default:
		return apperr.Invalid(code, "invalid input")
	}
}

func (h *Handlers) List(w http.ResponseWriter, r *http.Request) error {
	userID, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		return apperr.Unauthorized("invalid or expired token")
	}

	if err := h.svc.SeedIfEmpty(r.Context(), userID); err != nil {
		return apperr.Internal(err, "seed notifications failed")
	}

	limit := 100
	offset := 0

	if raw := r.URL.Query().Get("limit"); raw != "" {
		if v, err := strconv.Atoi(raw); err == nil {
			limit = v
		}
	}
	if raw := r.URL.Query().Get("offset"); raw != "" {
		if v, err := strconv.Atoi(raw); err == nil {
			offset = v
		}
	}

	items, code, err := h.svc.List(r.Context(), userID, limit, offset)
	if err != nil {
		return apperr.Internal(err, "list notifications failed")
	}
	if code != "" {
		return mapCode(code)
	}

	httputil.WriteJSON(w, http.StatusOK, map[string]any{"items": items})
	return nil
}

type createReq struct {
	Title       string `json:"title"`
	Message     string `json:"message"`
	Details     string `json:"details,omitempty"`
	ActionLabel string `json:"action_label,omitempty"`
	ActionTo    string `json:"action_to,omitempty"`
	Kind        string `json:"kind,omitempty"`
}

func (h *Handlers) Create(w http.ResponseWriter, r *http.Request) error {
	userID, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		return apperr.Unauthorized("invalid or expired token")
	}

	var req createReq
	if err := httputil.DecodeJSON(r, &req); err != nil {
		return apperr.Invalid("invalid_json", "invalid json body")
	}

	item, code, err := h.svc.Create(r.Context(), nsvc.CreateInput{
		UserID:      userID,
		Title:       req.Title,
		Message:     req.Message,
		Details:     req.Details,
		ActionLabel: req.ActionLabel,
		ActionTo:    req.ActionTo,
		Kind:        req.Kind,
	})
	if err != nil {
		return apperr.Internal(err, "create notification failed")
	}
	if code != "" {
		return mapCode(code)
	}

	httputil.WriteJSON(w, http.StatusOK, item)
	return nil
}

func (h *Handlers) MarkRead(w http.ResponseWriter, r *http.Request) error {
	userID, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		return apperr.Unauthorized("invalid or expired token")
	}

	id := r.PathValue("id")
	code, err := h.svc.MarkRead(r.Context(), userID, id)
	if err != nil {
		return apperr.Internal(err, "mark notification read failed")
	}
	if code != "" {
		return mapCode(code)
	}

	httputil.WriteJSON(w, http.StatusOK, map[string]any{"ok": true})
	return nil
}

func (h *Handlers) MarkAllRead(w http.ResponseWriter, r *http.Request) error {
	userID, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		return apperr.Unauthorized("invalid or expired token")
	}

	code, err := h.svc.MarkAllRead(r.Context(), userID)
	if err != nil {
		return apperr.Internal(err, "mark all notifications read failed")
	}
	if code != "" {
		return mapCode(code)
	}

	httputil.WriteJSON(w, http.StatusOK, map[string]any{"ok": true})
	return nil
}

func (h *Handlers) Delete(w http.ResponseWriter, r *http.Request) error {
	userID, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		return apperr.Unauthorized("invalid or expired token")
	}

	id := r.PathValue("id")
	code, err := h.svc.Delete(r.Context(), userID, id)
	if err != nil {
		return apperr.Internal(err, "delete notification failed")
	}
	if code != "" {
		return mapCode(code)
	}

	httputil.WriteJSON(w, http.StatusOK, map[string]any{"ok": true})
	return nil
}

func (h *Handlers) ClearAll(w http.ResponseWriter, r *http.Request) error {
	userID, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		return apperr.Unauthorized("invalid or expired token")
	}

	code, err := h.svc.ClearAll(r.Context(), userID)
	if err != nil {
		return apperr.Internal(err, "clear notifications failed")
	}
	if code != "" {
		return mapCode(code)
	}

	httputil.WriteJSON(w, http.StatusOK, map[string]any{"ok": true})
	return nil
}
