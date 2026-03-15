package jobs

import (
	"context"
	"strconv"
	"time"
)

func (s *Service) List(ctx context.Context, role, userID string, f ListFilter) ([]Job, error) {
	limit := f.Limit
	if limit <= 0 || limit > 200 {
		limit = 50
	}

	base := `
		SELECT id::text, kind, status, progress, created_by::text,
		       election_id::text, experiment_id::text, experiment_run_id::text,
		       error_text, created_at, started_at, finished_at
		FROM jobs
		WHERE 1=1
	`
	args := []any{}
	argn := 1

	if role != "admin" {
		base += ` AND created_by = $` + strconv.Itoa(argn)
		args = append(args, userID)
		argn++
	}

	if f.Status != nil {
		base += ` AND status = $` + strconv.Itoa(argn)
		args = append(args, *f.Status)
		argn++
	}
	if f.Kind != nil {
		base += ` AND kind = $` + strconv.Itoa(argn)
		args = append(args, *f.Kind)
		argn++
	}

	base += ` ORDER BY created_at DESC LIMIT $` + strconv.Itoa(argn) + ` OFFSET $` + strconv.Itoa(argn+1)
	args = append(args, limit, f.Offset)

	rows, err := s.db.Query(ctx, base, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]Job, 0)
	for rows.Next() {
		var j Job
		var createdAt time.Time
		var startedAt, finishedAt *time.Time

		if err := rows.Scan(
			&j.ID, &j.Kind, &j.Status, &j.Progress, &j.CreatedBy,
			&j.ElectionID, &j.ExperimentID, &j.ExperimentRunID,
			&j.ErrorText, &createdAt, &startedAt, &finishedAt,
		); err != nil {
			return nil, err
		}

		j.CreatedAt = createdAt.UTC().Format(time.RFC3339)
		if startedAt != nil {
			st := startedAt.UTC().Format(time.RFC3339)
			j.StartedAt = &st
		}
		if finishedAt != nil {
			ft := finishedAt.UTC().Format(time.RFC3339)
			j.FinishedAt = &ft
		}

		out = append(out, j)
	}
	return out, nil
}
