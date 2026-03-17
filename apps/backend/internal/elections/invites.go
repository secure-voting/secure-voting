package elections

import (
	"context"
	"errors"
	"net/mail"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func (s *Service) CreateInvite(ctx context.Context, electionID, adminUserID, email string) (InviteCreated, string, error) {
	if _, err := uuid.Parse(electionID); err != nil {
		return InviteCreated{}, "invalid_id", nil
	}

	email = strings.TrimSpace(strings.ToLower(email))
	if email == "" {
		return InviteCreated{}, "invalid_email", nil
	}
	if _, err := mail.ParseAddress(email); err != nil {
		return InviteCreated{}, "invalid_email", nil
	}

	tx, err := s.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return InviteCreated{}, "", err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var accessMode string
	err = tx.QueryRow(ctx, `
		SELECT access_mode
		FROM elections
		WHERE id = $1::uuid AND created_by = $2::uuid
		FOR UPDATE
	`, electionID, adminUserID).Scan(&accessMode)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return InviteCreated{}, "not_found", nil
		}
		return InviteCreated{}, "", err
	}
	if accessMode != "invite" {
		return InviteCreated{}, "not_invite_mode", nil
	}

	var registered bool
	err = tx.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1
			FROM users
			WHERE lower(email) = lower($1)
		)
	`, email).Scan(&registered)
	if err != nil {
		return InviteCreated{}, "", err
	}

	if !registered {
		return InviteCreated{}, "registration_required", nil
	}

	rawCode, codeHash := generateInviteCode()

	var inviteID string
	var createdAt time.Time
	err = tx.QueryRow(ctx, `
		INSERT INTO election_invites (election_id, email, invite_code_hash, status)
		VALUES ($1::uuid, $2, $3, 'created')
		RETURNING id::text, created_at
	`, electionID, email, codeHash).Scan(&inviteID, &createdAt)

	if err != nil {
		low := strings.ToLower(err.Error())
		if strings.Contains(low, "unique") || strings.Contains(low, "duplicate") {
			return InviteCreated{}, "email_already_invited", nil
		}
		return InviteCreated{}, "", err
	}

	_ = insertAudit(ctx, tx, &adminUserID, "invite_created", map[string]any{
		"target_type": "election_invite",
		"target_id":   inviteID,
		"after": map[string]any{
			"election_id":           electionID,
			"email":                 email,
			"status":                "created",
			"registration_required": !registered,
		},
	})

	if err := tx.Commit(ctx); err != nil {
		return InviteCreated{}, "", err
	}

	resp := InviteCreated{
		InviteID:              inviteID,
		Email:                 email,
		InviteCode:            rawCode,
		Status:                "created",
		CreatedAt:             createdAt.UTC().Format(time.RFC3339),
		RegistrationRequired:  false,
		RegistrationEmailSent: false,
		InviteEmailSent:       false,
	}

	return resp, "", nil
}

func (s *Service) ListInvites(ctx context.Context, electionID, adminUserID string) ([]Invite, string, error) {
	if _, err := uuid.Parse(electionID); err != nil {
		return nil, "invalid_id", nil
	}

	var exists int
	err := s.db.QueryRow(ctx, `
		SELECT 1
		FROM elections
		WHERE id = $1::uuid AND created_by = $2::uuid
	`, electionID, adminUserID).Scan(&exists)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, "not_found", nil
		}
		return nil, "", err
	}

	rows, err := s.db.Query(ctx, `
		SELECT id::text, email, status, sent_at, accepted_at, created_at
		FROM election_invites
		WHERE election_id = $1::uuid
		ORDER BY created_at DESC
	`, electionID)
	if err != nil {
		return nil, "", err
	}
	defer rows.Close()

	out := make([]Invite, 0)
	for rows.Next() {
		var it Invite
		var sentAt *time.Time
		var accAt *time.Time
		var createdAt time.Time

		if err := rows.Scan(&it.ID, &it.Email, &it.Status, &sentAt, &accAt, &createdAt); err != nil {
			return nil, "", err
		}

		it.CreatedAt = createdAt.UTC().Format(time.RFC3339)
		if sentAt != nil {
			v := sentAt.UTC().Format(time.RFC3339)
			it.SentAt = &v
		}
		if accAt != nil {
			v := accAt.UTC().Format(time.RFC3339)
			it.AcceptedAt = &v
		}

		out = append(out, it)
	}

	if err := rows.Err(); err != nil {
		return nil, "", err
	}

	return out, "", nil
}
