package auth

import (
	"context"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"
)

func EnsureBootstrapUser(ctx context.Context, db *pgxpool.Pool, email, password, role string) error {
	email = strings.TrimSpace(strings.ToLower(email))
	password = strings.TrimSpace(password)
	role = strings.TrimSpace(strings.ToLower(role))

	if email == "" && password == "" {
		return nil
	}
	if email == "" || password == "" {
		return nil
	}

	if !ValidateEmail(email) {
		return nil
	}
	if !ValidatePassword(password) {
		return nil
	}

	switch role {
	case "admin", "researcher":
	default:
		return nil
	}

	hashBytes, err := bcrypt.GenerateFromPassword([]byte(password), 12)
	if err != nil {
		return err
	}

	_, err = db.Exec(ctx, `
		INSERT INTO users (email, password_hash, role)
		VALUES ($1, $2, $3)
		ON CONFLICT (email)
		DO UPDATE SET
			password_hash = EXCLUDED.password_hash,
			role = EXCLUDED.role
	`, email, string(hashBytes), role)

	return err
}