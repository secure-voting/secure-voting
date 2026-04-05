package worker

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"secure-voting/apps/backend/internal/jobs"
)

func restoreRunHooks() func() {
	oldConsumeResultsFn := consumeResultsFn
	oldClaimNextJobFn := claimNextJobFn
	oldUpdateProgressFn := updateProgressFn
	oldMarkJobErrorFn := markJobErrorFn
	oldHandleExperimentRunFn := handleExperimentRunFn
	oldHandleTallyLocalFn := handleTallyLocalFn

	return func() {
		consumeResultsFn = oldConsumeResultsFn
		claimNextJobFn = oldClaimNextJobFn
		updateProgressFn = oldUpdateProgressFn
		markJobErrorFn = oldMarkJobErrorFn
		handleExperimentRunFn = oldHandleExperimentRunFn
		handleTallyLocalFn = oldHandleTallyLocalFn
	}
}

func TestRun_ReturnsConsumeResultsError(t *testing.T) {
	defer restoreRunHooks()()

	consumeResultsFn = func(_ *Worker, _ context.Context) error {
		return errors.New("consume failed")
	}

	w := &Worker{pollInterval: time.Millisecond}
	err := w.Run(context.Background())
	if err == nil || !strings.Contains(err.Error(), "consume failed") {
		t.Fatalf("expected consume failed error, got %v", err)
	}
}

func TestRun_ContextCanceled(t *testing.T) {
	defer restoreRunHooks()()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	consumeResultsFn = func(_ *Worker, ctx context.Context) error {
		<-ctx.Done()
		return ctx.Err()
	}

	w := &Worker{pollInterval: time.Millisecond}
	err := w.Run(ctx)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context canceled, got %v", err)
	}
}

func TestTick_NoJob(t *testing.T) {
	defer restoreRunHooks()()

	claimNextJobFn = func(_ *Worker, _ context.Context, _ []string) (jobs.ClaimedJob, bool, error) {
		return jobs.ClaimedJob{}, false, nil
	}

	if err := (&Worker{}).tick(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestTick_ClaimError(t *testing.T) {
	defer restoreRunHooks()()

	claimNextJobFn = func(_ *Worker, _ context.Context, _ []string) (jobs.ClaimedJob, bool, error) {
		return jobs.ClaimedJob{}, false, errors.New("boom")
	}

	err := (&Worker{}).tick(context.Background())
	if err == nil || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("expected boom, got %v", err)
	}
}

func TestTick_ExperimentRun_Delegates(t *testing.T) {
	defer restoreRunHooks()()

	called := false
	claimNextJobFn = func(_ *Worker, _ context.Context, _ []string) (jobs.ClaimedJob, bool, error) {
		return jobs.ClaimedJob{ID: "job-1", Kind: jobKindExperimentRun}, true, nil
	}
	updateProgressFn = func(_ *Worker, _ context.Context, _ string, _ int) error { return nil }
	handleExperimentRunFn = func(_ *Worker, _ context.Context, job jobs.ClaimedJob) error {
		called = true
		if job.Kind != jobKindExperimentRun {
			t.Fatalf("unexpected kind: %q", job.Kind)
		}
		return nil
	}

	if err := (&Worker{}).tick(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Fatal("expected handleExperimentRun to be called")
	}
}

func TestTick_Tally_Delegates(t *testing.T) {
	defer restoreRunHooks()()

	called := false
	claimNextJobFn = func(_ *Worker, _ context.Context, _ []string) (jobs.ClaimedJob, bool, error) {
		return jobs.ClaimedJob{ID: "job-1", Kind: jobKindTally}, true, nil
	}
	updateProgressFn = func(_ *Worker, _ context.Context, _ string, _ int) error { return nil }
	handleTallyLocalFn = func(_ *Worker, _ context.Context, job jobs.ClaimedJob) error {
		called = true
		if job.Kind != jobKindTally {
			t.Fatalf("unexpected kind: %q", job.Kind)
		}
		return nil
	}

	if err := (&Worker{}).tick(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Fatal("expected handleTallyLocal to be called")
	}
}

func TestTick_UnsupportedKind_MarksError(t *testing.T) {
	defer restoreRunHooks()()

	var marked string
	claimNextJobFn = func(_ *Worker, _ context.Context, _ []string) (jobs.ClaimedJob, bool, error) {
		return jobs.ClaimedJob{ID: "job-1", Kind: "weird"}, true, nil
	}
	updateProgressFn = func(_ *Worker, _ context.Context, _ string, _ int) error { return nil }
	markJobErrorFn = func(_ *Worker, _ context.Context, _ string, errText string) error {
		marked = errText
		return nil
	}

	if err := (&Worker{}).tick(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(marked, "unsupported job kind") {
		t.Fatalf("unexpected mark error: %q", marked)
	}
}
