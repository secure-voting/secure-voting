package adminsettings

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

func (s *Service) Get(ctx context.Context) (Settings, error) {
	var out Settings
	var publicBaseURL *string
	var tlsDomain *string
	var tlsContactEmail *string
	var backupSchedule *string
	var backupRetentionDays *int
	var databaseHost *string
	var databaseName *string
	var updatedAt time.Time

	err := s.db.QueryRow(ctx, `
		SELECT
			public_base_url,
			tls_mode,
			tls_domain,
			tls_contact_email,
			backup_enabled,
			backup_schedule,
			backup_retention_days,
			database_host,
			database_name,
			updated_at
		FROM admin_settings
		WHERE id = 1
	`).Scan(
		&publicBaseURL,
		&out.TLSMode,
		&tlsDomain,
		&tlsContactEmail,
		&out.BackupEnabled,
		&backupSchedule,
		&backupRetentionDays,
		&databaseHost,
		&databaseName,
		&updatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Settings{
				TLSMode:           "disabled",
				BackupEnabled:     false,
				UpdatedAt:         "",
				HasUnsavedWarning: true,
			}, nil
		}
		return Settings{}, err
	}

	out.PublicBaseURL = normalizeOptionalStringPtr(publicBaseURL)
	out.TLSDomain = normalizeOptionalStringPtr(tlsDomain)
	out.TLSContactEmail = normalizeOptionalStringPtr(tlsContactEmail)
	out.BackupSchedule = normalizeOptionalStringPtr(backupSchedule)
	out.BackupRetentionDays = backupRetentionDays
	out.DatabaseHost = normalizeOptionalStringPtr(databaseHost)
	out.DatabaseName = normalizeOptionalStringPtr(databaseName)
	out.UpdatedAt = updatedAt.UTC().Format(time.RFC3339)
	out.HasUnsavedWarning = true

	return out, nil
}

func (s *Service) Update(ctx context.Context, in UpdateInput) (Settings, string, error) {
	in.ActorUserID = strings.TrimSpace(in.ActorUserID)
	in.PublicBaseURL = strings.TrimSpace(in.PublicBaseURL)
	in.TLSMode = strings.TrimSpace(strings.ToLower(in.TLSMode))
	in.TLSDomain = strings.TrimSpace(in.TLSDomain)
	in.TLSContactEmail = strings.TrimSpace(in.TLSContactEmail)
	in.BackupSchedule = strings.TrimSpace(in.BackupSchedule)
	in.DatabaseHost = strings.TrimSpace(in.DatabaseHost)
	in.DatabaseName = strings.TrimSpace(in.DatabaseName)

	if in.ActorUserID == "" {
		return Settings{}, "unauthorized", nil
	}
	if !validTLSMode(in.TLSMode) {
		return Settings{}, "invalid_tls_mode", nil
	}
	if !validEmailOrEmpty(in.TLSContactEmail) {
		return Settings{}, "invalid_tls_contact_email", nil
	}
	if in.BackupRetentionDays != nil && *in.BackupRetentionDays <= 0 {
		return Settings{}, "invalid_backup_retention_days", nil
	}

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return Settings{}, "", err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	_, err = tx.Exec(ctx, `
		INSERT INTO admin_settings (
			id,
			public_base_url,
			tls_mode,
			tls_domain,
			tls_contact_email,
			backup_enabled,
			backup_schedule,
			backup_retention_days,
			database_host,
			database_name,
			updated_at,
			updated_by
		)
		VALUES (
			1, $1, $2, $3, $4, $5, $6, $7, $8, $9, now(), $10::uuid
		)
		ON CONFLICT (id) DO UPDATE SET
			public_base_url = EXCLUDED.public_base_url,
			tls_mode = EXCLUDED.tls_mode,
			tls_domain = EXCLUDED.tls_domain,
			tls_contact_email = EXCLUDED.tls_contact_email,
			backup_enabled = EXCLUDED.backup_enabled,
			backup_schedule = EXCLUDED.backup_schedule,
			backup_retention_days = EXCLUDED.backup_retention_days,
			database_host = EXCLUDED.database_host,
			database_name = EXCLUDED.database_name,
			updated_at = now(),
			updated_by = EXCLUDED.updated_by
	`,
		normalizeOptionalStringValue(in.PublicBaseURL),
		in.TLSMode,
		normalizeOptionalStringValue(in.TLSDomain),
		normalizeOptionalStringValue(in.TLSContactEmail),
		in.BackupEnabled,
		normalizeOptionalStringValue(in.BackupSchedule),
		in.BackupRetentionDays,
		normalizeOptionalStringValue(in.DatabaseHost),
		normalizeOptionalStringValue(in.DatabaseName),
		in.ActorUserID,
	)
	if err != nil {
		return Settings{}, "", err
	}

	_, err = tx.Exec(ctx, `
		INSERT INTO audit_log (actor_user_id, event_type, details)
		VALUES ($1::uuid, 'admin_settings_updated', $2::jsonb)
	`,
		in.ActorUserID,
		`{
			"target_type":"admin_settings",
			"target_id":"singleton",
			"after":{"saved":true}
		}`,
	)
	if err != nil {
		return Settings{}, "", err
	}

	if err := tx.Commit(ctx); err != nil {
		return Settings{}, "", err
	}

	out, err := s.Get(ctx)
	if err != nil {
		return Settings{}, "", err
	}
	return out, "", nil
}
