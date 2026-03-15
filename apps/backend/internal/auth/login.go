package auth

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"golang.org/x/crypto/bcrypt"
)

func (s *Service) Login(ctx context.Context, email, password, inviteCode string) (AuthResult, string, error) {
	email = strings.TrimSpace(strings.ToLower(email))
	inviteCode = strings.TrimSpace(inviteCode)

	if !ValidateEmail(email) {
		return AuthResult{}, "invalid_email", nil
	}
	if password == "" {
		return AuthResult{}, "invalid_password", nil
	}

	var userID, dbEmail, role, passHash string
	err := s.db.QueryRow(ctx,
		`SELECT id::text, email, role, password_hash
		 FROM users
		 WHERE email = $1`,
		email,
	).Scan(&userID, &dbEmail, &role, &passHash)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return AuthResult{}, "invalid_credentials", nil
		}
		return AuthResult{}, "", err
	}

	if err := bcrypt.CompareHashAndPassword([]byte(passHash), []byte(password)); err != nil {
		return AuthResult{}, "invalid_credentials", nil
	}

	tx, err := s.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return AuthResult{}, "", err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var inv acceptedInvite
	if inviteCode != "" {
		got, code, err := s.acceptInviteTx(ctx, tx, email, inviteCode)
		if err != nil {
			return AuthResult{}, "", err
		}
		if code != "" {
			return AuthResult{}, code, nil
		}
		inv = got

		_ = s.insertAudit(ctx, tx, &userID, "invite_accepted", map[string]any{
			"target_type": "election_invite",
			"target_id":   inv.ID,
			"details": map[string]any{
				"election_id": inv.ElectionID,
				"email":       email,
			},
		})
	}

	token, _, expiresAt, err := s.issueToken(ctx, tx, userID)
	if err != nil {
		return AuthResult{}, "", err
	}

	loginDetails := map[string]any{
		"target_type": "user",
		"target_id":   userID,
	}
	if inviteCode != "" {
		loginDetails["invite"] = map[string]any{
			"id":          inv.ID,
			"election_id": inv.ElectionID,
		}
	}
	_ = s.insertAudit(ctx, tx, &userID, "user_logged_in", loginDetails)

	if err := tx.Commit(ctx); err != nil {
		return AuthResult{}, "", err
	}

	return AuthResult{
		AccessToken: token,
		ExpiresAt:   expiresAt.UTC().Format(time.RFC3339),
		User: User{
			ID:    userID,
			Email: dbEmail,
			Role:  role,
		},
	}, "", nil
}
