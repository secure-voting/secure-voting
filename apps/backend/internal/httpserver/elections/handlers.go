package elections

import (
	"context"
	"log"
	"net/http"
	"strings"

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

func (h *Handlers) Create(w http.ResponseWriter, r *http.Request) {
	uid, _ := middleware.UserIDFromContext(r.Context())

	var req elections.CreateElectionInput
	if err := httputil.DecodeJSON(r, &req); err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "bad_request", "invalid json body")
		return
	}

	id, code, err := h.svc.Create(r.Context(), uid, req)
	if err != nil {
		log.Printf("elections.create error: %v", err)
		httputil.WriteError(w, http.StatusInternalServerError, "internal_error", "create election failed")
		return
	}
	if code != "" {
		httputil.WriteError(w, http.StatusBadRequest, "bad_request", code)
		return
	}

	httputil.WriteJSON(w, http.StatusOK, map[string]any{"id": id})
}

func (h *Handlers) List(w http.ResponseWriter, r *http.Request) {
	uid, _ := middleware.UserIDFromContext(r.Context())
	email, _ := middleware.EmailFromContext(r.Context())
	role, _ := middleware.RoleFromContext(r.Context())

	items, err := h.svc.ListForUser(r.Context(), uid, email, role)
	if err != nil {
		log.Printf("elections.list error: %v", err)
		httputil.WriteError(w, http.StatusInternalServerError, "internal_error", "list elections failed")
		return
	}
	httputil.WriteJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *Handlers) Get(w http.ResponseWriter, r *http.Request) {
	eid := strings.TrimSpace(r.PathValue("id"))
	uid, _ := middleware.UserIDFromContext(r.Context())
	email, _ := middleware.EmailFromContext(r.Context())
	role, _ := middleware.RoleFromContext(r.Context())

	item, code, err := h.svc.Get(r.Context(), eid, uid, email, role)
	if err != nil {
		log.Printf("elections.get error: %v", err)
		httputil.WriteError(w, http.StatusInternalServerError, "internal_error", "get election failed")
		return
	}
	if code != "" {
		switch code {
		case "not_found":
			httputil.WriteError(w, http.StatusNotFound, "not_found", "election not found")
		case "invalid_id":
			httputil.WriteError(w, http.StatusBadRequest, "bad_request", "invalid id")
		default:
			httputil.WriteError(w, http.StatusBadRequest, "bad_request", code)
		}
		return
	}

	httputil.WriteJSON(w, http.StatusOK, item)
}

func (h *Handlers) BallotMeta(w http.ResponseWriter, r *http.Request) {
	eid := strings.TrimSpace(r.PathValue("id"))
	uid, _ := middleware.UserIDFromContext(r.Context())
	email, _ := middleware.EmailFromContext(r.Context())
	role, _ := middleware.RoleFromContext(r.Context())

	meta, code, err := h.svc.GetBallotMeta(r.Context(), eid, uid, email, role)
	if err != nil {
		log.Printf("elections.ballot_meta error: %v", err)
		httputil.WriteError(w, http.StatusInternalServerError, "internal_error", "get ballot meta failed")
		return
	}
	if code != "" {
		if code == "not_found" {
			httputil.WriteError(w, http.StatusNotFound, "not_found", "election not found")
		} else {
			httputil.WriteError(w, http.StatusBadRequest, "bad_request", code)
		}
		return
	}

	httputil.WriteJSON(w, http.StatusOK, meta)
}

func (h *Handlers) UpdateRules(w http.ResponseWriter, r *http.Request) {
	eid := strings.TrimSpace(r.PathValue("id"))
	uid, _ := middleware.UserIDFromContext(r.Context())

	var req elections.UpdateRulesInput
	if err := httputil.DecodeJSON(r, &req); err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "bad_request", "invalid json body")
		return
	}

	code, err := h.svc.UpdateRules(r.Context(), eid, uid, req)
	if err != nil {
		log.Printf("elections.update_rules error: %v", err)
		httputil.WriteError(w, http.StatusInternalServerError, "internal_error", "update rules failed")
		return
	}
	if code != "" {
		switch code {
		case "not_found":
			httputil.WriteError(w, http.StatusNotFound, "not_found", "election not found")
		case "invalid_status":
			httputil.WriteError(w, http.StatusConflict, "conflict", "rules can be updated only in draft/scheduled")
		default:
			httputil.WriteError(w, http.StatusBadRequest, "bad_request", code)
		}
		return
	}

	httputil.WriteJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (h *Handlers) Action(w http.ResponseWriter, r *http.Request) {
	eid := strings.TrimSpace(r.PathValue("id"))
	action := strings.TrimSpace(r.PathValue("action"))
	uid, _ := middleware.UserIDFromContext(r.Context())

	code, err := h.svc.Action(r.Context(), eid, uid, action)
	if err != nil {
		log.Printf("elections.action error: %v", err)
		httputil.WriteError(w, http.StatusInternalServerError, "internal_error", "action failed")
		return
	}
	if code != "" {
		switch code {
		case "not_found":
			httputil.WriteError(w, http.StatusNotFound, "not_found", "election not found")
		case "invalid_transition":
			httputil.WriteError(w, http.StatusConflict, "conflict", "invalid state transition")
		default:
			httputil.WriteError(w, http.StatusBadRequest, "bad_request", code)
		}
		return
	}

	httputil.WriteJSON(w, http.StatusOK, map[string]any{"ok": true})
}

type createInviteReq struct {
	Email string `json:"email"`
}

func (h *Handlers) CreateInvite(w http.ResponseWriter, r *http.Request) {
	eid := strings.TrimSpace(r.PathValue("id"))
	uid, _ := middleware.UserIDFromContext(r.Context())

	var req createInviteReq
	if err := httputil.DecodeJSON(r, &req); err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "bad_request", "invalid json body")
		return
	}

	res, code, err := h.svc.CreateInvite(r.Context(), eid, uid, req.Email)
	if err != nil {
		log.Printf("elections.create_invite error: %v", err)
		httputil.WriteError(w, http.StatusInternalServerError, "internal_error", "create invite failed")
		return
	}
	if code != "" {
		switch code {
		case "not_found":
			httputil.WriteError(w, http.StatusNotFound, "not_found", "election not found")
		case "email_already_invited":
			httputil.WriteError(w, http.StatusConflict, "conflict", "email already invited")
		case "not_invite_mode":
			httputil.WriteError(w, http.StatusConflict, "conflict", "election is not in invite mode")
		default:
			httputil.WriteError(w, http.StatusBadRequest, "bad_request", code)
		}
		return
	}

	httputil.WriteJSON(w, http.StatusOK, res)
}

func (h *Handlers) ListInvites(w http.ResponseWriter, r *http.Request) {
	eid := strings.TrimSpace(r.PathValue("id"))
	uid, _ := middleware.UserIDFromContext(r.Context())

	items, code, err := h.svc.ListInvites(r.Context(), eid, uid)
	if err != nil {
		log.Printf("elections.list_invites error: %v", err)
		httputil.WriteError(w, http.StatusInternalServerError, "internal_error", "list invites failed")
		return
	}
	if code != "" {
		switch code {
		case "not_found":
			httputil.WriteError(w, http.StatusNotFound, "not_found", "election not found")
		default:
			httputil.WriteError(w, http.StatusBadRequest, "bad_request", code)
		}
		return
	}

	httputil.WriteJSON(w, http.StatusOK, map[string]any{"items": items})
}
