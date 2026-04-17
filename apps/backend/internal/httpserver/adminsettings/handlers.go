package adminsettings

import (
	"context"
	"net/http"

	"secure-voting/apps/backend/internal/adminsettings"
	"secure-voting/apps/backend/internal/apperr"
	"secure-voting/apps/backend/internal/httpserver/httputil"
	"secure-voting/apps/backend/internal/httpserver/middleware"
)

type Service interface {
	Get(ctx context.Context) (adminsettings.Settings, error)
	Update(ctx context.Context, in adminsettings.UpdateInput) (adminsettings.Settings, string, error)
}

type Handlers struct {
	svc Service
}

func NewHandlers(svc Service) *Handlers {
	return &Handlers{svc: svc}
}

func (h *Handlers) Get(w http.ResponseWriter, r *http.Request) error {
	out, err := h.svc.Get(r.Context())
	if err != nil {
		return apperr.Internal(err, "load admin settings failed")
	}
	httputil.WriteJSON(w, http.StatusOK, out)
	return nil
}

type updateReq struct {
	PublicBaseURL       string `json:"public_base_url"`
	TLSMode             string `json:"tls_mode"`
	TLSDomain           string `json:"tls_domain"`
	TLSContactEmail     string `json:"tls_contact_email"`
	BackupEnabled       bool   `json:"backup_enabled"`
	BackupSchedule      string `json:"backup_schedule"`
	BackupRetentionDays *int   `json:"backup_retention_days"`
	DatabaseHost        string `json:"database_host"`
	DatabaseName        string `json:"database_name"`
}

func mapCode(code string) error {
	switch code {
	case "unauthorized":
		return apperr.Unauthorized("invalid or expired token")
	case "invalid_tls_mode":
		return apperr.Invalid(code, "invalid tls_mode")
	case "invalid_tls_contact_email":
		return apperr.Invalid(code, "invalid tls_contact_email")
	case "invalid_backup_retention_days":
		return apperr.Invalid(code, "invalid backup_retention_days")
	default:
		return apperr.Invalid(code, "invalid input")
	}
}

func (h *Handlers) Update(w http.ResponseWriter, r *http.Request) error {
	actorUserID, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		return apperr.Unauthorized("invalid or expired token")
	}

	var req updateReq
	if err := httputil.DecodeJSON(r, &req); err != nil {
		return apperr.Invalid("invalid_json", "invalid json body")
	}

	out, code, err := h.svc.Update(r.Context(), adminsettings.UpdateInput{
		ActorUserID:         actorUserID,
		PublicBaseURL:       req.PublicBaseURL,
		TLSMode:             req.TLSMode,
		TLSDomain:           req.TLSDomain,
		TLSContactEmail:     req.TLSContactEmail,
		BackupEnabled:       req.BackupEnabled,
		BackupSchedule:      req.BackupSchedule,
		BackupRetentionDays: req.BackupRetentionDays,
		DatabaseHost:        req.DatabaseHost,
		DatabaseName:        req.DatabaseName,
	})
	if err != nil {
		return apperr.Internal(err, "update admin settings failed")
	}
	if code != "" {
		return mapCode(code)
	}

	httputil.WriteJSON(w, http.StatusOK, out)
	return nil
}
