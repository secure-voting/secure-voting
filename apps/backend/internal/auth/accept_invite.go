package auth

import (
	"context"
	"errors"
	"strings"

	"github.com/jackc/pgx/v5"
)

func (s *Service) AcceptInvite(ctx context.Context, userID, inviteCode string) (AcceptInviteResult, string, error) {
	userID = strings.TrimSpace(userID)
	inviteCode = strings.TrimSpace(inviteCode)

	if userID == "" {
		return AcceptInviteResult{}, "unauthorized", nil
	}
	if inviteCode == "" {
		return AcceptInviteResult{}, "invalid_invite_code", nil
	}

	tx, err := authBeginTxFn(ctx, s.db)
	if err != nil {
		return AcceptInviteResult{}, "", err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var email string
	err = tx.QueryRow(ctx, `
		SELECT email
		FROM users
		WHERE id = $1::uuid
		FOR UPDATE
	`, userID).Scan(&email)
	if errors.Is(err, pgx.ErrNoRows) {
		return AcceptInviteResult{}, "unauthorized", nil
	}
	if err != nil {
		return AcceptInviteResult{}, "", err
	}

	inv, code, err := s.acceptInviteTx(ctx, tx, strings.ToLower(strings.TrimSpace(email)), inviteCode)
	if err != nil {
		return AcceptInviteResult{}, "", err
	}
	if code != "" {
		return AcceptInviteResult{}, code, nil
	}

	_ = s.insertAudit(ctx, tx, &userID, "invite_accepted", map[string]any{
		"target_type": "election_invite",
		"target_id":   inv.ID,
		"details": map[string]any{
			"election_id": inv.ElectionID,
			"email":       email,
		},
	})

	if err := tx.Commit(ctx); err != nil {
		return AcceptInviteResult{}, "", err
	}

	return AcceptInviteResult{
		OK:         true,
		InviteID:   inv.ID,
		ElectionID: inv.ElectionID,
	}, "", nil
}
