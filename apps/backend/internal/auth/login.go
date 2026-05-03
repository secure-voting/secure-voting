package auth

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	"github.com/jackc/pgx/v5"
	"golang.org/x/crypto/bcrypt"
)

func (s *Service) Login(ctx context.Context, email, password, inviteCode string, opts LoginOptions) (AuthResult, string, error) {
	email = strings.TrimSpace(strings.ToLower(email))
	inviteCode = strings.TrimSpace(inviteCode)

	if !ValidateEmail(email) {
		return AuthResult{}, "invalid_email", nil
	}
	if password == "" {
		return AuthResult{}, "invalid_password", nil
	}

	var userID, dbEmail, role, passHash string
	var emailVerifiedAt sql.NullTime
	err := authDBQueryRowFn(ctx, s.db,
		`SELECT id::text, email, role, password_hash, email_verified_at
		FROM users
		WHERE email = $1`,
		email,
	).Scan(&userID, &dbEmail, &role, &passHash, &emailVerifiedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return AuthResult{}, "invalid_credentials", nil
		}
		return AuthResult{}, "", err
	}

	if err := bcrypt.CompareHashAndPassword([]byte(passHash), []byte(password)); err != nil {
		return AuthResult{}, "invalid_credentials", nil
	}

	tx, err := authBeginTxFn(ctx, s.db)
	if err != nil {
		return AuthResult{}, "", err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var lockedUserID string
	err = tx.QueryRow(ctx,
		`SELECT id::text
		 FROM users
		 WHERE id = $1::uuid
		 FOR UPDATE`,
		userID,
	).Scan(&lockedUserID)
	if err != nil {
		return AuthResult{}, "", err
	}

	var activeSessionID string
	err = tx.QueryRow(ctx,
		`SELECT id::text
		 FROM auth_sessions
		 WHERE user_id = $1::uuid
		   AND revoked_at IS NULL
		   AND expires_at > now()
		 ORDER BY last_used_at DESC NULLS LAST, created_at DESC
		 LIMIT 1
		 FOR UPDATE`,
		userID,
	).Scan(&activeSessionID)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return AuthResult{}, "", err
	}

	if activeSessionID != "" && !opts.ReplaceExistingSession {
		return AuthResult{}, "active_session_exists", nil
	}

	if activeSessionID != "" && opts.ReplaceExistingSession {
		_, err = tx.Exec(ctx,
			`UPDATE auth_sessions
			 SET revoked_at = COALESCE(revoked_at, now()),
			     revoked_reason = COALESCE(revoked_reason, 'replaced_by_new_login')
			 WHERE user_id = $1::uuid
			   AND revoked_at IS NULL`,
			userID,
		)
		if err != nil {
			return AuthResult{}, "", err
		}

		_, err = tx.Exec(ctx,
			`DELETE FROM api_tokens
			 WHERE user_id = $1::uuid`,
			userID,
		)
		if err != nil {
			return AuthResult{}, "", err
		}
	}

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

	pair, err := s.issueTokenPair(ctx, tx, userID, opts.UserAgent, opts.IPAddress)
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
	if opts.ReplaceExistingSession && activeSessionID != "" {
		loginDetails["replaced_session_id"] = activeSessionID
	}
	_ = s.insertAudit(ctx, tx, &userID, "user_logged_in", loginDetails)

	if err := tx.Commit(ctx); err != nil {
		return AuthResult{}, "", err
	}

	emailVerified, emailVerifiedAtText := emailVerificationFields(emailVerifiedAt)

	return authResultFromPair(User{
		ID:              userID,
		Email:           dbEmail,
		Role:            role,
		EmailVerified:   emailVerified,
		EmailVerifiedAt: emailVerifiedAtText,
	}, pair), "", nil
}
