package experimentruns

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func (s *Service) Get(ctx context.Context, role, userID, runID string) (Run, string, error) {
	if _, err := uuid.Parse(strings.TrimSpace(runID)); err != nil {
		return Run{}, "invalid_id", nil
	}

	var r Run
	var kernel *string
	var started, finished *time.Time
	var createdBy string

	err := getRunQueryRowFn(ctx, s.db, `
		SELECT r.id::text, r.experiment_id::text, r.dataset_id, r.status, r.kernel_task_id, r.started_at, r.finished_at,
		       e.created_by::text
		FROM experiment_runs r
		JOIN experiments e ON e.id = r.experiment_id
		WHERE r.id = $1::uuid
	`, runID).Scan(&r.ID, &r.ExperimentID, &r.DatasetID, &r.Status, &kernel, &started, &finished, &createdBy)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Run{}, "not_found", nil
		}
		return Run{}, "", err
	}

	if role != "admin" && createdBy != userID {
		return Run{}, "not_found", nil
	}

	r.KernelTaskID = kernel
	if started != nil {
		s := started.UTC().Format(time.RFC3339)
		r.StartedAt = &s
	}
	if finished != nil {
		s := finished.UTC().Format(time.RFC3339)
		r.FinishedAt = &s
	}

	return r, "", nil
}
