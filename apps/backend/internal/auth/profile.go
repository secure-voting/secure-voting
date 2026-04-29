package auth

import (
	"context"
	"errors"
	"strings"
	"database/sql"

	"github.com/jackc/pgx/v5"
)

func normalizeOptionalStringPtr(v *string) *string {
	if v == nil {
		return nil
	}
	s := strings.TrimSpace(*v)
	if s == "" {
		return nil
	}
	return &s
}

func normalizeOptionalStringValue(v string) any {
	s := strings.TrimSpace(v)
	if s == "" {
		return nil
	}
	return s
}

func (s *Service) GetProfile(ctx context.Context, userID string) (User, string, error) {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return User{}, "unauthorized", nil
	}

	var out User
	var fullName *string
	var phone *string
	var emailVerifiedAt sql.NullTime

	err := authDBQueryRowFn(ctx, s.db, `
		SELECT id::text, email, role, full_name, phone, email_verified_at
		FROM users
		WHERE id = $1::uuid
	`, userID).Scan(&out.ID, &out.Email, &out.Role, &fullName, &phone, &emailVerifiedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return User{}, "unauthorized", nil
		}
		return User{}, "", err
	}

	out.FullName = normalizeOptionalStringPtr(fullName)
	out.Phone = normalizeOptionalStringPtr(phone)
	out.EmailVerified, out.EmailVerifiedAt = emailVerificationFields(emailVerifiedAt)
	return out, "", nil
}

func (s *Service) UpdateProfile(ctx context.Context, userID, fullName, phone string) (User, string, error) {
	userID = strings.TrimSpace(userID)
	fullName = strings.TrimSpace(fullName)
	phone = strings.TrimSpace(phone)

	if userID == "" {
		return User{}, "unauthorized", nil
	}
	if !ValidateFullName(fullName) {
		return User{}, "invalid_full_name", nil
	}
	if !ValidatePhone(phone) {
		return User{}, "invalid_phone", nil
	}

	tx, err := authBeginTxFn(ctx, s.db)
	if err != nil {
		return User{}, "", err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var beforeEmail string
	var beforeRole string
	var beforeFullName *string
	var beforePhone *string

	err = tx.QueryRow(ctx, `
		SELECT email, role, full_name, phone
		FROM users
		WHERE id = $1::uuid
	`, userID).Scan(&beforeEmail, &beforeRole, &beforeFullName, &beforePhone)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return User{}, "unauthorized", nil
		}
		return User{}, "", err
	}

	var out User
	var afterFullName *string
	var afterPhone *string
	var afterEmailVerifiedAt sql.NullTime

	err = tx.QueryRow(ctx, `
		UPDATE users
		SET full_name = $2, phone = $3
		WHERE id = $1::uuid
		RETURNING id::text, email, role, full_name, phone, email_verified_at
	`, userID, normalizeOptionalStringValue(fullName), normalizeOptionalStringValue(phone)).
		Scan(&out.ID, &out.Email, &out.Role, &afterFullName, &afterPhone, &afterEmailVerifiedAt)
	if err != nil {
		return User{}, "", err
	}

	out.FullName = normalizeOptionalStringPtr(afterFullName)
	out.Phone = normalizeOptionalStringPtr(afterPhone)
	out.EmailVerified, out.EmailVerifiedAt = emailVerificationFields(afterEmailVerifiedAt)

	_ = s.insertAudit(ctx, tx, &userID, "user_profile_updated", map[string]any{
		"target_type": "user",
		"target_id":   userID,
		"before": map[string]any{
			"full_name": normalizeOptionalStringPtr(beforeFullName),
			"phone":     normalizeOptionalStringPtr(beforePhone),
		},
		"after": map[string]any{
			"full_name": out.FullName,
			"phone":     out.Phone,
		},
	})

	if err := tx.Commit(ctx); err != nil {
		return User{}, "", err
	}

	return out, "", nil
}
