package adminusers

import (
	"context"
	"net/http"
	"strconv"

	"secure-voting/apps/backend/internal/apperr"
	asvc "secure-voting/apps/backend/internal/auth"
	"secure-voting/apps/backend/internal/httpserver/httputil"
	"secure-voting/apps/backend/internal/httpserver/middleware"
)

type AdminUsersService interface {
	ListUsers(ctx context.Context, actorUserID string, limit, offset int) ([]asvc.AdminUser, string, error)
	UpdateUserRole(ctx context.Context, actorUserID, targetUserID, newRole string) (asvc.AdminUser, string, error)
}

type Handlers struct {
	svc AdminUsersService
}

func NewHandlers(svc AdminUsersService) *Handlers {
	return &Handlers{svc: svc}
}

func mapCode(code string) error {
	switch code {
	case "unauthorized":
		return apperr.Unauthorized("invalid or expired token")
	case "forbidden":
		return apperr.Forbidden("forbidden")
	case "invalid_id":
		return apperr.Invalid(code, "invalid id")
	case "invalid_role":
		return apperr.Invalid(code, "invalid role")
	case "self_role_change_forbidden":
		return apperr.Invalid(code, "cannot change own role")
	case "not_found":
		return apperr.NotFound("not found")
	default:
		return apperr.Invalid(code, "invalid input")
	}
}

func (h *Handlers) List(w http.ResponseWriter, r *http.Request) error {
	actorUserID, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		return apperr.Unauthorized("invalid or expired token")
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

	items, code, err := h.svc.ListUsers(r.Context(), actorUserID, limit, offset)
	if err != nil {
		return apperr.Internal(err, "list users failed")
	}
	if code != "" {
		return mapCode(code)
	}

	httputil.WriteJSON(w, http.StatusOK, map[string]any{"items": items})
	return nil
}

type updateRoleReq struct {
	Role string `json:"role"`
}

func (h *Handlers) UpdateRole(w http.ResponseWriter, r *http.Request) error {
	actorUserID, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		return apperr.Unauthorized("invalid or expired token")
	}

	targetUserID := r.PathValue("id")

	var req updateRoleReq
	if err := httputil.DecodeJSON(r, &req); err != nil {
		return apperr.Invalid("invalid_json", "invalid json body")
	}

	item, code, err := h.svc.UpdateUserRole(r.Context(), actorUserID, targetUserID, req.Role)
	if err != nil {
		return apperr.Internal(err, "update user role failed")
	}
	if code != "" {
		return mapCode(code)
	}

	httputil.WriteJSON(w, http.StatusOK, item)
	return nil
}
