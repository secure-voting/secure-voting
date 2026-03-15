package experimentruns

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"
)

func (s *Service) List(ctx context.Context, role, userID, experimentID string) ([]Run, string, error) {
	args := []any{}
	q := `
		SELECT r.id::text, r.experiment_id::text, r.dataset_id, r.status, r.kernel_task_id, r.started_at, r.finished_at,
		       e.created_by::text
		FROM experiment_runs r
		JOIN experiments e ON e.id = r.experiment_id
		WHERE 1=1
	`
	argn := 1

	if experimentID != "" {
		if _, err := uuid.Parse(strings.TrimSpace(experimentID)); err != nil {
			return nil, "invalid_experiment_id", nil
		}
		q += ` AND r.experiment_id = $` + itoa(argn) + `::uuid`
		args = append(args, experimentID)
	}

	if role != "admin" {
		q += ` AND e.created_by = $` + itoa(argn) + `::uuid`
		args = append(args, userID)
	}

	q += ` ORDER BY r.started_at NULLS LAST, r.id DESC`

	rows, err := s.db.Query(ctx, q, args...)
	if err != nil {
		return nil, "", err
	}
	defer rows.Close()

	out := make([]Run, 0)
	for rows.Next() {
		var r Run
		var kernel *string
		var started, finished *time.Time
		var createdBy string
		if err := rows.Scan(&r.ID, &r.ExperimentID, &r.DatasetID, &r.Status, &kernel, &started, &finished, &createdBy); err != nil {
			return nil, "", err
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
		out = append(out, r)
	}

	return out, "", nil
}
