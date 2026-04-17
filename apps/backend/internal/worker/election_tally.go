package worker

import (
	"context"
	"encoding/json"
	"log"
	"strings"
	"time"

	"github.com/segmentio/kafka-go"

	"secure-voting/apps/backend/internal/jobs"
)

var loadElectionTallyTaskFn = func(w *Worker, ctx context.Context, jobID, electionID string) (ElectionTallyTask, error) {
	return w.loadElectionTallyTask(ctx, jobID, electionID)
}

func (w *Worker) handleElectionTallyExternal(ctx context.Context, job jobs.ClaimedJob) error {
	if job.ElectionID == nil || strings.TrimSpace(*job.ElectionID) == "" {
		_ = markJobErrorFn(w, ctx, job.ID, "missing election_id in jobs row")
		return nil
	}

	task, err := loadElectionTallyTaskFn(w, ctx, job.ID, strings.TrimSpace(*job.ElectionID))
	if err != nil {
		return err
	}

	if task.TallyRule == "" {
		_ = markJobErrorFn(w, ctx, job.ID, "invalid tally_rule in election config")
		return nil
	}
	if task.BallotFormat == "" {
		_ = markJobErrorFn(w, ctx, job.ID, "invalid ballot_format in election config")
		return nil
	}

	_ = updateProgressFn(w, ctx, job.ID, 20)

	value, err := json.Marshal(task)
	if err != nil {
		_ = markJobErrorFn(w, ctx, job.ID, "failed to marshal election_tally task")
		return nil
	}

	err = writeTaskMessageFn(ctx, w, kafka.Message{
		Key:   []byte(task.JobID),
		Value: value,
		Time:  time.Now().UTC(),
	})
	if err != nil {
		_ = markJobErrorFn(w, ctx, job.ID, "kafka publish failed: "+err.Error())
		return nil
	}

	log.Printf(
		"worker.handleElectionTallyExternal: published task job_id=%s election_id=%s tally_rule=%s ballot_format=%s",
		task.JobID,
		task.ElectionID,
		task.TallyRule,
		task.BallotFormat,
	)

	_ = updateProgressFn(w, ctx, job.ID, 45)
	return nil
}

func (w *Worker) loadElectionTallyTask(ctx context.Context, jobID, electionID string) (ElectionTallyTask, error) {
	var (
		task               ElectionTallyTask
		committeeSize      *int
		quotaType          *string
		approvalMaxChoices *int
		rankingTopK        *int
		scoreMin           *int
		scoreMax           *int
		scoreStep          *int
		scoreAllowSkip     bool
		showAggregates     bool
	)

	err := w.db.QueryRow(ctx, `
		SELECT tally_rule, ballot_format,
		       committee_size, quota_type,
		       approval_max_choices, ranking_top_k,
		       score_min, score_max, score_step, score_allow_skip,
		       show_aggregates
		FROM elections
		WHERE id = $1::uuid
	`, electionID).Scan(
		&task.TallyRule,
		&task.BallotFormat,
		&committeeSize,
		&quotaType,
		&approvalMaxChoices,
		&rankingTopK,
		&scoreMin,
		&scoreMax,
		&scoreStep,
		&scoreAllowSkip,
		&showAggregates,
	)
	if err != nil {
		return ElectionTallyTask{}, err
	}

	task.Kind = electionTallyTaskKind
	task.JobID = strings.TrimSpace(jobID)
	task.ElectionID = strings.TrimSpace(electionID)
	task.TallyRule = normalizeExternalTallyRule(task.TallyRule)
	task.BallotFormat = normalizeExternalBallotFormat(task.BallotFormat)
	task.CommitteeSize = committeeSize
	task.QuotaType = quotaType
	task.ApprovalMaxChoices = approvalMaxChoices
	task.RankingTopK = rankingTopK
	task.ScoreMin = scoreMin
	task.ScoreMax = scoreMax
	task.ScoreStep = scoreStep
	task.ScoreAllowSkip = scoreAllowSkip
	task.ShowAggregates = showAggregates

	rows, err := w.db.Query(ctx, `
		SELECT id::text, name
		FROM candidates
		WHERE election_id = $1::uuid
		ORDER BY name ASC, id ASC
	`, electionID)
	if err != nil {
		return ElectionTallyTask{}, err
	}
	defer rows.Close()

	task.Candidates = make([]ElectionCandidate, 0, 16)
	for rows.Next() {
		var c ElectionCandidate
		if err := rows.Scan(&c.ID, &c.Name); err != nil {
			return ElectionTallyTask{}, err
		}
		c.ID = strings.TrimSpace(c.ID)
		c.Name = strings.TrimSpace(c.Name)
		if c.ID == "" {
			continue
		}
		task.Candidates = append(task.Candidates, c)
	}
	if err := rows.Err(); err != nil {
		return ElectionTallyTask{}, err
	}

	return task, nil
}
