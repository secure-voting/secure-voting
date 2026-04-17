package adminsettings

import (
	"net/mail"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Service struct {
	db *pgxpool.Pool
}

func NewService(db *pgxpool.Pool) *Service {
	return &Service{db: db}
}

type Settings struct {
	PublicBaseURL       *string `json:"public_base_url,omitempty"`
	TLSMode             string  `json:"tls_mode"`
	TLSDomain           *string `json:"tls_domain,omitempty"`
	TLSContactEmail     *string `json:"tls_contact_email,omitempty"`
	BackupEnabled       bool    `json:"backup_enabled"`
	BackupSchedule      *string `json:"backup_schedule,omitempty"`
	BackupRetentionDays *int    `json:"backup_retention_days,omitempty"`
	DatabaseHost        *string `json:"database_host,omitempty"`
	DatabaseName        *string `json:"database_name,omitempty"`
	UpdatedAt           string  `json:"updated_at"`
	HasUnsavedWarning   bool    `json:"has_unsaved_warning"`
}

type UpdateInput struct {
	ActorUserID         string
	PublicBaseURL       string
	TLSMode             string
	TLSDomain           string
	TLSContactEmail     string
	BackupEnabled       bool
	BackupSchedule      string
	BackupRetentionDays *int
	DatabaseHost        string
	DatabaseName        string
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

func normalizeOptionalStringValue(v string) any {
	s := strings.TrimSpace(v)
	if s == "" {
		return nil
	}
	return s
}

func validTLSMode(v string) bool {
	switch strings.TrimSpace(strings.ToLower(v)) {
	case "disabled", "lets_encrypt", "custom":
		return true
	default:
		return false
	}
}

func validEmailOrEmpty(v string) bool {
	v = strings.TrimSpace(v)
	if v == "" {
		return true
	}
	_, err := mail.ParseAddress(v)
	return err == nil
}
