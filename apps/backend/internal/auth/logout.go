package auth

import (
	"context"
	"strings"
)

func (s *Service) Logout(ctx context.Context, rawToken string, actorUserID *string) (bool, error) {
	rawToken = strings.TrimSpace(rawToken)
	if rawToken == "" {
		return false, nil
	}
	tokenHashHex := sha256Hex(rawToken)

	tx, err := authBeginTxFn(ctx, s.db)
	if err != nil {
		return false, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	ct, err := tx.Exec(ctx, `DELETE FROM api_tokens WHERE token_hash = $1`, tokenHashHex)
	if err != nil {
		return false, err
	}

	_ = s.insertAudit(ctx, tx, actorUserID, "user_logged_out", map[string]any{
		"target_type": "api_token",
	})

	if err := tx.Commit(ctx); err != nil {
		return false, err
	}

	return ct.RowsAffected() > 0, nil
}
