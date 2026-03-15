package worker

import (
	"context"
	"encoding/json"
	"strings"

	"secure-voting/apps/backend/internal/jobs"
	"secure-voting/apps/backend/internal/tally"
)

func (w *Worker) handleTallyLocal(ctx context.Context, job jobs.ClaimedJob) error {
	if job.ElectionID == nil || strings.TrimSpace(*job.ElectionID) == "" {
		_ = w.runner.MarkError(ctx, job.ID, "missing election_id in jobs row")
		return nil
	}
	eid := strings.TrimSpace(*job.ElectionID)

	_ = w.runner.UpdateProgress(ctx, job.ID, 20)

	out, code, err := tally.ComputeFromDB(ctx, w.db, eid)
	if err != nil {
		_ = w.runner.MarkError(ctx, job.ID, "tally compute error: "+err.Error())
		return nil
	}
	if code != "" {
		_ = w.runner.MarkError(ctx, job.ID, "tally failed: "+code)
		return nil
	}

	_ = w.runner.UpdateProgress(ctx, job.ID, 70)

	winnersJSON, _ := json.Marshal(out.Winners)
	paramsJSON, _ := json.Marshal(out.Params)
	metricsJSON, _ := json.Marshal(out.Metrics)
	protocolJSON, _ := json.Marshal(out.Protocol)

	tx, err := w.db.Begin(ctx)
	if err != nil {
		_ = w.runner.MarkError(ctx, job.ID, "db begin failed")
		return nil
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var resultID string
	var version int
	err = tx.QueryRow(ctx, `
WITH nextv AS (
	SELECT COALESCE(MAX(version),0)+1 AS v
	FROM results
	WHERE election_id=$1::uuid
)
INSERT INTO results (election_id, version, method, params, winners, metrics, protocol)
SELECT $1::uuid, nextv.v, $2, $3::jsonb, $4::jsonb, $5::jsonb, $6::jsonb
FROM nextv
RETURNING id::text, version
`, eid, out.Method,
		string(paramsJSON),
		string(winnersJSON),
		string(metricsJSON),
		string(protocolJSON),
	).Scan(&resultID, &version)
	if err != nil {
		_ = w.runner.MarkError(ctx, job.ID, "insert results failed: "+err.Error())
		return nil
	}

	_, err = tx.Exec(ctx, `
UPDATE elections
SET status='results_ready'
WHERE id=$1::uuid AND status='closed'
`, eid)
	if err != nil {
		_ = w.runner.MarkError(ctx, job.ID, "update election status failed: "+err.Error())
		return nil
	}

	ref := map[string]any{
		"result_id":   resultID,
		"version":     version,
		"election_id": eid,
	}
	refJSON, _ := json.Marshal(ref)

	_, err = tx.Exec(ctx, `
UPDATE jobs
SET status='done', progress=100, finished_at=now(), error_text=NULL, result_ref=$2::jsonb
WHERE id=$1::uuid
`, job.ID, string(refJSON))
	if err != nil {
		_ = w.runner.MarkError(ctx, job.ID, "update job failed: "+err.Error())
		return nil
	}

	if err := tx.Commit(ctx); err != nil {
		_ = w.runner.MarkError(ctx, job.ID, "commit failed: "+err.Error())
		return nil
	}

	return nil
}
