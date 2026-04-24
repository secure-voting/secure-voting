package notifications

import (
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Service struct {
	db *pgxpool.Pool
}

func NewService(db *pgxpool.Pool) *Service {
	return &Service{db: db}
}

type Kind string

const (
	KindInfo    Kind = "info"
	KindSuccess Kind = "success"
	KindWarning Kind = "warning"
	KindError   Kind = "error"
)

type Item struct {
	ID          string  `json:"id"`
	Title       string  `json:"title"`
	Message     string  `json:"message"`
	Details     *string `json:"details,omitempty"`
	ActionLabel *string `json:"action_label,omitempty"`
	ActionTo    *string `json:"action_to,omitempty"`
	Kind        string  `json:"kind"`
	CreatedAt   string  `json:"created_at"`
	Read        bool    `json:"read"`
}

type CreateInput struct {
	UserID      string
	Title       string
	Message     string
	Details     string
	ActionLabel string
	ActionTo    string
	Kind        string
}

func normalizeOptionalStringPtr(v *string) *string {
	if v == nil {
		return nil
	}
	s := strings.TrimSpace(*v)
	if s == "" {
		return nil
	}
	return &s
}

func normalizeOptionalValue(v string) any {
	s := strings.TrimSpace(v)
	if s == "" {
		return nil
	}
	return s
}

func normalizeKind(v string) string {
	v = strings.ToLower(strings.TrimSpace(v))
	switch v {
	case string(KindInfo), string(KindSuccess), string(KindWarning), string(KindError):
		return v
	default:
		return string(KindInfo)
	}
}

func validateTitle(v string) bool {
	v = strings.TrimSpace(v)
	return v != "" && len(v) <= 200
}

func validateMessage(v string) bool {
	v = strings.TrimSpace(v)
	return v != "" && len(v) <= 4000
}

func validateDetails(v string) bool {
	return len(strings.TrimSpace(v)) <= 10000
}

func validateActionLabel(v string) bool {
	return len(strings.TrimSpace(v)) <= 120
}

func validateActionTo(v string) bool {
	return len(strings.TrimSpace(v)) <= 500
}

func formatTime(t time.Time) string {
	return t.UTC().Format(time.RFC3339)
}
