package auth

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

func (s *Service) VerifyAccessToken(ctx context.Context, rawToken string) (userID, email, role string, ok bool, err error) {
	rawToken = strings.TrimSpace(rawToken)
	if rawToken == "" {
		return "", "", "", false, nil
	}
	tokenHashHex := sha256Hex(rawToken)

	var expiresAt time.Time
	err = authDBQueryRowFn(ctx, s.db,
		`SELECT u.id::text, u.email, u.role, t.expires_at
		FROM api_tokens t
		JOIN users u ON u.id = t.user_id
		LEFT JOIN auth_sessions s ON s.id = t.session_id
		WHERE t.token_hash = $1
		  AND t.expires_at > now()
		  AND (
			t.session_id IS NULL
			OR (
			s.revoked_at IS NULL
			AND s.expires_at > now()
			)
		  )
		LIMIT 1`,
		tokenHashHex,
	).Scan(&userID, &email, &role, &expiresAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", "", "", false, nil
		}
		return "", "", "", false, err
	}

	_ = expiresAt
	return userID, email, role, true, nil
}
