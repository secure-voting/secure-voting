package auth

import (
	"context"
	"database/sql"
	"encoding/hex"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

type issuedTokenPair struct {
	SessionID        string
	AccessToken     string
	AccessExpiresAt time.Time
	RefreshToken    string
	RefreshExpiresAt time.Time
}

func randomOpaqueToken() (string, error) {
	b := make([]byte, 32)
	if _, err := randReadFn(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func nullStringValue(v string) any {
	v = strings.TrimSpace(v)
	if v == "" {
		return nil
	}
	return v
}

func (s *Service) issueToken(ctx context.Context, tx txLike, userID string) (token string, tokenHashHex string, expiresAt time.Time, err error) {
	return s.issueAccessToken(ctx, tx, userID, nil)
}

func (s *Service) issueAccessToken(ctx context.Context, tx txLike, userID string, sessionID *string) (token string, tokenHashHex string, expiresAt time.Time, err error) {
	token, err = randomOpaqueToken()
	if err != nil {
		return "", "", time.Time{}, err
	}

	tokenHashHex = sha256Hex(token)
	expiresAt = nowFn().UTC().Add(s.tokenTTL)

	var sessionArg any
	if sessionID != nil && strings.TrimSpace(*sessionID) != "" {
		sessionArg = strings.TrimSpace(*sessionID)
	}

	scopes := []string{}

	_, err = tx.Exec(ctx,
		`INSERT INTO api_tokens (user_id, session_id, token_hash, scopes, expires_at)
		 VALUES ($1::uuid, $2::uuid, $3, $4, $5)`,
		userID, sessionArg, tokenHashHex, scopes, expiresAt,
	)
	if err != nil {
		return "", "", time.Time{}, err
	}

	return token, tokenHashHex, expiresAt, nil
}

func (s *Service) issueTokenPair(ctx context.Context, tx txLike, userID, userAgent, ipAddress string) (issuedTokenPair, error) {
	refreshToken, err := randomOpaqueToken()
	if err != nil {
		return issuedTokenPair{}, err
	}

	refreshTokenHash := sha256Hex(refreshToken)
	refreshExpiresAt := nowFn().UTC().Add(s.refreshTokenTTL)

	var sessionID string
	err = tx.QueryRow(ctx,
		`INSERT INTO auth_sessions (user_id, refresh_token_hash, user_agent, ip_address, expires_at)
		 VALUES ($1::uuid, $2, $3, $4, $5)
		 RETURNING id::text`,
		userID,
		refreshTokenHash,
		nullStringValue(userAgent),
		nullStringValue(ipAddress),
		refreshExpiresAt,
	).Scan(&sessionID)
	if err != nil {
		return issuedTokenPair{}, err
	}

	accessToken, _, accessExpiresAt, err := s.issueAccessToken(ctx, tx, userID, &sessionID)
	if err != nil {
		return issuedTokenPair{}, err
	}

	return issuedTokenPair{
		SessionID:         sessionID,
		AccessToken:       accessToken,
		AccessExpiresAt:   accessExpiresAt,
		RefreshToken:      refreshToken,
		RefreshExpiresAt:  refreshExpiresAt,
	}, nil
}

func authResultFromPair(user User, pair issuedTokenPair) AuthResult {
	return AuthResult{
		AccessToken:      pair.AccessToken,
		ExpiresAt:        pair.AccessExpiresAt.UTC().Format(time.RFC3339),
		RefreshToken:     pair.RefreshToken,
		RefreshExpiresAt: pair.RefreshExpiresAt.UTC().Format(time.RFC3339),
		User:             user,
	}
}

func (s *Service) Refresh(ctx context.Context, rawRefreshToken string) (AuthResult, string, error) {
	rawRefreshToken = strings.TrimSpace(rawRefreshToken)
	if rawRefreshToken == "" {
		return AuthResult{}, "invalid_refresh_token", nil
	}

	refreshHash := sha256Hex(rawRefreshToken)

	tx, err := authBeginTxFn(ctx, s.db)
	if err != nil {
		return AuthResult{}, "", err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var sessionID string
	var userID string
	var email string
	var role string
	var sessionExpiresAt time.Time
	var revokedAt sql.NullTime

	err = tx.QueryRow(ctx,
		`SELECT s.id::text, s.user_id::text, u.email, u.role, s.expires_at, s.revoked_at
		 FROM auth_sessions s
		 JOIN users u ON u.id = s.user_id
		 WHERE s.refresh_token_hash = $1
		 FOR UPDATE OF s`,
		refreshHash,
	).Scan(&sessionID, &userID, &email, &role, &sessionExpiresAt, &revokedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return AuthResult{}, "invalid_refresh_token", nil
		}
		return AuthResult{}, "", err
	}

	if revokedAt.Valid || !sessionExpiresAt.After(nowFn().UTC()) {
		return AuthResult{}, "invalid_refresh_token", nil
	}

	nextRefreshToken, err := randomOpaqueToken()
	if err != nil {
		return AuthResult{}, "", err
	}

	nextRefreshHash := sha256Hex(nextRefreshToken)
	nextRefreshExpiresAt := nowFn().UTC().Add(s.refreshTokenTTL)

	_, err = tx.Exec(ctx,
		`UPDATE auth_sessions
		 SET refresh_token_hash = $2,
		     last_used_at = now(),
		     expires_at = $3
		 WHERE id = $1::uuid
		   AND revoked_at IS NULL`,
		sessionID,
		nextRefreshHash,
		nextRefreshExpiresAt,
	)
	if err != nil {
		return AuthResult{}, "", err
	}

	_, err = tx.Exec(ctx,
		`DELETE FROM api_tokens
		 WHERE session_id = $1::uuid`,
		sessionID,
	)
	if err != nil {
		return AuthResult{}, "", err
	}

	accessToken, _, accessExpiresAt, err := s.issueAccessToken(ctx, tx, userID, &sessionID)
	if err != nil {
		return AuthResult{}, "", err
	}

	_ = s.insertAudit(ctx, tx, &userID, "auth_token_refreshed", map[string]any{
		"target_type": "auth_session",
		"target_id":   sessionID,
	})

	if err := tx.Commit(ctx); err != nil {
		return AuthResult{}, "", err
	}

	return AuthResult{
		AccessToken:      accessToken,
		ExpiresAt:        accessExpiresAt.UTC().Format(time.RFC3339),
		RefreshToken:     nextRefreshToken,
		RefreshExpiresAt: nextRefreshExpiresAt.UTC().Format(time.RFC3339),
		User: User{
			ID:    userID,
			Email: email,
			Role:  role,
		},
	}, "", nil
}