package auth

import (
	"context"
	"net"
	"net/http"
	"strings"

	"secure-voting/apps/backend/internal/apperr"
	asvc "secure-voting/apps/backend/internal/auth"
	"secure-voting/apps/backend/internal/httpserver/httputil"
	"secure-voting/apps/backend/internal/httpserver/middleware"
)

type AuthService interface {
	Register(ctx context.Context, email, password, role, inviteCode string) (asvc.AuthResult, string, error)
	Login(ctx context.Context, email, password, inviteCode string, opts asvc.LoginOptions) (asvc.AuthResult, string, error)
	Refresh(ctx context.Context, refreshToken string) (asvc.AuthResult, string, error)
	Logout(ctx context.Context, rawToken string, actorUserID *string) (bool, error)
	ChangePassword(ctx context.Context, userID, currentPassword, newPassword string) (string, error)
	GetProfile(ctx context.Context, userID string) (asvc.User, string, error)
	UpdateProfile(ctx context.Context, userID, fullName, phone string) (asvc.User, string, error)
	RequestEmailVerification(ctx context.Context, userID string) (asvc.EmailVerificationRequestResult, string, error)
	ConfirmEmailVerification(ctx context.Context, userID, code string) (asvc.User, string, error)
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
	Email                  string `json:"email"`
	Password               string `json:"password"`
	InviteCode             string `json:"invite_code,omitempty"`
	ReplaceExistingSession bool   `json:"replace_existing_session,omitempty"`
}

type refreshReq struct {
	RefreshToken string `json:"refresh_token"`
}

func clientIPAddress(r *http.Request) string {
	forwardedFor := strings.TrimSpace(r.Header.Get("X-Forwarded-For"))
	if forwardedFor != "" {
		parts := strings.Split(forwardedFor, ",")
		if len(parts) > 0 {
			return strings.TrimSpace(parts[0])
		}
	}

	realIP := strings.TrimSpace(r.Header.Get("X-Real-IP"))
	if realIP != "" {
		return realIP
	}

	host, _, err := net.SplitHostPort(strings.TrimSpace(r.RemoteAddr))
	if err == nil {
		return host
	}

	return strings.TrimSpace(r.RemoteAddr)
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

func mapRefreshCode(code string) error {
	switch code {
	case "invalid_refresh_token":
		return apperr.Unauthorized("invalid refresh token")
	default:
		return apperr.Invalid(code, "invalid input")
	}
}

func (h *Handlers) Login(w http.ResponseWriter, r *http.Request) error {
	var req loginReq
	if err := httputil.DecodeJSON(r, &req); err != nil {
		return apperr.Invalid("invalid_json", "invalid json body")
	}

	res, code, err := h.svc.Login(r.Context(), req.Email, req.Password, req.InviteCode, asvc.LoginOptions{
		ReplaceExistingSession: req.ReplaceExistingSession,
		UserAgent:              r.UserAgent(),
		IPAddress:              clientIPAddress(r),
	})
	if err != nil {
		return apperr.Internal(err, "login failed")
	}
	if code == "active_session_exists" {
		httputil.WriteJSON(w, http.StatusConflict, map[string]any{
			"error": map[string]any{
				"code":    "active_session_exists",
				"message": "active session already exists",
			},
		})
		return nil
	}
	if code != "" {
		return mapLoginCode(code)
	}

	httputil.WriteJSON(w, http.StatusOK, res)
	return nil
}

func (h *Handlers) Refresh(w http.ResponseWriter, r *http.Request) error {
	var req refreshReq
	if err := httputil.DecodeJSON(r, &req); err != nil {
		return apperr.Invalid("invalid_json", "invalid json body")
	}

	res, code, err := h.svc.Refresh(r.Context(), req.RefreshToken)
	if err != nil {
		return apperr.Internal(err, "refresh token failed")
	}
	if code != "" {
		return mapRefreshCode(code)
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

type confirmEmailVerificationReq struct {
	Code string `json:"code"`
}

func mapEmailVerificationCode(code string) error {
	switch code {
	case "unauthorized":
		return apperr.Unauthorized("invalid or expired token")
	case "email_delivery_not_configured":
		return apperr.Invalid(code, "email delivery is not configured")
	case "invalid_verification_code":
		return apperr.Invalid(code, "invalid verification code")
	case "verification_code_expired":
		return apperr.Invalid(code, "verification code has expired")
	case "verification_attempts_exceeded":
		return apperr.Invalid(code, "verification attempts exceeded")
	default:
		return apperr.Invalid(code, "invalid input")
	}
}

func (h *Handlers) RequestEmailVerification(w http.ResponseWriter, r *http.Request) error {
	userID, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		return apperr.Unauthorized("invalid or expired token")
	}

	res, code, err := h.svc.RequestEmailVerification(r.Context(), userID)
	if err != nil {
		return apperr.Internal(err, "request email verification failed")
	}
	if code != "" {
		return mapEmailVerificationCode(code)
	}

	httputil.WriteJSON(w, http.StatusOK, res)
	return nil
}

func (h *Handlers) ConfirmEmailVerification(w http.ResponseWriter, r *http.Request) error {
	userID, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		return apperr.Unauthorized("invalid or expired token")
	}

	var req confirmEmailVerificationReq
	if err := httputil.DecodeJSON(r, &req); err != nil {
		return apperr.Invalid("invalid_json", "invalid json body")
	}

	res, code, err := h.svc.ConfirmEmailVerification(r.Context(), userID, req.Code)
	if err != nil {
		return apperr.Internal(err, "confirm email verification failed")
	}
	if code != "" {
		return mapEmailVerificationCode(code)
	}

	httputil.WriteJSON(w, http.StatusOK, res)
	return nil
}
