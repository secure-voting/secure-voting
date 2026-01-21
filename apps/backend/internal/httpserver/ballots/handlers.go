package ballots

import (
	"log"
	"net/http"
	"strings"

	"secure-voting/apps/backend/internal/ballots"
	"secure-voting/apps/backend/internal/httpserver/httputil"
	"secure-voting/apps/backend/internal/httpserver/middleware"
)

type Handlers struct {
	svc *ballots.Service
}

func NewHandlers(svc *ballots.Service) *Handlers {
	return &Handlers{svc: svc}
}

func (h *Handlers) Submit(w http.ResponseWriter, r *http.Request) {
	eid := strings.TrimSpace(r.PathValue("id"))
	uid, _ := middleware.UserIDFromContext(r.Context())

	idemKey := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
	var req ballots.SubmitReq
	if err := httputil.DecodeJSON(r, &req); err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "bad_request", "invalid json body")
		return
	}

	res, code, err := h.svc.Submit(r.Context(), eid, uid, idemKey, req)
	if err != nil {
		log.Printf("ballots.submit error: %v", err)
		httputil.WriteError(w, http.StatusInternalServerError, "internal_error", "submit ballot failed")
		return
	}
	if code != "" {
		switch code {
		case "not_found":
			httputil.WriteError(w, http.StatusNotFound, "not_found", "election not found")
		case "missing_idempotency_key":
			httputil.WriteError(w, http.StatusBadRequest, "bad_request", "missing Idempotency-Key header")
		case "election_not_active":
			httputil.WriteError(w, http.StatusConflict, "conflict", "election is not active")
		case "already_submitted":
			httputil.WriteError(w, http.StatusConflict, "conflict", "already submitted")
		default:
			httputil.WriteError(w, http.StatusBadRequest, "bad_request", code)
		}
		return
	}

	httputil.WriteJSON(w, http.StatusOK, res)
}

func (h *Handlers) Me(w http.ResponseWriter, r *http.Request) {
	eid := strings.TrimSpace(r.PathValue("id"))
	uid, _ := middleware.UserIDFromContext(r.Context())

	res, code, err := h.svc.MyBallot(r.Context(), eid, uid)
	if err != nil {
		log.Printf("ballots.me error: %v", err)
		httputil.WriteError(w, http.StatusInternalServerError, "internal_error", "get my ballot failed")
		return
	}
	if code != "" {
		httputil.WriteError(w, http.StatusBadRequest, "bad_request", code)
		return
	}
	httputil.WriteJSON(w, http.StatusOK, res)
}
