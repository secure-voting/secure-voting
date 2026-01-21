package auth

import (
	"log"
	"net/http"

	asvc "secure-voting/apps/backend/internal/auth"
	"secure-voting/apps/backend/internal/httpserver/httputil"
	"secure-voting/apps/backend/internal/httpserver/middleware"
)

type Handlers struct {
	svc *asvc.Service
}

func NewHandlers(svc *asvc.Service) *Handlers {
	return &Handlers{svc: svc}
}

type registerReq struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (h *Handlers) Register(w http.ResponseWriter, r *http.Request) {
	var req registerReq
	if err := httputil.DecodeJSON(r, &req); err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "bad_request", "invalid json body")
		return
	}

	res, code, err := h.svc.Register(r.Context(), req.Email, req.Password)
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
		case "email_taken":
			httputil.WriteError(w, http.StatusConflict, "conflict", "email already registered")
		default:
			httputil.WriteError(w, http.StatusBadRequest, "bad_request", "invalid input")
		}
		return
	}

	httputil.WriteJSON(w, http.StatusOK, res)
}

type loginReq struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (h *Handlers) Login(w http.ResponseWriter, r *http.Request) {
	var req loginReq
	if err := httputil.DecodeJSON(r, &req); err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "bad_request", "invalid json body")
		return
	}

	res, code, err := h.svc.Login(r.Context(), req.Email, req.Password)
	if err != nil {
		log.Printf("auth.login error: %v", err)
		httputil.WriteError(w, http.StatusInternalServerError, "internal_error", "login failed")
		return
	}
	if code != "" {
		httputil.WriteError(w, http.StatusUnauthorized, "unauthorized", "invalid credentials")
		return
	}

	httputil.WriteJSON(w, http.StatusOK, res)
}

func (h *Handlers) Me(w http.ResponseWriter, r *http.Request) {
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
	rawToken, ok := middleware.TokenFromContext(r.Context())
	if !ok {
		httputil.WriteError(w, http.StatusUnauthorized, "unauthorized", "invalid or expired token")
		return
	}
	uid, _ := middleware.UserIDFromContext(r.Context())

	_, err := h.svc.Logout(r.Context(), rawToken, &uid)
	if err != nil {
		log.Printf("auth.logout error: %v", err)
		httputil.WriteError(w, http.StatusInternalServerError, "internal_error", "logout failed")
		return
	}

	httputil.WriteJSON(w, http.StatusOK, map[string]any{"ok": true})
}
