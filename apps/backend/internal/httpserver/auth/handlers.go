package auth

import (
	"context"
	"log"
	"net/http"

	asvc "secure-voting/apps/backend/internal/auth"
	"secure-voting/apps/backend/internal/httpserver/httputil"
	"secure-voting/apps/backend/internal/httpserver/middleware"
)

type AuthService interface {
	Register(ctx context.Context, email, password, role, inviteCode string) (asvc.AuthResult, string, error)
	Login(ctx context.Context, email, password, inviteCode string) (asvc.AuthResult, string, error)
	Logout(ctx context.Context, rawToken string, actorUserID *string) (bool, error)
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
	Role       string `json:"role,omitempty"`
	InviteCode string `json:"invite_code,omitempty"`
}

func (h *Handlers) Register(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		httputil.WriteError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}

	var req registerReq
	if err := httputil.DecodeJSON(r, &req); err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "bad_request", "invalid json body")
		return
	}

	res, code, err := h.svc.Register(r.Context(), req.Email, req.Password, req.Role, req.InviteCode)
	if err != nil {
		log.Printf("auth.register error: %v", err)
		httputil.WriteError(w, http.StatusInternalServerError, "internal_error", "register failed")
		return
	}
	if code != "" {
		switch code {
		case "invalid_email":
			httputil.WriteError(w, http.StatusBadRequest, "bad_request", "invalid email")
		case "invalid_password":
			httputil.WriteError(w, http.StatusBadRequest, "bad_request", "password must be at least 8 characters")
		case "invalid_role":
			httputil.WriteError(w, http.StatusBadRequest, "bad_request", "invalid role")
		case "email_taken":
			httputil.WriteError(w, http.StatusConflict, "conflict", "email already registered")
		case "invalid_invite_code":
			httputil.WriteError(w, http.StatusBadRequest, "bad_request", "invalid invite_code")
		case "invite_code_inactive":
			httputil.WriteError(w, http.StatusBadRequest, "bad_request", "invite_code is not active")
		case "invite_email_mismatch":
			httputil.WriteError(w, http.StatusBadRequest, "bad_request", "invite_code does not match email")
		default:
			httputil.WriteError(w, http.StatusBadRequest, "bad_request", "invalid input")
		}
		return
	}

	httputil.WriteJSON(w, http.StatusOK, res)
}

type loginReq struct {
	Email      string `json:"email"`
	Password   string `json:"password"`
	InviteCode string `json:"invite_code,omitempty"`
}

func (h *Handlers) Login(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		httputil.WriteError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}

	var req loginReq
	if err := httputil.DecodeJSON(r, &req); err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "bad_request", "invalid json body")
		return
	}

	res, code, err := h.svc.Login(r.Context(), req.Email, req.Password, req.InviteCode)
	if err != nil {
		log.Printf("auth.login error: %v", err)
		httputil.WriteError(w, http.StatusInternalServerError, "internal_error", "login failed")
		return
	}
	if code != "" {
		switch code {
		case "invalid_credentials":
			httputil.WriteError(w, http.StatusUnauthorized, "unauthorized", "invalid credentials")
		case "invalid_email":
			httputil.WriteError(w, http.StatusBadRequest, "bad_request", "invalid email")
		case "invalid_password":
			httputil.WriteError(w, http.StatusBadRequest, "bad_request", "invalid password")
		case "invalid_invite_code":
			httputil.WriteError(w, http.StatusBadRequest, "bad_request", "invalid invite_code")
		case "invite_code_inactive":
			httputil.WriteError(w, http.StatusBadRequest, "bad_request", "invite_code is not active")
		case "invite_email_mismatch":
			httputil.WriteError(w, http.StatusBadRequest, "bad_request", "invite_code does not match email")
		default:
			httputil.WriteError(w, http.StatusBadRequest, "bad_request", "invalid input")
		}
		return
	}

	httputil.WriteJSON(w, http.StatusOK, res)
}

func (h *Handlers) Me(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		httputil.WriteError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}

	uid, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		httputil.WriteError(w, http.StatusUnauthorized, "unauthorized", "invalid or expired token")
		return
	}
	email, _ := middleware.EmailFromContext(r.Context())
	role, _ := middleware.RoleFromContext(r.Context())

	httputil.WriteJSON(w, http.StatusOK, asvc.User{
		ID:    uid,
		Email: email,
		Role:  role,
	})
}

func (h *Handlers) Logout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		httputil.WriteError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}

	rawToken, ok := middleware.TokenFromContext(r.Context())
	if !ok {
		httputil.WriteError(w, http.StatusUnauthorized, "unauthorized", "invalid or expired token")
		return
	}

	uid, okUID := middleware.UserIDFromContext(r.Context())
	var actor *string
	if okUID {
		actor = &uid
	}

	_, err := h.svc.Logout(r.Context(), rawToken, actor)
	if err != nil {
		log.Printf("auth.logout error: %v", err)
		httputil.WriteError(w, http.StatusInternalServerError, "internal_error", "logout failed")
		return
	}

	httputil.WriteJSON(w, http.StatusOK, map[string]any{"ok": true})
}
