package worker

import (
	"context"
	"testing"

	"secure-voting/apps/backend/internal/jobs"
)

func TestHandleTallyJob_RoutesApprovalToExternal(t *testing.T) {
	oldLoadElectionRouteMetaFn := loadElectionRouteMetaFn
	oldHandleElectionTallyExternalFn := handleElectionTallyExternalFn
	oldMarkJobErrorFn := markJobErrorFn

	defer func() {
		loadElectionRouteMetaFn = oldLoadElectionRouteMetaFn
		handleElectionTallyExternalFn = oldHandleElectionTallyExternalFn
		markJobErrorFn = oldMarkJobErrorFn
	}()

	loadElectionRouteMetaFn = func(w *Worker, ctx context.Context, electionID string) (string, string, error) {
		return "approval", "approval", nil
	}

	externalCalled := 0
	markErrorCalled := 0

	handleElectionTallyExternalFn = func(w *Worker, ctx context.Context, job jobs.ClaimedJob) error {
		externalCalled++
		return nil
	}

	markJobErrorFn = func(w *Worker, ctx context.Context, jobID, errText string) error {
		markErrorCalled++
		return nil
	}

	electionID := "44444444-4444-4444-4444-444444444444"
	job := jobs.ClaimedJob{
		ID:         "55555555-5555-5555-5555-555555555555",
		Kind:       jobKindTally,
		ElectionID: &electionID,
	}

	w := &Worker{}
	if err := w.handleTallyJob(context.Background(), job); err != nil {
		t.Fatalf("handleTallyJob returned error: %v", err)
	}

	if externalCalled != 1 {
		t.Fatalf("expected externalCalled=1, got %d", externalCalled)
	}
	if markErrorCalled != 0 {
		t.Fatalf("expected markErrorCalled=0, got %d", markErrorCalled)
	}
}

func TestHandleTallyJob_RoutesUnsupportedRankingToExternal(t *testing.T) {
	oldLoadElectionRouteMetaFn := loadElectionRouteMetaFn
	oldHandleElectionTallyExternalFn := handleElectionTallyExternalFn
	oldMarkJobErrorFn := markJobErrorFn

	defer func() {
		loadElectionRouteMetaFn = oldLoadElectionRouteMetaFn
		handleElectionTallyExternalFn = oldHandleElectionTallyExternalFn
		markJobErrorFn = oldMarkJobErrorFn
	}()

	loadElectionRouteMetaFn = func(w *Worker, ctx context.Context, electionID string) (string, string, error) {
		return "ranking", "unknown_rule", nil
	}

	externalCalled := 0
	markErrorCalled := 0

	handleElectionTallyExternalFn = func(w *Worker, ctx context.Context, job jobs.ClaimedJob) error {
		externalCalled++
		return nil
	}

	markJobErrorFn = func(w *Worker, ctx context.Context, jobID, errText string) error {
		markErrorCalled++
		return nil
	}

	electionID := "44444444-4444-4444-4444-444444444444"
	job := jobs.ClaimedJob{
		ID:         "55555555-5555-5555-5555-555555555555",
		Kind:       jobKindTally,
		ElectionID: &electionID,
	}

	w := &Worker{}
	if err := w.handleTallyJob(context.Background(), job); err != nil {
		t.Fatalf("handleTallyJob returned error: %v", err)
	}

	if externalCalled != 1 {
		t.Fatalf("expected externalCalled=1, got %d", externalCalled)
	}
	if markErrorCalled != 0 {
		t.Fatalf("expected markErrorCalled=0, got %d", markErrorCalled)
	}
}
