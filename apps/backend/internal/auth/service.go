package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/mail"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"
)

type Service struct {
	db       *pgxpool.Pool
	tokenTTL time.Duration
}

func NewService(db *pgxpool.Pool, tokenTTL time.Duration) *Service {
	return &Service{db: db, tokenTTL: tokenTTL}
}

type User struct {
	ID    string `json:"id"`
	Email string `json:"email"`
	Role  string `json:"role"`
}

type AuthResult struct {
	AccessToken string `json:"access_token"`
	ExpiresAt   string `json:"expires_at"`
	User        User   `json:"user"`
}

func ValidateEmail(email string) bool {
	email = strings.TrimSpace(email)
	if email == "" {
		return false
	}
	_, err := mail.ParseAddress(email)
	return err == nil
}

func ValidatePassword(password string) bool {
	return len(password) >= 8
}

func (s *Service) Register(ctx context.Context, email, password string) (AuthResult, string, error) {
	email = strings.TrimSpace(strings.ToLower(email))
	if !ValidateEmail(email) {
		return AuthResult{}, "invalid_email", nil
	}
	if !ValidatePassword(password) {
		return AuthResult{}, "invalid_password", nil
	}

	passHashBytes, err := bcrypt.GenerateFromPassword([]byte(password), 12)
	if err != nil {
		return AuthResult{}, "", err
	}
	passHash := string(passHashBytes)

	tx, err := s.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return AuthResult{}, "", err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	role := "voter"
	var userID string
	err = tx.QueryRow(ctx,
		`INSERT INTO users (email, password_hash, role)
		 VALUES ($1, $2, $3)
		 RETURNING id::text`,
		email, passHash, role,
	).Scan(&userID)
	if err != nil {
		low := strings.ToLower(err.Error())
		if strings.Contains(low, "duplicate") || strings.Contains(low, "unique") {
			return AuthResult{}, "email_taken", nil
		}
		return AuthResult{}, "", err
	}

	token, _, expiresAt, err := s.issueToken(ctx, tx, userID)
	if err != nil {
		return AuthResult{}, "", err
	}

	_ = s.insertAudit(ctx, tx, &userID, "user_registered", map[string]any{
		"target_type": "user",
		"target_id":   userID,
		"after": map[string]any{
			"email": email,
			"role":  role,
		},
	})

	if err := tx.Commit(ctx); err != nil {
		return AuthResult{}, "", err
	}

	return AuthResult{
		AccessToken: token,
		ExpiresAt:   expiresAt.UTC().Format(time.RFC3339),
		User: User{
			ID:    userID,
			Email: email,
			Role:  role,
		},
	}, "", nil
}

func (s *Service) Login(ctx context.Context, email, password string) (AuthResult, string, error) {
	email = strings.TrimSpace(strings.ToLower(email))
	if !ValidateEmail(email) {
		return AuthResult{}, "invalid_email", nil
	}
	if password == "" {
		return AuthResult{}, "invalid_password", nil
	}

	var userID, dbEmail, role, passHash string
	err := s.db.QueryRow(ctx,
		`SELECT id::text, email, role, password_hash
		 FROM users
		 WHERE email = $1`,
		email,
	).Scan(&userID, &dbEmail, &role, &passHash)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return AuthResult{}, "invalid_credentials", nil
		}
		return AuthResult{}, "", err
	}

	if err := bcrypt.CompareHashAndPassword([]byte(passHash), []byte(password)); err != nil {
		return AuthResult{}, "invalid_credentials", nil
	}

	tx, err := s.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return AuthResult{}, "", err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	token, _, expiresAt, err := s.issueToken(ctx, tx, userID)
	if err != nil {
		return AuthResult{}, "", err
	}

	_ = s.insertAudit(ctx, tx, &userID, "user_logged_in", map[string]any{
		"target_type": "user",
		"target_id":   userID,
	})

	if err := tx.Commit(ctx); err != nil {
		return AuthResult{}, "", err
	}

	return AuthResult{
		AccessToken: token,
		ExpiresAt:   expiresAt.UTC().Format(time.RFC3339),
		User: User{
			ID:    userID,
			Email: dbEmail,
			Role:  role,
		},
	}, "", nil
}

// TokenVerifier interface implementation (used by middleware)
func (s *Service) VerifyAccessToken(ctx context.Context, rawToken string) (userID, email, role string, ok bool, err error) {
	rawToken = strings.TrimSpace(rawToken)
	if rawToken == "" {
		return "", "", "", false, nil
	}
	tokenHashHex := hashToken(rawToken)

	var expiresAt time.Time
	err = s.db.QueryRow(ctx,
		`SELECT u.id::text, u.email, u.role, t.expires_at
		 FROM api_tokens t
		 JOIN users u ON u.id = t.user_id
		 WHERE t.token_hash = $1
		   AND t.expires_at > now()
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

func (s *Service) Logout(ctx context.Context, rawToken string, actorUserID *string) (bool, error) {
	rawToken = strings.TrimSpace(rawToken)
	if rawToken == "" {
		return false, nil
	}
	tokenHashHex := hashToken(rawToken)

	tx, err := s.db.BeginTx(ctx, pgx.TxOptions{})
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

func (s *Service) issueToken(ctx context.Context, tx pgx.Tx, userID string) (token string, tokenHashHex string, expiresAt time.Time, err error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", "", time.Time{}, err
	}
	token = hex.EncodeToString(b)
	tokenHashHex = hashToken(token)
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

func (s *Service) insertAudit(ctx context.Context, tx pgx.Tx, actorUserID *string, eventType string, details map[string]any) error {
	if details == nil {
		details = map[string]any{}
	}
	b, err := json.Marshal(details)
	if err != nil {
		return err
	}

	if actorUserID == nil {
		_, err = tx.Exec(ctx,
			`INSERT INTO audit_log (actor_user_id, event_type, details)
			 VALUES (NULL, $1, $2::jsonb)`,
			eventType, string(b),
		)
		return err
	}

	_, err = tx.Exec(ctx,
		`INSERT INTO audit_log (actor_user_id, event_type, details)
		 VALUES ($1::uuid, $2, $3::jsonb)`,
		*actorUserID, eventType, string(b),
	)
	return err
}

func hashToken(raw string) string {
	h := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(h[:])
}
