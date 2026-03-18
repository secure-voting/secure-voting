package worker

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/segmentio/kafka-go"

	"secure-voting/apps/backend/internal/jobs"
)

func restoreExperimentRunHooks() func() {
	oldWriteTaskMessageFn := writeTaskMessageFn
	oldLoadExperimentFn := loadExperimentFn
	oldLoadDatasetMetaFn := loadDatasetMetaFn
	oldMarkRunRunningFn := markRunRunningFn
	oldFailRunAndJobFn := failRunAndJobFn
	oldFetchResultMessageFn := fetchResultMessageFn
	oldApplyExperimentRunResultFn := applyExperimentRunResultFn
	oldCommitResultFn := commitResultFn
	oldUpdateProgressFn := updateProgressFn
	oldMarkJobErrorFn := markJobErrorFn

	return func() {
		writeTaskMessageFn = oldWriteTaskMessageFn
		loadExperimentFn = oldLoadExperimentFn
		loadDatasetMetaFn = oldLoadDatasetMetaFn
		markRunRunningFn = oldMarkRunRunningFn
		failRunAndJobFn = oldFailRunAndJobFn
		fetchResultMessageFn = oldFetchResultMessageFn
		applyExperimentRunResultFn = oldApplyExperimentRunResultFn
		commitResultFn = oldCommitResultFn
		updateProgressFn = oldUpdateProgressFn
		markJobErrorFn = oldMarkJobErrorFn
	}
}

func TestHandleExperimentRun_MissingExperimentRunID(t *testing.T) {
	defer restoreExperimentRunHooks()()

	var marked string
	markJobErrorFn = func(_ *Worker, _ context.Context, _, errText string) error {
		marked = errText
		return nil
	}

	w := &Worker{}
	job := jobs.ClaimedJob{ID: "job-1"}

	if err := w.handleExperimentRun(context.Background(), job); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if marked != "missing experiment_run_id in jobs row" {
		t.Fatalf("unexpected mark error: %q", marked)
	}
}

func TestHandleExperimentRun_InvalidPayloadMarksError(t *testing.T) {
	defer restoreExperimentRunHooks()()

	var marked string
	markJobErrorFn = func(_ *Worker, _ context.Context, _, errText string) error {
		marked = errText
		return nil
	}

	runID := "11111111-1111-1111-1111-111111111111"
	w := &Worker{}
	job := jobs.ClaimedJob{
		ID:              "job-1",
		ExperimentRunID: &runID,
		Payload:         []byte("{"),
	}

	if err := w.handleExperimentRun(context.Background(), job); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if marked != "invalid payload json" {
		t.Fatalf("unexpected mark error: %q", marked)
	}
}

func TestHandleExperimentRun_SuccessPublishesTask(t *testing.T) {
	defer restoreExperimentRunHooks()()

	runID := "11111111-1111-1111-1111-111111111111"
	expID := "22222222-2222-2222-2222-222222222222"
	dsID := "507f1f77bcf86cd799439011"

	payload, _ := json.Marshal(map[string]string{
		"experiment_id": expID,
		"dataset_id":    dsID,
		"run_id":        runID,
	})

	var progresses []int
	updateProgressFn = func(_ *Worker, _ context.Context, _ string, progress int) error {
		progresses = append(progresses, progress)
		return nil
	}

	loadExperimentFn = func(_ *Worker, _ context.Context, experimentID string) (string, *int64, json.RawMessage, string, error) {
		if experimentID != expID {
			t.Fatalf("unexpected experiment id: %q", experimentID)
		}
		return "algo", nil, json.RawMessage(`{"ballot_format":"ranking","tally_rule":"plurality"}`), "", nil
	}

	loadDatasetMetaFn = func(_ *Worker, _ context.Context, datasetHex string) (DatasetInfo, string, error) {
		if datasetHex != dsID {
			t.Fatalf("unexpected dataset id: %q", datasetHex)
		}
		return DatasetInfo{
			ID:     dsID,
			Name:   "dataset",
			Format: "ranking",
			Candidates: []DatasetCandidate{
				{ID: "c1", Name: "Alice"},
			},
		}, "", nil
	}

	var markedRunID, kernelTaskID string
	markRunRunningFn = func(_ *Worker, _ context.Context, rid, kid string) error {
		markedRunID = rid
		kernelTaskID = kid
		return nil
	}

	var published kafka.Message
	writeTaskMessageFn = func(_ context.Context, _ *Worker, msg kafka.Message) error {
		published = msg
		return nil
	}

	w := &Worker{}
	job := jobs.ClaimedJob{
		ID:              "job-1",
		ExperimentRunID: &runID,
		Payload:         payload,
	}

	if err := w.handleExperimentRun(context.Background(), job); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if markedRunID != runID {
		t.Fatalf("expected markRunRunning run_id=%s got %s", runID, markedRunID)
	}
	if kernelTaskID != "job-1" {
		t.Fatalf("expected kernel task id job-1 got %s", kernelTaskID)
	}
	if string(published.Key) != runID {
		t.Fatalf("expected kafka key=%s got %q", runID, string(published.Key))
	}

	var task ExperimentRunTask
	if err := json.Unmarshal(published.Value, &task); err != nil {
		t.Fatalf("unmarshal task: %v", err)
	}
	if task.RunID != runID || task.ExperimentID != expID || task.DatasetID != dsID {
		t.Fatalf("unexpected task payload: %#v", task)
	}

	if len(progresses) != 3 || progresses[0] != 15 || progresses[1] != 30 || progresses[2] != 45 {
		t.Fatalf("unexpected progresses: %#v", progresses)
	}
}

