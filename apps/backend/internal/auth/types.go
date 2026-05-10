package auth

import (
	"database/sql"
	"net/mail"
	"regexp"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Service struct {
	db              *pgxpool.Pool
	tokenTTL        time.Duration
	refreshTokenTTL time.Duration
	emailVerifier   EmailVerificationSender
}

type EmailVerificationSender interface {
	SendEmailVerificationCode(email, code, expiresAt string) (string, error)
}

func NewService(db *pgxpool.Pool, tokenTTL time.Duration) *Service {
	return NewServiceWithRefreshTTL(db, tokenTTL, 30*24*time.Hour)
}

func (s *Service) WithEmailVerificationSender(sender EmailVerificationSender) *Service {
	s.emailVerifier = sender
	return s
}

func NewServiceWithRefreshTTL(db *pgxpool.Pool, tokenTTL, refreshTokenTTL time.Duration) *Service {
	return &Service{
		db:              db,
		tokenTTL:        tokenTTL,
		refreshTokenTTL: refreshTokenTTL,
	}
}

type User struct {
	ID              string  `json:"id"`
	Email           string  `json:"email"`
	Role            string  `json:"role"`
	FullName        *string `json:"full_name,omitempty"`
	Phone           *string `json:"phone,omitempty"`
	EmailVerified   bool    `json:"email_verified"`
	EmailVerifiedAt *string `json:"email_verified_at,omitempty"`
}

type AuthResult struct {
	AccessToken      string `json:"access_token"`
	ExpiresAt        string `json:"expires_at"`
	RefreshToken     string `json:"refresh_token,omitempty"`
	RefreshExpiresAt string `json:"refresh_expires_at,omitempty"`
	User             User   `json:"user"`
}

type EmailVerificationRequestResult struct {
	OK               bool   `json:"ok"`
	AlreadyVerified  bool   `json:"already_verified"`
	Delivery         string `json:"delivery,omitempty"`
	ExpiresAt        string `json:"expires_at,omitempty"`
	MaxAttempts      int    `json:"max_attempts,omitempty"`
	VerificationCode string `json:"verification_code,omitempty"`
}

type RegisterInput struct {
	Email      string
	Password   string
	Role       string
	InviteCode string
}

type LoginOptions struct {
	ReplaceExistingSession bool
	UserAgent              string
	IPAddress              string
}

func emailVerificationFields(verifiedAt sql.NullTime) (bool, *string) {
	if !verifiedAt.Valid {
		return false, nil
	}

	formatted := verifiedAt.Time.UTC().Format(time.RFC3339)
	return true, &formatted
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

func ValidateFullName(fullName string) bool {
	fullName = strings.TrimSpace(fullName)
	return len(fullName) <= 120
}

var phonePattern = regexp.MustCompile(`^\+?[0-9 ()-]{5,32}$`)

func ValidatePhone(phone string) bool {
	phone = strings.TrimSpace(phone)
	if phone == "" {
		return true
	}
	return phonePattern.MatchString(phone)
}

type acceptedInvite struct {
	ID         string
	ElectionID string
}

type AcceptInviteResult struct {
	OK         bool   `json:"ok"`
	InviteID   string `json:"invite_id"`
	ElectionID string `json:"election_id"`
}

type ChangePasswordInput struct {
	UserID          string
	CurrentPassword string
	NewPassword     string
}

type UpdateProfileInput struct {
	UserID   string
	FullName string
	Phone    string
}

type AdminUser struct {
	ID        string  `json:"id"`
	Email     string  `json:"email"`
	Role      string  `json:"role"`
	FullName  *string `json:"full_name,omitempty"`
	Phone     *string `json:"phone,omitempty"`
	CreatedAt string  `json:"created_at"`
}

func ValidateRole(role string) bool {
	switch strings.TrimSpace(strings.ToLower(role)) {
	case "admin", "researcher", "voter":
		return true
	default:
		return false
	}
}
