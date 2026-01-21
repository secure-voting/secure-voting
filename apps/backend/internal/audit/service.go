package audit

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Service struct {
	db *pgxpool.Pool
}

func NewService(db *pgxpool.Pool) *Service {
	return &Service{db: db}
}

type Record struct {
	ID         int64   `json:"id"`
	OccurredAt string  `json:"occurred_at"`
	ActorUserID *string `json:"actor_user_id,omitempty"`
	EventType  string  `json:"event_type"`
	Details    any     `json:"details"`
}

type ListFilter struct {
	EventType   *string
	ActorUserID *string
	Since       *time.Time
	Until       *time.Time
	Limit       int
	Offset      int
}

func ParseUUIDOrEmpty(s string) (string, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", true
	}
	_, err := uuid.Parse(s)
	return s, err == nil
}

func (s *Service) List(ctx context.Context, role, userID string, f ListFilter) ([]Record, error) {
	limit := f.Limit
	if limit <= 0 || limit > 200 {
		limit = 50
	}

	// Не-admin: показываем только свои записи (actor_user_id = user)
	// Admin: все
	q := `
		SELECT id, occurred_at, actor_user_id::text, event_type, details
		FROM audit_log
		WHERE 1=1
	`
	args := []any{}
	argn := 1

	if role != "admin" {
		q += ` AND actor_user_id = $` + itoa(argn)
		args = append(args, userID)
		argn++
	}

	if f.EventType != nil {
		q += ` AND event_type = $` + itoa(argn)
		args = append(args, *f.EventType)
		argn++
	}
	if f.ActorUserID != nil {
		q += ` AND actor_user_id = $` + itoa(argn) + `::uuid`
		args = append(args, *f.ActorUserID)
		argn++
	}
	if f.Since != nil {
		q += ` AND occurred_at >= $` + itoa(argn)
		args = append(args, *f.Since)
		argn++
	}
	if f.Until != nil {
		q += ` AND occurred_at <= $` + itoa(argn)
		args = append(args, *f.Until)
		argn++
	}

	q += ` ORDER BY occurred_at DESC LIMIT $` + itoa(argn) + ` OFFSET $` + itoa(argn+1)
	args = append(args, limit, f.Offset)

	rows, err := s.db.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Record
	for rows.Next() {
		var r Record
		var t time.Time
		var details any
		if err := rows.Scan(&r.ID, &t, &r.ActorUserID, &r.EventType, &details); err != nil {
			return nil, err
		}
		r.OccurredAt = t.UTC().Format(time.RFC3339)
		r.Details = details
		out = append(out, r)
	}
	return out, nil
}

func (s *Service) ParseTimeRFC3339(v string) (*time.Time, error) {
	v = strings.TrimSpace(v)
	if v == "" {
		return nil, nil
	}
	t, err := time.Parse(time.RFC3339, v)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

func ParseInt(v string) (int, bool) {
	v = strings.TrimSpace(v)
	if v == "" {
		return 0, false
	}
	n, err := strconv.Atoi(v)
	return n, err == nil
}

func itoa(i int) string { return strconv.Itoa(i) }

var _ = errors.Is
var _ = pgx.ErrNoRows
