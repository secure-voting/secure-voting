package auth

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

func (s *Service) ensureAdmin(ctx context.Context, userID string) (string, error) {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return "unauthorized", nil
	}

	var role string
	err := authDBQueryRowFn(ctx, s.db, `
		SELECT role
		FROM users
		WHERE id = $1::uuid
	`, userID).Scan(&role)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "unauthorized", nil
		}
		return "", err
	}

	if strings.TrimSpace(strings.ToLower(role)) != "admin" {
		return "forbidden", nil
	}

	return "", nil
}

func (s *Service) ListUsers(ctx context.Context, actorUserID string, limit, offset int) ([]AdminUser, string, error) {
	code, err := s.ensureAdmin(ctx, actorUserID)
	if err != nil || code != "" {
		return nil, code, err
	}

	if limit <= 0 || limit > 200 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}

	rows, err := s.db.Query(ctx, `
		SELECT id::text, email, role, full_name, phone, created_at
		FROM users
		ORDER BY created_at DESC, id DESC
		LIMIT $1 OFFSET $2
	`, limit, offset)
	if err != nil {
		return nil, "", err
	}
	defer rows.Close()

	items := make([]AdminUser, 0, 32)
	for rows.Next() {
		var item AdminUser
		var fullName *string
		var phone *string
		var createdAt time.Time

		if err := rows.Scan(
			&item.ID,
			&item.Email,
			&item.Role,
			&fullName,
			&phone,
			&createdAt,
		); err != nil {
			return nil, "", err
		}

		item.FullName = normalizeOptionalStringPtr(fullName)
		item.Phone = normalizeOptionalStringPtr(phone)
		item.CreatedAt = createdAt.UTC().Format(time.RFC3339)

		items = append(items, item)
	}

	if err := rows.Err(); err != nil {
		return nil, "", err
	}

	return items, "", nil
}

func (s *Service) UpdateUserRole(ctx context.Context, actorUserID, targetUserID, newRole string) (AdminUser, string, error) {
	actorUserID = strings.TrimSpace(actorUserID)
	targetUserID = strings.TrimSpace(targetUserID)
	newRole = strings.TrimSpace(strings.ToLower(newRole))

	code, err := s.ensureAdmin(ctx, actorUserID)
	if err != nil || code != "" {
		return AdminUser{}, code, err
	}

	if targetUserID == "" {
		return AdminUser{}, "invalid_id", nil
	}
	if !ValidateRole(newRole) {
		return AdminUser{}, "invalid_role", nil
	}
	if actorUserID == targetUserID {
		return AdminUser{}, "self_role_change_forbidden", nil
	}

	tx, err := authBeginTxFn(ctx, s.db)
	if err != nil {
		return AdminUser{}, "", err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var before AdminUser
	var beforeFullName *string
	var beforePhone *string
	var beforeCreatedAt time.Time

	err = tx.QueryRow(ctx, `
		SELECT id::text, email, role, full_name, phone, created_at
		FROM users
		WHERE id = $1::uuid
	`, targetUserID).Scan(
		&before.ID,
		&before.Email,
		&before.Role,
		&beforeFullName,
		&beforePhone,
		&beforeCreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return AdminUser{}, "not_found", nil
		}
		return AdminUser{}, "", err
	}

	before.FullName = normalizeOptionalStringPtr(beforeFullName)
	before.Phone = normalizeOptionalStringPtr(beforePhone)
	before.CreatedAt = beforeCreatedAt.UTC().Format(time.RFC3339)

	var after AdminUser
	var afterFullName *string
	var afterPhone *string
	var afterCreatedAt time.Time

	err = tx.QueryRow(ctx, `
		UPDATE users
		SET role = $2
		WHERE id = $1::uuid
		RETURNING id::text, email, role, full_name, phone, created_at
	`, targetUserID, newRole).Scan(
		&after.ID,
		&after.Email,
		&after.Role,
		&afterFullName,
		&afterPhone,
		&afterCreatedAt,
	)
	if err != nil {
		return AdminUser{}, "", err
	}

	after.FullName = normalizeOptionalStringPtr(afterFullName)
	after.Phone = normalizeOptionalStringPtr(afterPhone)
	after.CreatedAt = afterCreatedAt.UTC().Format(time.RFC3339)

	_ = s.insertAudit(ctx, tx, &actorUserID, "user_role_updated", map[string]any{
		"target_type": "user",
		"target_id":   targetUserID,
		"before": map[string]any{
			"role": before.Role,
		},
		"after": map[string]any{
			"role": after.Role,
		},
	})

	if err := tx.Commit(ctx); err != nil {
		return AdminUser{}, "", err
	}

	return after, "", nil
}
