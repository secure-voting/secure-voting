package auth

import (
	"context"
	"errors"
	"strings"

	"github.com/jackc/pgx/v5"
)

func (s *Service) acceptInviteTx(ctx context.Context, tx txLike, email, inviteCode string) (acceptedInvite, string, error) {
	inviteCode = strings.TrimSpace(inviteCode)
	if inviteCode == "" {
		return acceptedInvite{}, "", nil
	}

	inviteHashHex := sha256Hex(inviteCode)

	var inviteID, inviteEmail, inviteStatus, inviteElectionID string
	err := tx.QueryRow(ctx, `
		SELECT id::text, email, status, election_id::text
		FROM election_invites
		WHERE invite_code_hash = $1
		LIMIT 1
	`, inviteHashHex).Scan(&inviteID, &inviteEmail, &inviteStatus, &inviteElectionID)

	if errors.Is(err, pgx.ErrNoRows) {
		return acceptedInvite{}, "invalid_invite_code", nil
	}
	if err != nil {
		return acceptedInvite{}, "", err
	}

	if inviteStatus != "created" && inviteStatus != "sent" {
		return acceptedInvite{}, "invite_code_inactive", nil
	}

	if strings.TrimSpace(inviteEmail) != "" && strings.ToLower(strings.TrimSpace(inviteEmail)) != email {
		return acceptedInvite{}, "invite_email_mismatch", nil
	}

	ct, err := tx.Exec(ctx, `
		UPDATE election_invites
		SET status = 'accepted', accepted_at = now()
		WHERE id = $1::uuid AND status IN ('created','sent')
	`, inviteID)
	if err != nil {
		return acceptedInvite{}, "", err
	}
	if ct.RowsAffected() == 0 {
		return acceptedInvite{}, "invite_code_inactive", nil
	}

	return acceptedInvite{ID: inviteID, ElectionID: inviteElectionID}, "", nil
}
