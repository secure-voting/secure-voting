package auth

import (
	"context"
	"crypto/subtle"
	"database/sql"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

const (
	emailVerificationTTL         = 15 * time.Minute
	emailVerificationMaxAttempts = 5
	emailVerificationCodeLength  = 16
)

const emailVerificationAlphabet = "23456789ABCDEFGHJKLMNPQRSTUVWXYZ"

func randomEmailVerificationCode() (string, error) {
	b := make([]byte, emailVerificationCodeLength)
	if _, err := randReadFn(b); err != nil {
		return "", err
	}

	out := make([]byte, emailVerificationCodeLength)
	for i, v := range b {
		out[i] = emailVerificationAlphabet[int(v)%len(emailVerificationAlphabet)]
	}

	return groupVerificationCode(string(out)), nil
}

func groupVerificationCode(code string) string {
	code = normalizeVerificationCode(code)
	if code == "" {
		return ""
	}

	var b strings.Builder
	for i, r := range code {
		if i > 0 && i%4 == 0 {
			b.WriteByte('-')
		}
		b.WriteRune(r)
	}
	return b.String()
}

func normalizeVerificationCode(code string) string {
	code = strings.TrimSpace(code)
	if code == "" {
		return ""
	}

	var b strings.Builder
	for _, r := range code {
		switch {
		case r == '-' || r == ' ' || r == '\t' || r == '\n' || r == '\r':
			continue
		default:
			b.WriteRune(r)
		}
	}

	return strings.ToUpper(strings.TrimSpace(b.String()))
}

func verificationCodeHash(code string) string {
	return sha256Hex(normalizeVerificationCode(code))
}

func verificationCodeMatches(storedHash, rawCode string) bool {
	got := verificationCodeHash(rawCode)
	if len(storedHash) != len(got) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(storedHash), []byte(got)) == 1
}

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

	code, err := randomEmailVerificationCode()
	if err != nil {
		return EmailVerificationRequestResult{}, "", err
	}

	codeHash := verificationCodeHash(code)
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
		INSERT INTO email_verification_tokens (
			user_id,
			token_hash,
			attempts_count,
			max_attempts,
			expires_at
		)
		VALUES ($1::uuid, $2, 0, $3, $4)
	`, userID, codeHash, emailVerificationMaxAttempts, expiresAt)
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
		OK:               true,
		AlreadyVerified:  false,
		Delivery:         "dev",
		ExpiresAt:        expiresAt.Format(time.RFC3339),
		MaxAttempts:      emailVerificationMaxAttempts,
		VerificationCode: code,
	}, "", nil
}

func (s *Service) ConfirmEmailVerification(ctx context.Context, userID, rawCode string) (User, string, error) {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return User{}, "unauthorized", nil
	}

	rawCode = normalizeVerificationCode(rawCode)
	if rawCode == "" {
		return User{}, "invalid_verification_code", nil
	}

	tx, err := authBeginTxFn(ctx, s.db)
	if err != nil {
		return User{}, "", err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var tokenID string
	var storedHash string
	var email string
	var role string
	var fullName *string
	var phone *string
	var expiresAt time.Time
	var attemptsCount int
	var maxAttempts int
	var emailVerifiedAt sql.NullTime

	err = tx.QueryRow(ctx, `
		SELECT
			t.id::text,
			t.token_hash,
			u.email,
			u.role,
			u.full_name,
			u.phone,
			t.expires_at,
			t.attempts_count,
			t.max_attempts,
			u.email_verified_at
		FROM email_verification_tokens t
		JOIN users u ON u.id = t.user_id
		WHERE t.user_id = $1::uuid
		  AND t.used_at IS NULL
		ORDER BY t.created_at DESC
		LIMIT 1
		FOR UPDATE OF t, u
	`, userID).Scan(
		&tokenID,
		&storedHash,
		&email,
		&role,
		&fullName,
		&phone,
		&expiresAt,
		&attemptsCount,
		&maxAttempts,
		&emailVerifiedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return User{}, "invalid_verification_code", nil
		}
		return User{}, "", err
	}

	now := nowFn().UTC()

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

		return User{}, "verification_code_expired", nil
	}

	if attemptsCount >= maxAttempts {
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

		return User{}, "verification_attempts_exceeded", nil
	}

	if !verificationCodeMatches(storedHash, rawCode) {
		nextAttempts := attemptsCount + 1

		if nextAttempts >= maxAttempts {
			_, err = tx.Exec(ctx, `
				UPDATE email_verification_tokens
				SET attempts_count = attempts_count + 1,
				    used_at = now()
				WHERE id = $1::uuid
			`, tokenID)
			if err != nil {
				return User{}, "", err
			}

			if err := tx.Commit(ctx); err != nil {
				return User{}, "", err
			}

			return User{}, "verification_attempts_exceeded", nil
		}

		_, err = tx.Exec(ctx, `
			UPDATE email_verification_tokens
			SET attempts_count = attempts_count + 1
			WHERE id = $1::uuid
		`, tokenID)
		if err != nil {
			return User{}, "", err
		}

		if err := tx.Commit(ctx); err != nil {
			return User{}, "", err
		}

		return User{}, "invalid_verification_code", nil
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
