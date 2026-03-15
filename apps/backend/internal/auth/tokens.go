package auth

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"time"

	"github.com/jackc/pgx/v5"
)

func (s *Service) issueToken(ctx context.Context, tx pgx.Tx, userID string) (token string, tokenHashHex string, expiresAt time.Time, err error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", "", time.Time{}, err
	}
	token = hex.EncodeToString(b)

	tokenHashHex = sha256Hex(token)
	expiresAt = time.Now().UTC().Add(s.tokenTTL)

	scopes := []string{}

	_, err = tx.Exec(ctx,
		`INSERT INTO api_tokens (user_id, token_hash, scopes, expires_at)
		 VALUES ($1::uuid, $2, $3, $4)`,
		userID, tokenHashHex, scopes, expiresAt,
	)
	if err != nil {
		return "", "", time.Time{}, err
	}
	return token, tokenHashHex, expiresAt, nil
}
