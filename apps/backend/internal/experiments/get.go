package experiments

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func (s *Service) Get(ctx context.Context, role, userID, id string) (Experiment, string, error) {
	if _, err := uuid.Parse(strings.TrimSpace(id)); err != nil {
		return Experiment{}, "invalid_id", nil
	}

	var e Experiment
	var createdAt time.Time
	var params []byte

	err := s.db.QueryRow(ctx, `
		SELECT id::text, type, COALESCE(params,'{}'::jsonb), status, seed, created_by::text, created_at
		FROM experiments
		WHERE id=$1::uuid
	`, id).Scan(&e.ID, &e.Type, &params, &e.Status, &e.Seed, &e.CreatedBy, &createdAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Experiment{}, "not_found", nil
		}
		return Experiment{}, "", err
	}

	if role != "admin" && e.CreatedBy != userID {
		return Experiment{}, "not_found", nil
	}

	e.Params = params
	e.CreatedAt = createdAt.UTC().Format(time.RFC3339)
	return e, "", nil
}
