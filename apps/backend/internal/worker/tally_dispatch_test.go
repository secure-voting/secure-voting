package worker

import (
	"context"
	"testing"

	"secure-voting/apps/backend/internal/jobs"
)

func TestHandleTallyJob_RoutesRankingToExternal(t *testing.T) {
	oldLoadMeta := loadElectionRouteMetaFn
	oldHandleExternal := handleElectionTallyExternalFn
	oldHandleLocal := handleTallyLocalFn
	defer func() {
		loadElectionRouteMetaFn = oldLoadMeta
		handleElectionTallyExternalFn = oldHandleExternal
		handleTallyLocalFn = oldHandleLocal
	}()

	electionID := "11111111-1111-1111-1111-111111111111"
	job := jobs.ClaimedJob{
		ID:         "job-1",
		Kind:       jobKindTally,
		ElectionID: &electionID,
	}

	loadElectionRouteMetaFn = func(_ *Worker, _ context.Context, _ string) (string, string, error) {
		return "ranking", "plurality", nil
	}

	externalCalled := 0
	localCalled := 0

	handleElectionTallyExternalFn = func(_ *Worker, _ context.Context, got jobs.ClaimedJob) error {
		externalCalled++
		if got.ID != job.ID {
			t.Fatalf("unexpected job id: %s", got.ID)
		}
		return nil
	}

	handleTallyLocalFn = func(_ *Worker, _ context.Context, _ jobs.ClaimedJob) error {
		localCalled++
		return nil
	}

	w := &Worker{}
	if err := w.handleTallyJob(context.Background(), job); err != nil {
		t.Fatalf("handleTallyJob error: %v", err)
	}

	if externalCalled != 1 {
		t.Fatalf("expected externalCalled=1, got %d", externalCalled)
	}
	if localCalled != 0 {
		t.Fatalf("expected localCalled=0, got %d", localCalled)
	}
}

func TestHandleTallyJob_RoutesApprovalToLocal(t *testing.T) {
	oldLoadMeta := loadElectionRouteMetaFn
	oldHandleExternal := handleElectionTallyExternalFn
	oldHandleLocal := handleTallyLocalFn
	defer func() {
		loadElectionRouteMetaFn = oldLoadMeta
		handleElectionTallyExternalFn = oldHandleExternal
		handleTallyLocalFn = oldHandleLocal
	}()

	electionID := "22222222-2222-2222-2222-222222222222"
	job := jobs.ClaimedJob{
		ID:         "job-2",
		Kind:       jobKindTally,
		ElectionID: &electionID,
	}

	loadElectionRouteMetaFn = func(_ *Worker, _ context.Context, _ string) (string, string, error) {
		return "approval", "approval", nil
	}

	externalCalled := 0
	localCalled := 0

	handleElectionTallyExternalFn = func(_ *Worker, _ context.Context, _ jobs.ClaimedJob) error {
		externalCalled++
		return nil
	}

	handleTallyLocalFn = func(_ *Worker, _ context.Context, got jobs.ClaimedJob) error {
		localCalled++
		if got.ID != job.ID {
			t.Fatalf("unexpected job id: %s", got.ID)
		}
		return nil
	}

	w := &Worker{}
	if err := w.handleTallyJob(context.Background(), job); err != nil {
		t.Fatalf("handleTallyJob error: %v", err)
	}

	if externalCalled != 0 {
		t.Fatalf("expected externalCalled=0, got %d", externalCalled)
	}
	if localCalled != 1 {
		t.Fatalf("expected localCalled=1, got %d", localCalled)
	}
}

func TestHandleTallyJob_RoutesUnsupportedRankingToLocal(t *testing.T) {
	oldLoadMeta := loadElectionRouteMetaFn
	oldHandleExternal := handleElectionTallyExternalFn
	oldHandleLocal := handleTallyLocalFn
	defer func() {
		loadElectionRouteMetaFn = oldLoadMeta
		handleElectionTallyExternalFn = oldHandleExternal
		handleTallyLocalFn = oldHandleLocal
	}()

	electionID := "33333333-3333-3333-3333-333333333333"
	job := jobs.ClaimedJob{
		ID:         "job-3",
		Kind:       jobKindTally,
		ElectionID: &electionID,
	}

	loadElectionRouteMetaFn = func(_ *Worker, _ context.Context, _ string) (string, string, error) {
		return "ranking", "practical_condorcet", nil
	}

	externalCalled := 0
	localCalled := 0

	handleElectionTallyExternalFn = func(_ *Worker, _ context.Context, _ jobs.ClaimedJob) error {
		externalCalled++
		return nil
	}

	handleTallyLocalFn = func(_ *Worker, _ context.Context, _ jobs.ClaimedJob) error {
		localCalled++
		return nil
	}

	w := &Worker{}
	if err := w.handleTallyJob(context.Background(), job); err != nil {
		t.Fatalf("handleTallyJob error: %v", err)
	}

	if externalCalled != 0 {
		t.Fatalf("expected externalCalled=0, got %d", externalCalled)
	}
	if localCalled != 1 {
		t.Fatalf("expected localCalled=1, got %d", localCalled)
	}
}
