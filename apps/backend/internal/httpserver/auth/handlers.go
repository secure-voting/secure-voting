package auth

import (
	"context"
	"net/http"

	"secure-voting/apps/backend/internal/apperr"
	asvc "secure-voting/apps/backend/internal/auth"
	"secure-voting/apps/backend/internal/httpserver/httputil"
	"secure-voting/apps/backend/internal/httpserver/middleware"
)

type AuthService interface {
	Register(ctx context.Context, email, password, role, inviteCode string) (asvc.AuthResult, string, error)
	Login(ctx context.Context, email, password, inviteCode string) (asvc.AuthResult, string, error)
	Logout(ctx context.Context, rawToken string, actorUserID *string) (bool, error)
	ChangePassword(ctx context.Context, userID, currentPassword, newPassword string) (string, error)
	GetProfile(ctx context.Context, userID string) (asvc.User, string, error)
	UpdateProfile(ctx context.Context, userID, fullName, phone string) (asvc.User, string, error)
}

type Handlers struct {
	svc AuthService
}

func NewHandlers(svc AuthService) *Handlers {
	return &Handlers{svc: svc}
}

type registerReq struct {
	Email      string `json:"email"`
	Password   string `json:"password"`
	InviteCode string `json:"invite_code,omitempty"`
}

func mapRegisterCode(code string) error {
	switch code {
	case "invalid_email":
		return apperr.Invalid(code, "invalid email")
	case "invalid_password":
		return apperr.Invalid(code, "password must be at least 8 characters")
	case "invalid_role":
		return apperr.Invalid(code, "invalid role")
	case "email_taken":
		return apperr.Conflict(code, "email already registered")
	case "invalid_invite_code":
		return apperr.Invalid(code, "invalid invite_code")
	case "invite_code_inactive":
		return apperr.Invalid(code, "invite_code is not active")
	case "invite_email_mismatch":
		return apperr.Invalid(code, "invite_code does not match email")
	default:
		return apperr.Invalid(code, "invalid input")
	}
}

func (h *Handlers) Register(w http.ResponseWriter, r *http.Request) error {
	var req registerReq
	if err := httputil.DecodeJSON(r, &req); err != nil {
		return apperr.Invalid("invalid_json", "invalid json body")
	}

	res, code, err := h.svc.Register(r.Context(), req.Email, req.Password, "", req.InviteCode)
	if err != nil {
		return apperr.Internal(err, "register failed")
	}
	if code != "" {
		return mapRegisterCode(code)
	}

	httputil.WriteJSON(w, http.StatusOK, res)
	return nil
}

type loginReq struct {
	Email      string `json:"email"`
	Password   string `json:"password"`
	InviteCode string `json:"invite_code,omitempty"`
}

func mapLoginCode(code string) error {
	switch code {
	case "invalid_credentials":
		return apperr.Unauthorized("invalid credentials")
	case "invalid_email":
		return apperr.Invalid(code, "invalid email")
	case "invalid_password":
		return apperr.Invalid(code, "invalid password")
	case "invalid_invite_code":
		return apperr.Invalid(code, "invalid invite_code")
	case "invite_code_inactive":
		return apperr.Invalid(code, "invite_code is not active")
	case "invite_email_mismatch":
		return apperr.Invalid(code, "invite_code does not match email")
	default:
		return apperr.Invalid(code, "invalid input")
	}
}

func (h *Handlers) Login(w http.ResponseWriter, r *http.Request) error {
	var req loginReq
	if err := httputil.DecodeJSON(r, &req); err != nil {
		return apperr.Invalid("invalid_json", "invalid json body")
	}

	res, code, err := h.svc.Login(r.Context(), req.Email, req.Password, req.InviteCode)
	if err != nil {
		return apperr.Internal(err, "login failed")
	}
	if code != "" {
		return mapLoginCode(code)
	}

	httputil.WriteJSON(w, http.StatusOK, res)
	return nil
}

func (h *Handlers) Me(w http.ResponseWriter, r *http.Request) error {
	uid, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		return apperr.Unauthorized("invalid or expired token")
	}

	res, code, err := h.svc.GetProfile(r.Context(), uid)
	if err != nil {
		return apperr.Internal(err, "load profile failed")
	}
	if code == "unauthorized" {
		return apperr.Unauthorized("invalid or expired token")
	}
	if code != "" {
		return apperr.Invalid(code, "invalid input")
	}

	httputil.WriteJSON(w, http.StatusOK, res)
	return nil
}

func (h *Handlers) Logout(w http.ResponseWriter, r *http.Request) error {
	rawToken, ok := middleware.TokenFromContext(r.Context())
	if !ok {
		return apperr.Unauthorized("invalid or expired token")
	}

	uid, okUID := middleware.UserIDFromContext(r.Context())
	var actor *string
	if okUID {
		actor = &uid
	}

	_, err := h.svc.Logout(r.Context(), rawToken, actor)
	if err != nil {
		return apperr.Internal(err, "logout failed")
	}

	httputil.WriteJSON(w, http.StatusOK, map[string]any{"ok": true})
	return nil
}

type changePasswordReq struct {
	CurrentPassword string `json:"current_password"`
	NewPassword     string `json:"new_password"`
}

func mapChangePasswordCode(code string) error {
	switch code {
	case "unauthorized":
		return apperr.Unauthorized("invalid or expired token")
	case "invalid_current_password":
		return apperr.Invalid(code, "current password is incorrect")
	case "invalid_password":
		return apperr.Invalid(code, "password must be at least 8 characters")
	case "password_unchanged":
		return apperr.Invalid(code, "new password must differ from current password")
	default:
		return apperr.Invalid(code, "invalid input")
	}
}

func (h *Handlers) ChangePassword(w http.ResponseWriter, r *http.Request) error {
	userID, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		return apperr.Unauthorized("invalid or expired token")
	}

	var req changePasswordReq
	if err := httputil.DecodeJSON(r, &req); err != nil {
		return apperr.Invalid("invalid_json", "invalid json body")
	}

	code, err := h.svc.ChangePassword(r.Context(), userID, req.CurrentPassword, req.NewPassword)
	if err != nil {
		return apperr.Internal(err, "change password failed")
	}
	if code != "" {
		return mapChangePasswordCode(code)
	}

	httputil.WriteJSON(w, http.StatusOK, map[string]any{"ok": true})
	return nil
}

type updateProfileReq struct {
	FullName string `json:"full_name"`
	Phone    string `json:"phone"`
}

func mapUpdateProfileCode(code string) error {
	switch code {
	case "unauthorized":
		return apperr.Unauthorized("invalid or expired token")
	case "invalid_full_name":
		return apperr.Invalid(code, "full_name is too long")
	case "invalid_phone":
		return apperr.Invalid(code, "invalid phone")
	default:
		return apperr.Invalid(code, "invalid input")
	}
}

func (h *Handlers) UpdateProfile(w http.ResponseWriter, r *http.Request) error {
	userID, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		return apperr.Unauthorized("invalid or expired token")
	}

	var req updateProfileReq
	if err := httputil.DecodeJSON(r, &req); err != nil {
		return apperr.Invalid("invalid_json", "invalid json body")
	}

	res, code, err := h.svc.UpdateProfile(r.Context(), userID, req.FullName, req.Phone)
	if err != nil {
		return apperr.Internal(err, "update profile failed")
	}
	if code != "" {
		return mapUpdateProfileCode(code)
	}

	httputil.WriteJSON(w, http.StatusOK, res)
	return nil
}
