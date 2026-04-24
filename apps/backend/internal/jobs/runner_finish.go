package jobs

import (
	"context"
	"encoding/json"
	"strings"
)

func (r *Runner) MarkDone(ctx context.Context, jobID string, resultRef map[string]any) error {
	var ref any
	if resultRef != nil {
		b, err := json.Marshal(resultRef)
		if err != nil {
			return err
		}
		ref = string(b)
	}

	return runnerExecFn(ctx, r.db, `
UPDATE jobs
SET status='done', progress=100, finished_at=now(), error_text=NULL, result_ref=$2::jsonb
WHERE id=$1::uuid
`, jobID, ref)
}

func (r *Runner) MarkError(ctx context.Context, jobID string, errorText string) error {
	errorText = strings.TrimSpace(errorText)
	if errorText == "" {
		errorText = "job failed"
	}

	return runnerExecFn(ctx, r.db, `
UPDATE jobs
SET status='error', finished_at=now(), error_text=$2
WHERE id=$1::uuid
`, jobID, errorText)
}
