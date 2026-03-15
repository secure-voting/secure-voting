package elections

import (
	"context"
	"net/http"
	"strings"

	"secure-voting/apps/backend/internal/apperr"
	"secure-voting/apps/backend/internal/elections"
	"secure-voting/apps/backend/internal/httpserver/httputil"
	"secure-voting/apps/backend/internal/httpserver/middleware"
)

type Service interface {
	Create(ctx context.Context, createdBy string, in elections.CreateElectionInput) (string, string, error)
	ListForUser(ctx context.Context, userID, email, role string) ([]elections.ElectionSummary, error)
	Get(ctx context.Context, electionID, userID, email, role string) (elections.ElectionDetail, string, error)
	GetBallotMeta(ctx context.Context, electionID, userID, email, role string) (elections.BallotMeta, string, error)
	UpdateRules(ctx context.Context, electionID, adminUserID string, in elections.UpdateRulesInput) (string, error)
	Action(ctx context.Context, electionID, adminUserID, action string) (string, error)
	CreateInvite(ctx context.Context, electionID, adminUserID, email string) (elections.InviteCreated, string, error)
	ListInvites(ctx context.Context, electionID, adminUserID string) ([]elections.Invite, string, error)
}

type Handlers struct {
	svc Service
}

func NewHandlers(svc Service) *Handlers {
	return &Handlers{svc: svc}
}

func (h *Handlers) Create(w http.ResponseWriter, r *http.Request) error {
	uid, _ := middleware.UserIDFromContext(r.Context())

	var req elections.CreateElectionInput
	if err := httputil.DecodeJSON(r, &req); err != nil {
		return apperr.Invalid("invalid_json", "invalid json body")
	}

	id, code, err := h.svc.Create(r.Context(), uid, req)
	if err != nil {
		return apperr.Internal(err, "create election failed")
	}
	if code != "" {
		// пока сохраняем текущее поведение: message = code (контракт)
		return apperr.Invalid(code, code)
	}

	httputil.WriteJSON(w, http.StatusOK, map[string]any{"id": id})
	return nil
}

func (h *Handlers) List(w http.ResponseWriter, r *http.Request) error {
	uid, _ := middleware.UserIDFromContext(r.Context())
	email, _ := middleware.EmailFromContext(r.Context())
	role, _ := middleware.RoleFromContext(r.Context())

	items, err := h.svc.ListForUser(r.Context(), uid, email, role)
	if err != nil {
		return apperr.Internal(err, "list elections failed")
	}

	httputil.WriteJSON(w, http.StatusOK, map[string]any{"items": items})
	return nil
}

func (h *Handlers) Get(w http.ResponseWriter, r *http.Request) error {
	eid := strings.TrimSpace(r.PathValue("id"))
	uid, _ := middleware.UserIDFromContext(r.Context())
	email, _ := middleware.EmailFromContext(r.Context())
	role, _ := middleware.RoleFromContext(r.Context())

	item, code, err := h.svc.Get(r.Context(), eid, uid, email, role)
	if err != nil {
		return apperr.Internal(err, "get election failed")
	}
	if code != "" {
		switch code {
		case "not_found":
			return apperr.NotFound("election not found")
		case "invalid_id":
			return apperr.Invalid(code, "invalid id")
		default:
			return apperr.Invalid(code, code)
		}
	}

	httputil.WriteJSON(w, http.StatusOK, item)
	return nil
}

func (h *Handlers) BallotMeta(w http.ResponseWriter, r *http.Request) error {
	eid := strings.TrimSpace(r.PathValue("id"))
	uid, _ := middleware.UserIDFromContext(r.Context())
	email, _ := middleware.EmailFromContext(r.Context())
	role, _ := middleware.RoleFromContext(r.Context())

	meta, code, err := h.svc.GetBallotMeta(r.Context(), eid, uid, email, role)
	if err != nil {
		return apperr.Internal(err, "get ballot meta failed")
	}
	if code != "" {
		if code == "not_found" {
			return apperr.NotFound("election not found")
		}
		return apperr.Invalid(code, code)
	}

	httputil.WriteJSON(w, http.StatusOK, meta)
	return nil
}

func (h *Handlers) UpdateRules(w http.ResponseWriter, r *http.Request) error {
	eid := strings.TrimSpace(r.PathValue("id"))
	uid, _ := middleware.UserIDFromContext(r.Context())

	var req elections.UpdateRulesInput
	if err := httputil.DecodeJSON(r, &req); err != nil {
		return apperr.Invalid("invalid_json", "invalid json body")
	}

	code, err := h.svc.UpdateRules(r.Context(), eid, uid, req)
	if err != nil {
		return apperr.Internal(err, "update rules failed")
	}
	if code != "" {
		switch code {
		case "not_found":
			return apperr.NotFound("election not found")
		case "invalid_status":
			return apperr.Conflict(code, "rules can be updated only in draft/scheduled")
		default:
			return apperr.Invalid(code, code)
		}
	}

	httputil.WriteJSON(w, http.StatusOK, map[string]any{"ok": true})
	return nil
}

func (h *Handlers) Action(w http.ResponseWriter, r *http.Request) error {
	eid := strings.TrimSpace(r.PathValue("id"))
	action := strings.TrimSpace(r.PathValue("action"))
	uid, _ := middleware.UserIDFromContext(r.Context())

	code, err := h.svc.Action(r.Context(), eid, uid, action)
	if err != nil {
		return apperr.Internal(err, "action failed")
	}
	if code != "" {
		switch code {
		case "not_found":
			return apperr.NotFound("election not found")
		case "invalid_transition":
			return apperr.Conflict(code, "invalid state transition")
		default:
			return apperr.Invalid(code, code)
		}
	}

	httputil.WriteJSON(w, http.StatusOK, map[string]any{"ok": true})
	return nil
}

type createInviteReq struct {
	Email string `json:"email"`
}

func (h *Handlers) CreateInvite(w http.ResponseWriter, r *http.Request) error {
	eid := strings.TrimSpace(r.PathValue("id"))
	uid, _ := middleware.UserIDFromContext(r.Context())

	var req createInviteReq
	if err := httputil.DecodeJSON(r, &req); err != nil {
		return apperr.Invalid("invalid_json", "invalid json body")
	}

	res, code, err := h.svc.CreateInvite(r.Context(), eid, uid, req.Email)
	if err != nil {
		return apperr.Internal(err, "create invite failed")
	}
	if code != "" {
		switch code {
		case "not_found":
			return apperr.NotFound("election not found")
		case "email_already_invited":
			return apperr.Conflict(code, "email already invited")
		case "not_invite_mode":
			return apperr.Conflict(code, "election is not in invite mode")
		default:
			return apperr.Invalid(code, code)
		}
	}

	httputil.WriteJSON(w, http.StatusOK, res)
	return nil
}

func (h *Handlers) ListInvites(w http.ResponseWriter, r *http.Request) error {
	eid := strings.TrimSpace(r.PathValue("id"))
	uid, _ := middleware.UserIDFromContext(r.Context())

	items, code, err := h.svc.ListInvites(r.Context(), eid, uid)
	if err != nil {
		return apperr.Internal(err, "list invites failed")
	}
	if code != "" {
		switch code {
		case "not_found":
			return apperr.NotFound("election not found")
		default:
			return apperr.Invalid(code, code)
		}
	}

	httputil.WriteJSON(w, http.StatusOK, map[string]any{"items": items})
	return nil
}