func TestConsumeResults_ExperimentRunResultKind_AppliesAndCommits(t *testing.T) {
	defer restoreExperimentRunHooks()()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	msg := kafka.Message{
		Topic:     "secure-voting.compute.results",
		Partition: 0,
		Offset:    1,
		Key:       []byte("11111111-1111-1111-1111-111111111111"),
		Value:     []byte(`{"kind":"experiment_run_result","run_id":"11111111-1111-1111-1111-111111111111","status":"done","winners":["c1"]}`),
	}

	calls := 0
	fetchResultMessageFn = func(_ context.Context, _ *Worker) (kafka.Message, error) {
		if calls == 0 {
			calls++
			return msg, nil
		}
		cancel()
		return kafka.Message{}, context.Canceled
	}

	applied := false
	applyExperimentRunResultFn = func(_ context.Context, _ *Worker, res ExperimentRunResult) error {
		applied = true
		if res.Kind != experimentRunResultKind {
			t.Fatalf("unexpected kind: %q", res.Kind)
		}
		return nil
	}

	commits := 0
	commitResultFn = func(_ context.Context, _ *Worker, _ kafka.Message) error {
		commits++
		return nil
	}

	err := (&Worker{}).consumeResults(ctx)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context canceled, got %v", err)
	}
	if !applied {
		t.Fatal("expected apply to be called")
	}
	if commits != 1 {
		t.Fatalf("expected 1 commit, got %d", commits)
	}
}

func TestConsumeResults_BadJSON_CommitsAndSkips(t *testing.T) {
	defer restoreExperimentRunHooks()()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	msg := kafka.Message{
		Topic:     "secure-voting.compute.results",
		Partition: 0,
		Offset:    2,
		Value:     []byte("{"),
	}

	calls := 0
	fetchResultMessageFn = func(_ context.Context, _ *Worker) (kafka.Message, error) {
		if calls == 0 {
			calls++
			return msg, nil
		}
		cancel()
		return kafka.Message{}, context.Canceled
	}

	applyExperimentRunResultFn = func(_ context.Context, _ *Worker, _ ExperimentRunResult) error {
		t.Fatal("apply should not be called")
		return nil
	}

	commits := 0
	commitResultFn = func(_ context.Context, _ *Worker, _ kafka.Message) error {
		commits++
		return nil
	}

	err := (&Worker{}).consumeResults(ctx)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context canceled, got %v", err)
	}
	if commits != 1 {
		t.Fatalf("expected 1 commit, got %d", commits)
	}
}

func TestConsumeResults_ApplyError_DoesNotCommit(t *testing.T) {
	defer restoreExperimentRunHooks()()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	msg := kafka.Message{
		Topic:     "secure-voting.compute.results",
		Partition: 0,
		Offset:    3,
		Key:       []byte("11111111-1111-1111-1111-111111111111"),
		Value:     []byte(`{"kind":"experiment_run_result","run_id":"11111111-1111-1111-1111-111111111111","status":"done","winners":["c1"]}`),
	}

	calls := 0
	fetchResultMessageFn = func(_ context.Context, _ *Worker) (kafka.Message, error) {
		if calls == 0 {
			calls++
			return msg, nil
		}
		cancel()
		return kafka.Message{}, context.Canceled
	}

	applyExperimentRunResultFn = func(_ context.Context, _ *Worker, _ ExperimentRunResult) error {
		return errors.New("boom")
	}

	commits := 0
	commitResultFn = func(_ context.Context, _ *Worker, _ kafka.Message) error {
		commits++
		return nil
	}

	err := (&Worker{}).consumeResults(ctx)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context canceled, got %v", err)
	}
	if commits != 0 {
		t.Fatalf("expected 0 commits, got %d", commits)
	}
}
