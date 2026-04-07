package auth

import (
	"net/mail"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
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

type ChangePasswordInput struct {
	UserID          string
	CurrentPassword string
	NewPassword     string
}