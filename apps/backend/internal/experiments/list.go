package experiments

import (
	"context"
	"strconv"
	"strings"
	"time"
)

func (s *Service) List(ctx context.Context, role, userID string, p ListParams) ([]Experiment, error) {
	limit := p.Limit
	if limit <= 0 || limit > 200 {
		limit = 50
	}

	offset := p.Offset
	if offset < 0 {
		offset = 0
	}

	q := `
		SELECT id::text, type, COALESCE(params,'{}'::jsonb), status, seed, created_by::text, created_at
		FROM experiments
	`
	var where []string
	var args []any
	i := 1

	if role != "admin" {
		where = append(where, "created_by = $"+strconv.Itoa(i)+"::uuid")
		args = append(args, userID)
		i++
	}

	if strings.TrimSpace(p.Type) != "" {
		where = append(where, "type = $"+strconv.Itoa(i))
		args = append(args, norm(p.Type))
		i++
	}
	if strings.TrimSpace(p.Status) != "" {
		where = append(where, "status = $"+strconv.Itoa(i))
		args = append(args, strings.TrimSpace(p.Status))
		i++
	}

	if len(where) > 0 {
		q += " WHERE " + strings.Join(where, " AND ")
	}

	q += " ORDER BY created_at DESC LIMIT $" + strconv.Itoa(i) + " OFFSET $" + strconv.Itoa(i+1)
	args = append(args, limit, offset)

	rows, err := s.db.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]Experiment, 0)
	for rows.Next() {
		var e Experiment
		var createdAt time.Time
		var params []byte

		if err := rows.Scan(&e.ID, &e.Type, &params, &e.Status, &e.Seed, &e.CreatedBy, &createdAt); err != nil {
			return nil, err
		}

		e.Params = params
		e.CreatedAt = createdAt.UTC().Format(time.RFC3339)
		out = append(out, e)
	}

	return out, nil
}
