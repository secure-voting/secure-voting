package auth

import (
	"context"
	"strings"

	"golang.org/x/crypto/bcrypt"
)

func (s *Service) Register(ctx context.Context, email, password, _ string, inviteCode string) (AuthResult, string, error) {
	email = strings.TrimSpace(strings.ToLower(email))
	inviteCode = strings.TrimSpace(inviteCode)

	if !ValidateEmail(email) {
		return AuthResult{}, "invalid_email", nil
	}
	if !ValidatePassword(password) {
		return AuthResult{}, "invalid_password", nil
	}

	assignedRole := "voter"

	passHashBytes, err := bcrypt.GenerateFromPassword([]byte(password), 12)
	if err != nil {
		return AuthResult{}, "", err
	}
	passHash := string(passHashBytes)

	tx, err := authBeginTxFn(ctx, s.db)
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
	}

	var userID string
	err = tx.QueryRow(
		ctx,
		`INSERT INTO users (email, password_hash, role)
		 VALUES ($1, $2, $3)
		 RETURNING id::text`,
		email, passHash, assignedRole,
	).Scan(&userID)
	if err != nil {
		le := strings.ToLower(err.Error())
		if strings.Contains(le, "duplicate") || strings.Contains(le, "unique") {
			return AuthResult{}, "email_taken", nil
		}
		return AuthResult{}, "", err
	}

	pair, err := s.issueTokenPair(ctx, tx, userID, "", "")
	if err != nil {
		return AuthResult{}, "", err
	}

	details := map[string]any{
		"target_type": "user",
		"target_id":   userID,
		"after": map[string]any{
			"email": email,
			"role":  assignedRole,
		},
	}
	if inviteCode != "" {
		details["invite"] = map[string]any{
			"id":          inv.ID,
			"election_id": inv.ElectionID,
		}
	}

	_ = s.insertAudit(ctx, tx, &userID, "user_registered", details)

	if err := tx.Commit(ctx); err != nil {
		return AuthResult{}, "", err
	}

	return authResultFromPair(User{
		ID:    userID,
		Email: email,
		Role:  assignedRole,
	}, pair), "", nil
}
