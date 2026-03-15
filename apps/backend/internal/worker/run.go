package worker

import (
	"context"
	"log"
	"time"
)

func (w *Worker) Run(ctx context.Context) error {
	errCh := make(chan error, 1)

	go func() {
		errCh <- w.consumeResults(ctx)
	}()

	ticker := time.NewTicker(w.pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case err := <-errCh:
			return err
		case <-ticker.C:
			if err := w.tick(ctx); err != nil {
				log.Printf("worker.tick error: %v", err)
			}
		}
	}
}

func (w *Worker) tick(ctx context.Context) error {
	job, ok, err := w.runner.ClaimNext(ctx, []string{jobKindTally, jobKindExperimentRun})
	if err != nil {
		return err
	}
	if !ok {
		return nil
	}

	_ = w.runner.UpdateProgress(ctx, job.ID, 5)

	switch job.Kind {
	case jobKindExperimentRun:
		return w.handleExperimentRun(ctx, job)
	case jobKindTally:
		return w.handleTallyLocal(ctx, job)
	default:
		_ = w.runner.MarkError(ctx, job.ID, "unsupported job kind: "+job.Kind)
		return nil
	}
}
