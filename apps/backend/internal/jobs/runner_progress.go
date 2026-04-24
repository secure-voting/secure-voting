package jobs

import "context"

func (r *Runner) UpdateProgress(ctx context.Context, jobID string, progress int) error {
	if progress < 0 {
		progress = 0
	}
	if progress > 100 {
		progress = 100
	}

	return runnerExecFn(ctx, r.db, `
UPDATE jobs
SET progress = $2
WHERE id = $1::uuid
  AND status = 'running'
  AND progress <> $2
`, jobID, progress)
}
