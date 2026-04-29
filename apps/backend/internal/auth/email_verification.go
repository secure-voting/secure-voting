package auth

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

const emailVerificationTTL = 24 * time.Hour

func (s *Service) RequestEmailVerification(ctx context.Context, userID string) (EmailVerificationRequestResult, string, error) {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return EmailVerificationRequestResult{}, "unauthorized", nil
	}

	tx, err := authBeginTxFn(ctx, s.db)
	if err != nil {
		return EmailVerificationRequestResult{}, "", err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var emailVerifiedAt sql.NullTime
	err = tx.QueryRow(ctx, `
		SELECT email_verified_at
		FROM users
		WHERE id = $1::uuid
		FOR UPDATE
	`, userID).Scan(&emailVerifiedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return EmailVerificationRequestResult{}, "unauthorized", nil
		}
		return EmailVerificationRequestResult{}, "", err
	}

	if emailVerifiedAt.Valid {
		if err := tx.Commit(ctx); err != nil {
			return EmailVerificationRequestResult{}, "", err
		}

		return EmailVerificationRequestResult{
			OK:              true,
			AlreadyVerified: true,
		}, "", nil
	}

	token, err := randomOpaqueToken()
	if err != nil {
		return EmailVerificationRequestResult{}, "", err
	}

	tokenHash := sha256Hex(token)
	expiresAt := nowFn().UTC().Add(emailVerificationTTL)

	_, err = tx.Exec(ctx, `
		UPDATE email_verification_tokens
		SET used_at = now()
		WHERE user_id = $1::uuid
		  AND used_at IS NULL
	`, userID)
	if err != nil {
		return EmailVerificationRequestResult{}, "", err
	}

	_, err = tx.Exec(ctx, `
		INSERT INTO email_verification_tokens (user_id, token_hash, expires_at)
		VALUES ($1::uuid, $2, $3)
	`, userID, tokenHash, expiresAt)
	if err != nil {
		return EmailVerificationRequestResult{}, "", err
	}

	if err := s.insertAudit(ctx, tx, &userID, "email_verification_requested", map[string]any{
		"target_type": "user",
		"target_id":   userID,
	}); err != nil {
		return EmailVerificationRequestResult{}, "", err
	}

	if err := tx.Commit(ctx); err != nil {
		return EmailVerificationRequestResult{}, "", err
	}

	return EmailVerificationRequestResult{
		OK:                true,
		AlreadyVerified:   false,
		ExpiresAt:         expiresAt.Format(time.RFC3339),
		VerificationToken: token,
		VerificationURL:   "/verify-email?token=" + token,
	}, "", nil
}

func (s *Service) ConfirmEmailVerification(ctx context.Context, rawToken string) (User, string, error) {
	rawToken = strings.TrimSpace(rawToken)
	if rawToken == "" {
		return User{}, "invalid_verification_token", nil
	}

	tokenHash := sha256Hex(rawToken)

	tx, err := authBeginTxFn(ctx, s.db)
	if err != nil {
		return User{}, "", err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var tokenID string
	var userID string
	var email string
	var role string
	var fullName *string
	var phone *string
	var expiresAt time.Time
	var usedAt sql.NullTime
	var emailVerifiedAt sql.NullTime

	err = tx.QueryRow(ctx, `
		SELECT
			t.id::text,
			u.id::text,
			u.email,
			u.role,
			u.full_name,
			u.phone,
			t.expires_at,
			t.used_at,
			u.email_verified_at
		FROM email_verification_tokens t
		JOIN users u ON u.id = t.user_id
		WHERE t.token_hash = $1
		FOR UPDATE OF t, u
	`, tokenHash).Scan(
		&tokenID,
		&userID,
		&email,
		&role,
		&fullName,
		&phone,
		&expiresAt,
		&usedAt,
		&emailVerifiedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return User{}, "invalid_verification_token", nil
		}
		return User{}, "", err
	}

	now := nowFn().UTC()

	if usedAt.Valid {
		return User{}, "verification_token_used", nil
	}

	if !expiresAt.After(now) {
		_, err = tx.Exec(ctx, `
			UPDATE email_verification_tokens
			SET used_at = now()
			WHERE id = $1::uuid
		`, tokenID)
		if err != nil {
			return User{}, "", err
		}

		if err := tx.Commit(ctx); err != nil {
			return User{}, "", err
		}

		return User{}, "verification_token_expired", nil
	}

	if !emailVerifiedAt.Valid {
		err = tx.QueryRow(ctx, `
			UPDATE users
			SET email_verified_at = now()
			WHERE id = $1::uuid
			RETURNING email_verified_at
		`, userID).Scan(&emailVerifiedAt)
		if err != nil {
			return User{}, "", err
		}
	}

	_, err = tx.Exec(ctx, `
		UPDATE email_verification_tokens
		SET used_at = now()
		WHERE id = $1::uuid
	`, tokenID)
	if err != nil {
		return User{}, "", err
	}

	if err := s.insertAudit(ctx, tx, &userID, "email_verified", map[string]any{
		"target_type": "user",
		"target_id":   userID,
	}); err != nil {
		return User{}, "", err
	}

	if err := tx.Commit(ctx); err != nil {
		return User{}, "", err
	}

	emailVerified, emailVerifiedAtText := emailVerificationFields(emailVerifiedAt)

	return User{
		ID:              userID,
		Email:           email,
		Role:            role,
		FullName:        normalizeOptionalStringPtr(fullName),
		Phone:           normalizeOptionalStringPtr(phone),
		EmailVerified:   emailVerified,
		EmailVerifiedAt: emailVerifiedAtText,
	}, "", nil
}
