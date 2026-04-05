package worker

import (
	"context"
	"log"
	"time"

	"secure-voting/apps/backend/internal/jobs"
)

var consumeResultsFn = func(w *Worker, ctx context.Context) error {
	return w.consumeResults(ctx)
}

var claimNextJobFn = func(w *Worker, ctx context.Context, kinds []string) (jobs.ClaimedJob, bool, error) {
	return w.runner.ClaimNext(ctx, kinds)
}

var updateProgressFn = func(w *Worker, ctx context.Context, jobID string, progress int) error {
	if w.runner == nil {
		return nil
	}
	return w.runner.UpdateProgress(ctx, jobID, progress)
}

var markJobErrorFn = func(w *Worker, ctx context.Context, jobID, errText string) error {
	if w.runner == nil {
		return nil
	}
	return w.runner.MarkError(ctx, jobID, errText)
}

var handleExperimentRunFn = func(w *Worker, ctx context.Context, job jobs.ClaimedJob) error {
	return w.handleExperimentRun(ctx, job)
}

var handleTallyLocalFn = func(w *Worker, ctx context.Context, job jobs.ClaimedJob) error {
	return w.handleTallyLocal(ctx, job)
}

var handleTallyJobFn = func(w *Worker, ctx context.Context, job jobs.ClaimedJob) error {
	return w.handleTallyJob(ctx, job)
}

func (w *Worker) Run(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	errCh := make(chan error, 1)
	doneCh := make(chan struct{})

	go func() {
		defer close(doneCh)
		errCh <- consumeResultsFn(w, ctx)
	}()

	ticker := time.NewTicker(w.pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			<-doneCh
			return ctx.Err()

		case err := <-errCh:
			<-doneCh
			return err

		case <-ticker.C:
			if err := w.tick(ctx); err != nil {
				log.Printf("worker.tick error: %v", err)
			}
		}
	}
}

func (w *Worker) tick(ctx context.Context) error {
	job, ok, err := claimNextJobFn(w, ctx, []string{jobKindTally, jobKindExperimentRun})
	if err != nil {
		return err
	}
	if !ok {
		return nil
	}

	_ = updateProgressFn(w, ctx, job.ID, 5)

	switch job.Kind {
	case jobKindExperimentRun:
		return handleExperimentRunFn(w, ctx, job)
	case jobKindTally:
		return handleTallyJobFn(w, ctx, job)
	default:
		_ = markJobErrorFn(w, ctx, job.ID, "unsupported job kind: "+job.Kind)
		return nil
	}
}
