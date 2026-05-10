package auth

import (
	"context"
	"errors"
	"strings"

	"github.com/jackc/pgx/v5"
	"golang.org/x/crypto/bcrypt"
)

func (s *Service) ChangePassword(ctx context.Context, userID, currentPassword, newPassword string) (string, error) {
	userID = strings.TrimSpace(userID)
	currentPassword = strings.TrimSpace(currentPassword)
	newPassword = strings.TrimSpace(newPassword)

	if userID == "" {
		return "unauthorized", nil
	}
	if currentPassword == "" {
		return "invalid_current_password", nil
	}
	if !ValidatePassword(newPassword) {
		return "invalid_password", nil
	}

	tx, err := authBeginTxFn(ctx, s.db)
	if err != nil {
		return "", err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var currentHash string
	err = tx.QueryRow(
		ctx,
		`SELECT password_hash
		 FROM users
		 WHERE id = $1::uuid`,
		userID,
	).Scan(&currentHash)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "unauthorized", nil
		}
		return "", err
	}

	if err := bcrypt.CompareHashAndPassword([]byte(currentHash), []byte(currentPassword)); err != nil {
		return "invalid_current_password", nil
	}

	if err := bcrypt.CompareHashAndPassword([]byte(currentHash), []byte(newPassword)); err == nil {
		return "password_unchanged", nil
	}

	newHashBytes, err := bcrypt.GenerateFromPassword([]byte(newPassword), 12)
	if err != nil {
		return "", err
	}

	_, err = tx.Exec(
		ctx,
		`UPDATE users
		 SET password_hash = $2
		 WHERE id = $1::uuid`,
		userID,
		string(newHashBytes),
	)
	if err != nil {
		return "", err
	}

	_ = s.insertAudit(ctx, tx, &userID, "user_password_changed", map[string]any{
		"target_type": "user",
		"target_id":   userID,
	})

	if err := tx.Commit(ctx); err != nil {
		return "", err
	}

	return "", nil
}
