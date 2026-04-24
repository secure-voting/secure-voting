package jobs

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func (s *Service) Get(ctx context.Context, role, userID, jobID string) (Job, string, error) {
	if _, err := uuid.Parse(jobID); err != nil {
		return Job{}, "invalid_id", nil
	}

	var j Job
	var createdAt time.Time
	var startedAt, finishedAt *time.Time

	q := `
		SELECT id::text, kind, status, progress, created_by::text,
		       election_id::text, experiment_id::text, experiment_run_id::text,
		       error_text, created_at, started_at, finished_at
		FROM jobs
		WHERE id = $1::uuid
	`
	err := getJobQueryRowFn(ctx, s.db, q, jobID).Scan(
		&j.ID, &j.Kind, &j.Status, &j.Progress, &j.CreatedBy,
		&j.ElectionID, &j.ExperimentID, &j.ExperimentRunID,
		&j.ErrorText, &createdAt, &startedAt, &finishedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Job{}, "not_found", nil
		}
		return Job{}, "", err
	}

	if role != "admin" && j.CreatedBy != userID {
		return Job{}, "not_found", nil
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

	return j, "", nil
}
