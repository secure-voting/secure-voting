package worker

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/segmentio/kafka-go"

	"secure-voting/apps/backend/internal/jobs"
)

func TestHandleElectionTallyExternal_PublishesTask(t *testing.T) {
	oldLoadTask := loadElectionTallyTaskFn
	oldWriteTask := writeTaskMessageFn
	oldUpdateProgress := updateProgressFn
	defer func() {
		loadElectionTallyTaskFn = oldLoadTask
		writeTaskMessageFn = oldWriteTask
		updateProgressFn = oldUpdateProgress
	}()

	electionID := "44444444-4444-4444-4444-444444444444"
	job := jobs.ClaimedJob{
		ID:         "55555555-5555-5555-5555-555555555555",
		Kind:       jobKindTally,
		ElectionID: &electionID,
	}

	loadElectionTallyTaskFn = func(_ *Worker, _ context.Context, jobID, gotElectionID string) (ElectionTallyTask, error) {
		if jobID != job.ID {
			t.Fatalf("unexpected jobID: %s", jobID)
		}
		if gotElectionID != electionID {
			t.Fatalf("unexpected electionID: %s", gotElectionID)
		}

		return ElectionTallyTask{
			Kind:         electionTallyTaskKind,
			JobID:        job.ID,
			ElectionID:   electionID,
			TallyRule:    "plurality",
			BallotFormat: "ranking",
			Candidates: []ElectionCandidate{
				{ID: "c1", Name: "Alice"},
				{ID: "c2", Name: "Bob"},
			},
		}, nil
	}

	progresses := make([]int, 0, 2)
	updateProgressFn = func(_ *Worker, _ context.Context, jobID string, progress int) error {
		if jobID != job.ID {
			t.Fatalf("unexpected progress jobID: %s", jobID)
		}
		progresses = append(progresses, progress)
		return nil
	}
	var captured kafka.Message
	writeTaskMessageFn = func(_ context.Context, _ *Worker, msg kafka.Message) error {
		captured = msg
		return nil
	}

	w := &Worker{}
	if err := w.handleElectionTallyExternal(context.Background(), job); err != nil {
		t.Fatalf("handleElectionTallyExternal error: %v", err)
	}
	if string(captured.Key) != job.ID {
		t.Fatalf("unexpected kafka key: %q", string(captured.Key))
	}

	var task ElectionTallyTask
	if err := json.Unmarshal(captured.Value, &task); err != nil {
		t.Fatalf("unmarshal task: %v", err)
	}

	if task.Kind != electionTallyTaskKind {
		t.Fatalf("unexpected task kind: %q", task.Kind)
	}
	if task.JobID != job.ID {
		t.Fatalf("unexpected task job id: %q", task.JobID)
	}
	if task.ElectionID != electionID {
		t.Fatalf("unexpected task election id: %q", task.ElectionID)
	}
	if task.BallotFormat != "ranking" {
		t.Fatalf("unexpected ballot format: %q", task.BallotFormat)
	}
	if task.TallyRule != "plurality" {
		t.Fatalf("unexpected tally rule: %q", task.TallyRule)
	}
	if len(task.Candidates) != 2 {
		t.Fatalf("unexpected candidates len: %d", len(task.Candidates))
	}

	if len(progresses) != 2 || progresses[0] != 20 || progresses[1] != 45 {
		t.Fatalf("unexpected progresses: %#v", progresses)
	}
}
