package notifications

import (
	"context"
	"strings"
	"time"
)

func (s *Service) List(ctx context.Context, userID string, limit, offset int) ([]Item, string, error) {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return nil, "unauthorized", nil
	}

	if limit <= 0 || limit > 100 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}

	rows, err := s.db.Query(ctx, `
		SELECT id::text, title, message, details, action_label, action_to, kind, created_at, read_at
		FROM notifications
		WHERE user_id = $1::uuid
		ORDER BY created_at DESC, id DESC
		LIMIT $2 OFFSET $3
	`, userID, limit, offset)
	if err != nil {
		return nil, "", err
	}
	defer rows.Close()

	items := make([]Item, 0, 16)
	for rows.Next() {
		var it Item
		var details *string
		var actionLabel *string
		var actionTo *string
		var createdAt time.Time
		var readAt *time.Time

		if err := rows.Scan(
			&it.ID,
			&it.Title,
			&it.Message,
			&details,
			&actionLabel,
			&actionTo,
			&it.Kind,
			&createdAt,
			&readAt,
		); err != nil {
			return nil, "", err
		}

		it.Details = normalizeOptionalStringPtr(details)
		it.ActionLabel = normalizeOptionalStringPtr(actionLabel)
		it.ActionTo = normalizeOptionalStringPtr(actionTo)
		it.CreatedAt = formatTime(createdAt)
		it.Read = readAt != nil

		items = append(items, it)
	}

	if err := rows.Err(); err != nil {
		return nil, "", err
	}

	return items, "", nil
}

func (s *Service) Create(ctx context.Context, in CreateInput) (Item, string, error) {
	in.UserID = strings.TrimSpace(in.UserID)
	in.Title = strings.TrimSpace(in.Title)
	in.Message = strings.TrimSpace(in.Message)
	in.Details = strings.TrimSpace(in.Details)
	in.ActionLabel = strings.TrimSpace(in.ActionLabel)
	in.ActionTo = strings.TrimSpace(in.ActionTo)
	in.Kind = normalizeKind(in.Kind)

	if in.UserID == "" {
		return Item{}, "unauthorized", nil
	}
	if !validateTitle(in.Title) {
		return Item{}, "invalid_title", nil
	}
	if !validateMessage(in.Message) {
		return Item{}, "invalid_message", nil
	}
	if !validateDetails(in.Details) {
		return Item{}, "invalid_details", nil
	}
	if !validateActionLabel(in.ActionLabel) {
		return Item{}, "invalid_action_label", nil
	}
	if !validateActionTo(in.ActionTo) {
		return Item{}, "invalid_action_to", nil
	}

	var out Item
	var details *string
	var actionLabel *string
	var actionTo *string
	var createdAt time.Time
	var readAt *time.Time

	err := s.db.QueryRow(ctx, `
		INSERT INTO notifications (
			user_id, title, message, details, action_label, action_to, kind
		)
		VALUES ($1::uuid, $2, $3, $4, $5, $6, $7)
		RETURNING id::text, title, message, details, action_label, action_to, kind, created_at, read_at
	`,
		in.UserID,
		in.Title,
		in.Message,
		normalizeOptionalValue(in.Details),
		normalizeOptionalValue(in.ActionLabel),
		normalizeOptionalValue(in.ActionTo),
		in.Kind,
	).Scan(
		&out.ID,
		&out.Title,
		&out.Message,
		&details,
		&actionLabel,
		&actionTo,
		&out.Kind,
		&createdAt,
		&readAt,
	)
	if err != nil {
		return Item{}, "", err
	}

	out.Details = normalizeOptionalStringPtr(details)
	out.ActionLabel = normalizeOptionalStringPtr(actionLabel)
	out.ActionTo = normalizeOptionalStringPtr(actionTo)
	out.CreatedAt = formatTime(createdAt)
	out.Read = readAt != nil

	return out, "", nil
}

func (s *Service) MarkRead(ctx context.Context, userID, notificationID string) (string, error) {
	userID = strings.TrimSpace(userID)
	notificationID = strings.TrimSpace(notificationID)

	if userID == "" {
		return "unauthorized", nil
	}
	if notificationID == "" {
		return "invalid_id", nil
	}

	tag, err := s.db.Exec(ctx, `
		UPDATE notifications
		SET read_at = COALESCE(read_at, now())
		WHERE id = $1::uuid AND user_id = $2::uuid
	`, notificationID, userID)
	if err != nil {
		return "", err
	}
	if tag.RowsAffected() == 0 {
		return "not_found", nil
	}

	return "", nil
}

func (s *Service) MarkAllRead(ctx context.Context, userID string) (string, error) {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return "unauthorized", nil
	}

	_, err := s.db.Exec(ctx, `
		UPDATE notifications
		SET read_at = COALESCE(read_at, now())
		WHERE user_id = $1::uuid
	`, userID)
	if err != nil {
		return "", err
	}
	return "", nil
}

func (s *Service) Delete(ctx context.Context, userID, notificationID string) (string, error) {
	userID = strings.TrimSpace(userID)
	notificationID = strings.TrimSpace(notificationID)

	if userID == "" {
		return "unauthorized", nil
	}
	if notificationID == "" {
		return "invalid_id", nil
	}

	tag, err := s.db.Exec(ctx, `
		DELETE FROM notifications
		WHERE id = $1::uuid AND user_id = $2::uuid
	`, notificationID, userID)
	if err != nil {
		return "", err
	}
	if tag.RowsAffected() == 0 {
		return "not_found", nil
	}

	return "", nil
}

func (s *Service) ClearAll(ctx context.Context, userID string) (string, error) {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return "unauthorized", nil
	}

	_, err := s.db.Exec(ctx, `
		DELETE FROM notifications
		WHERE user_id = $1::uuid
	`, userID)
	if err != nil {
		return "", err
	}
	return "", nil
}

func (s *Service) SeedIfEmpty(ctx context.Context, userID string) error {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return nil
	}

	var count int
	err := s.db.QueryRow(ctx, `
		SELECT count(*)
		FROM notifications
		WHERE user_id = $1::uuid
	`, userID).Scan(&count)
	if err != nil {
		return err
	}
	if count > 0 {
		return nil
	}

	_, _, err = s.Create(ctx, CreateInput{
		UserID:  userID,
		Title:   "Добро пожаловать",
		Message: "Раздел уведомлений подключен к backend persistence.",
		Kind:    string(KindInfo),
	})
	return err
}
