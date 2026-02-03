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

type RegisterInput struct {
	Email      string
	Password   string
	Role       string
	InviteCode string
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

type acceptedInvite struct {
	ID         string
	ElectionID string
}

func (s *Service) acceptInviteTx(ctx context.Context, tx pgx.Tx, email, inviteCode string) (acceptedInvite, string, error) {
	inviteCode = strings.TrimSpace(inviteCode)
	if inviteCode == "" {
		return acceptedInvite{}, "", nil
	}

	inviteHashHex := sha256Hex(inviteCode)

	var inviteID, inviteEmail, inviteStatus, inviteElectionID string
	err := tx.QueryRow(ctx, `
		SELECT id::text, email, status, election_id::text
		FROM election_invites
		WHERE invite_code_hash = $1
		LIMIT 1
	`, inviteHashHex).Scan(&inviteID, &inviteEmail, &inviteStatus, &inviteElectionID)

	if errors.Is(err, pgx.ErrNoRows) {
		return acceptedInvite{}, "invalid_invite_code", nil
	}
	if err != nil {
		return acceptedInvite{}, "", err
	}

	if inviteStatus != "created" && inviteStatus != "sent" {
		return acceptedInvite{}, "invite_code_inactive", nil
	}

	if strings.TrimSpace(inviteEmail) != "" && strings.ToLower(strings.TrimSpace(inviteEmail)) != email {
		return acceptedInvite{}, "invite_email_mismatch", nil
	}

	ct, err := tx.Exec(ctx, `
		UPDATE election_invites
		SET status = 'accepted', accepted_at = now()
		WHERE id = $1::uuid AND status IN ('created','sent')
	`, inviteID)
	if err != nil {
		return acceptedInvite{}, "", err
	}
	if ct.RowsAffected() == 0 {
		return acceptedInvite{}, "invite_code_inactive", nil
	}

	return acceptedInvite{ID: inviteID, ElectionID: inviteElectionID}, "", nil
}

func (s *Service) Register(ctx context.Context, email, password, role, inviteCode string) (AuthResult, string, error) {
	email = strings.TrimSpace(strings.ToLower(email))
	inviteCode = strings.TrimSpace(inviteCode)
	role = strings.TrimSpace(strings.ToLower(role))

	if !ValidateEmail(email) {
		return AuthResult{}, "invalid_email", nil
	}
	if !ValidatePassword(password) {
		return AuthResult{}, "invalid_password", nil
	}

	if role == "" {
		role = "voter"
	}

	if inviteCode != "" {
		role = "voter"
	}

	switch role {
	case "admin", "voter", "researcher":
	default:
		return AuthResult{}, "invalid_role", nil
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
	err = tx.QueryRow(ctx,
		`INSERT INTO users (email, password_hash, role)
		 VALUES ($1, $2, $3)
		 RETURNING id::text`,
		email, passHash, role,
	).Scan(&userID)

	if err != nil {
		le := strings.ToLower(err.Error())
		if strings.Contains(le, "duplicate") || strings.Contains(le, "unique") {
			return AuthResult{}, "email_taken", nil
		}
		return AuthResult{}, "", err
	}

	token, _, expiresAt, err := s.issueToken(ctx, tx, userID)
	if err != nil {
		return AuthResult{}, "", err
	}

	details := map[string]any{
		"target_type": "user",
		"target_id":   userID,
		"after": map[string]any{
			"email": email,
			"role":  role,
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

func (s *Service) Login(ctx context.Context, email, password, inviteCode string) (AuthResult, string, error) {
	email = strings.TrimSpace(strings.ToLower(email))
	inviteCode = strings.TrimSpace(inviteCode)

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

		_ = s.insertAudit(ctx, tx, &userID, "invite_accepted", map[string]any{
			"target_type": "election_invite",
			"target_id":   inv.ID,
			"details": map[string]any{
				"election_id": inv.ElectionID,
				"email":       email,
			},
		})
	}

	token, _, expiresAt, err := s.issueToken(ctx, tx, userID)
	if err != nil {
		return AuthResult{}, "", err
	}

	loginDetails := map[string]any{
		"target_type": "user",
		"target_id":   userID,
	}
	if inviteCode != "" {
		loginDetails["invite"] = map[string]any{
			"id":          inv.ID,
			"election_id": inv.ElectionID,
		}
	}
	_ = s.insertAudit(ctx, tx, &userID, "user_logged_in", loginDetails)

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

func (s *Service) VerifyAccessToken(ctx context.Context, rawToken string) (userID, email, role string, ok bool, err error) {
	rawToken = strings.TrimSpace(rawToken)
	if rawToken == "" {
		return "", "", "", false, nil
	}
	tokenHashHex := sha256Hex(rawToken)

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
	tokenHashHex := sha256Hex(rawToken)

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

func sha256Hex(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}
