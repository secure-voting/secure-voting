package worker

import (
	"context"
	"strings"

	"secure-voting/apps/backend/internal/jobs"
)

var loadElectionRouteMetaFn = func(w *Worker, ctx context.Context, electionID string) (string, string, error) {
	var ballotFormat string
	var tallyRule string

	err := w.db.QueryRow(ctx, `
		SELECT ballot_format, tally_rule
		FROM elections
		WHERE id = $1::uuid
	`, strings.TrimSpace(electionID)).Scan(&ballotFormat, &tallyRule)
	if err != nil {
		return "", "", err
	}

	return ballotFormat, tallyRule, nil
}

var handleElectionTallyExternalFn = func(w *Worker, ctx context.Context, job jobs.ClaimedJob) error {
	return w.handleElectionTallyExternal(ctx, job)
}

func (w *Worker) handleTallyJob(ctx context.Context, job jobs.ClaimedJob) error {
	if job.ElectionID == nil || strings.TrimSpace(*job.ElectionID) == "" {
		return handleTallyLocalFn(w, ctx, job)
	}

	ballotFormat, tallyRule, err := loadElectionRouteMetaFn(w, ctx, strings.TrimSpace(*job.ElectionID))
	if err != nil {
		return err
	}

	if supportsExternalElectionTally(ballotFormat, tallyRule) {
		return handleElectionTallyExternalFn(w, ctx, job)
	}

	return handleTallyLocalFn(w, ctx, job)
}
