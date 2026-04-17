package worker

import (
	"context"

	"secure-voting/apps/backend/internal/jobs"
)

var handleTallyLocalFn = func(w *Worker, ctx context.Context, job jobs.ClaimedJob) error {
	_ = markJobErrorFn(w, ctx, job.ID, "local tally removed")
	return nil
}
