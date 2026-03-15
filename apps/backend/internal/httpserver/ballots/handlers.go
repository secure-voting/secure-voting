package ballots

import (
	"context"
	"net/http"
	"strings"

	"secure-voting/apps/backend/internal/apperr"
	"secure-voting/apps/backend/internal/ballots"
	"secure-voting/apps/backend/internal/httpserver/httputil"
	"secure-voting/apps/backend/internal/httpserver/middleware"
)

type Service interface {
	Submit(ctx context.Context, electionID, userID, email, idemKey string, req ballots.SubmitReq) (ballots.SubmitResp, string, error)
	MyBallot(ctx context.Context, electionID, userID, email string) (ballots.MyBallotResp, string, error)
}

type Handlers struct {
	svc Service
}

func NewHandlers(svc Service) *Handlers {
	return &Handlers{svc: svc}
}

func mapSubmitCode(code string) error {
		switch code {
		case "not_found":
			return apperr.NotFound("election not found")
		case "invalid_id":
			return apperr.Invalid(code, "invalid id")
		case "missing_idempotency_key":
			return apperr.Invalid(code, "missing Idempotency-Key header")
		case "invalid_idempotency_key":
			return apperr.Invalid(code, "invalid Idempotency-Key header")
		case "not_active":
			return apperr.Conflict(code, "election is not active")
		case "idempotency_in_progress":
			return apperr.Conflict(code, "idempotency request in progress")
		case "already_submitted":
			return apperr.Conflict(code, "already submitted")
		default:
			return apperr.Invalid(code, code)
		}
}

func mapMeCode(code string) error {
	switch code {
	case "not_found":
		return apperr.NotFound("election not found")
	case "invalid_id":
		return apperr.Invalid(code, "invalid id")
	default:
		return apperr.Invalid(code, code)
	}
}

func (h *Handlers) Submit(w http.ResponseWriter, r *http.Request) error {
	eid := strings.TrimSpace(r.PathValue("id"))
	uid, _ := middleware.UserIDFromContext(r.Context())
	email, _ := middleware.EmailFromContext(r.Context())

	idemKey := strings.TrimSpace(r.Header.Get("Idempotency-Key"))

	var req ballots.SubmitReq
	if err := httputil.DecodeJSON(r, &req); err != nil {
		return apperr.Invalid("invalid_json", "invalid json body")
	}

	res, code, err := h.svc.Submit(r.Context(), eid, uid, email, idemKey, req)
	if err != nil {
		return apperr.Internal(err, "submit ballot failed")
	}
	if code != "" {
		switch code {
		case "invalid_id":
			httputil.WriteError(w, http.StatusBadRequest, "invalid_id", "invalid election id")
			return nil
		case "missing_idempotency_key":
			httputil.WriteError(w, http.StatusBadRequest, "missing_idempotency_key", "idempotency key is required")
			return nil
		case "invalid_idempotency_key":
			httputil.WriteError(w, http.StatusBadRequest, "invalid_idempotency_key", "invalid idempotency key")
			return nil
		case "not_found":
			httputil.WriteError(w, http.StatusNotFound, "not_found", "election not found")
			return nil
		case "not_active":
			httputil.WriteError(w, http.StatusConflict, "not_active", "election is not active")
			return nil
		case "invalid_ballot":
			httputil.WriteError(w, http.StatusBadRequest, "invalid_ballot", "invalid ballot")
			return nil
		case "idempotency_in_progress":
			httputil.WriteError(w, http.StatusConflict, "idempotency_in_progress", "request is already in progress")
			return nil
		case "already_submitted":
			httputil.WriteError(w, http.StatusConflict, "already_submitted", "already submitted")
			return nil
		default:
			httputil.WriteError(w, http.StatusBadRequest, code, code)
			return nil
		}
	}
	httputil.WriteJSON(w, http.StatusOK, res)
	return nil
}

func (h *Handlers) Me(w http.ResponseWriter, r *http.Request) error {
	eid := strings.TrimSpace(r.PathValue("id"))
	uid, _ := middleware.UserIDFromContext(r.Context())
	email, _ := middleware.EmailFromContext(r.Context())

	res, code, err := h.svc.MyBallot(r.Context(), eid, uid, email)
	if err != nil {
		return apperr.Internal(err, "get my ballot failed")
	}
	if code != "" {
		return mapMeCode(code)
	}

	httputil.WriteJSON(w, http.StatusOK, res)
	return nil
}
