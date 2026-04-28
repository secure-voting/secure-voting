package auth

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	"github.com/jackc/pgx/v5"
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

	var sessionID sql.NullString
	var deletedUserID string

	err = tx.QueryRow(ctx,
		`DELETE FROM api_tokens
		 WHERE token_hash = $1
		 RETURNING session_id::text, user_id::text`,
		tokenHashHex,
	).Scan(&sessionID, &deletedUserID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return false, nil
		}
		return false, err
	}

	if sessionID.Valid && strings.TrimSpace(sessionID.String) != "" {
		_, err = tx.Exec(ctx,
			`UPDATE auth_sessions
			 SET revoked_at = COALESCE(revoked_at, now()),
			     revoked_reason = COALESCE(revoked_reason, 'logout')
			 WHERE id = $1::uuid`,
			sessionID.String,
		)
		if err != nil {
			return false, err
		}
	}

	actor := actorUserID
	if actor == nil && strings.TrimSpace(deletedUserID) != "" {
		actor = &deletedUserID
	}

	_ = s.insertAudit(ctx, tx, actor, "user_logged_out", map[string]any{
		"target_type": "auth_session",
		"target_id":   sessionID.String,
	})

	if err := tx.Commit(ctx); err != nil {
		return false, err
	}

	return true, nil
}