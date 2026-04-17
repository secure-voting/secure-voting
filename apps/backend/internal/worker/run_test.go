package worker

import (
	"context"
	"testing"
	"time"

	"secure-voting/apps/backend/internal/jobs"
)

func TestTick_Tally_Delegates(t *testing.T) {
	oldClaimNextJobFn := claimNextJobFn
	oldHandleTallyJobFn := handleTallyJobFn
	oldHandleExperimentRunFn := handleExperimentRunFn
	oldRunSchedulersFn := runSchedulersFn
	oldHandleTallyLocalFn := handleTallyLocalFn

	defer func() {
		claimNextJobFn = oldClaimNextJobFn
		handleTallyJobFn = oldHandleTallyJobFn
		handleExperimentRunFn = oldHandleExperimentRunFn
		runSchedulersFn = oldRunSchedulersFn
		handleTallyLocalFn = oldHandleTallyLocalFn
	}()

	electionID := "44444444-4444-4444-4444-444444444444"

	claimNextJobFn = func(w *Worker, ctx context.Context, kinds []string) (jobs.ClaimedJob, bool, error) {
		return jobs.ClaimedJob{
			ID:         "55555555-5555-5555-5555-555555555555",
			Kind:       jobKindTally,
			ElectionID: &electionID,
		}, true, nil
	}

	runSchedulersFn = func(w *Worker, ctx context.Context) error {
		return nil
	}

	tallyJobCalled := 0
	experimentCalled := 0
	localCalled := 0

	handleTallyJobFn = func(w *Worker, ctx context.Context, job jobs.ClaimedJob) error {
		tallyJobCalled++
		return nil
	}

	handleExperimentRunFn = func(w *Worker, ctx context.Context, job jobs.ClaimedJob) error {
		experimentCalled++
		return nil
	}

	handleTallyLocalFn = func(w *Worker, ctx context.Context, job jobs.ClaimedJob) error {
		localCalled++
		return nil
	}

	w := &Worker{
		pollInterval:     time.Second,
		scheduleInterval: time.Hour,
		nextScheduleAt:   time.Now().UTC().Add(time.Hour),
	}

	if err := w.tick(context.Background()); err != nil {
		t.Fatalf("tick returned error: %v", err)
	}

	if tallyJobCalled != 1 {
		t.Fatalf("expected handleTallyJob to be called once, got %d", tallyJobCalled)
	}
	if experimentCalled != 0 {
		t.Fatalf("expected handleExperimentRun not to be called, got %d", experimentCalled)
	}
	if localCalled != 0 {
		t.Fatalf("expected handleTallyLocal not to be called, got %d", localCalled)
	}
}
