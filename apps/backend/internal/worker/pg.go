package worker

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"github.com/jackc/pgx/v5"
)

func (w *Worker) loadExperiment(ctx context.Context, experimentID string) (expType string, expSeed *int64, params json.RawMessage, code string, err error) {
	var t string
	var p []byte
	var seed *int64

	err = w.db.QueryRow(ctx, `
SELECT type, COALESCE(params,'{}'::jsonb), seed
FROM experiments
WHERE id=$1::uuid
`, experimentID).Scan(&t, &p, &seed)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", nil, nil, "not_found", nil
		}
		return "", nil, nil, "", err
	}

	return t, seed, p, "", nil
}

func (w *Worker) markRunRunning(ctx context.Context, runID string, kernelTaskID string) error {
	_, err := w.db.Exec(ctx, `
UPDATE experiment_runs
SET status='running',
    started_at=COALESCE(started_at, now()),
    kernel_task_id=$2
WHERE id=$1::uuid
`, runID, kernelTaskID)
	return err
}

func (w *Worker) failRunAndJob(ctx context.Context, runID, jobID, errText string) error {
	errText = strings.TrimSpace(errText)
	if errText == "" {
		errText = "job failed"
	}

	tx, err := w.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	_, err = tx.Exec(ctx, `
UPDATE experiment_runs
SET status='error', finished_at=now()
WHERE id=$1::uuid
`, runID)
	if err != nil {
		return err
	}

	_, err = tx.Exec(ctx, `
UPDATE jobs
SET status='error', finished_at=now(), error_text=$2
WHERE id=$1::uuid
`, jobID, errText)
	if err != nil {
		return err
	}

	return tx.Commit(ctx)
}
