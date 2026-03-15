package worker

import (
	"context"
	"encoding/json"
	"strings"
	"time"
)

func (w *Worker) applyExperimentRunResult(ctx context.Context, res ExperimentRunResult) error {
	oidHex, err := w.upsertExperimentResult(ctx, res)
	if err != nil {
		return err
	}

	tx, err := w.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	now := time.Now().UTC()

	if res.Status == "done" {
		_, err = tx.Exec(ctx, `
UPDATE experiment_runs
SET status='done', finished_at=$2
WHERE id=$1::uuid
`, res.RunID, now)
		if err != nil {
			return err
		}

		ref := map[string]any{
			"mongo_experiment_result_id": oidHex,
			"run_id":                     res.RunID,
		}
		refJSON, _ := json.Marshal(ref)

		_, err = tx.Exec(ctx, `
UPDATE jobs
SET status='done', progress=100, finished_at=$2, error_text=NULL, result_ref=$3::jsonb
WHERE kind='experiment_run'
  AND experiment_run_id=$1::uuid
  AND status IN ('queued','running')
`, res.RunID, now, string(refJSON))
		if err != nil {
			return err
		}
	} else {
		errText := strings.TrimSpace(res.ErrorText)
		if errText == "" {
			errText = "experiment_run failed"
		}

		_, err = tx.Exec(ctx, `
UPDATE experiment_runs
SET status='error', finished_at=$2
WHERE id=$1::uuid
`, res.RunID, now)
		if err != nil {
			return err
		}

		_, err = tx.Exec(ctx, `
UPDATE jobs
SET status='error', finished_at=$2, error_text=$3
WHERE kind='experiment_run'
  AND experiment_run_id=$1::uuid
  AND status IN ('queued','running')
`, res.RunID, now, errText)
		if err != nil {
			return err
		}
	}

	return tx.Commit(ctx)
}
