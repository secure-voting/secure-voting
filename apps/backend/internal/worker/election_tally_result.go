package worker

import (
	"context"
	"encoding/json"
	"strings"
)

var applyElectionTallyResultFn = func(ctx context.Context, w *Worker, res ElectionTallyResult) error {
	return w.applyElectionTallyResult(ctx, res)
}

func (w *Worker) applyElectionTallyResult(ctx context.Context, res ElectionTallyResult) error {
	res.JobID = strings.TrimSpace(res.JobID)
	res.ElectionID = strings.TrimSpace(res.ElectionID)
	res.Status = strings.TrimSpace(res.Status)
	res.ErrorText = strings.TrimSpace(res.ErrorText)
	res.Method = strings.TrimSpace(res.Method)
	res.TallyRule = normalizeExternalRankingTallyRule(res.TallyRule)

	if res.Status == "error" {
		errText := res.ErrorText
		if errText == "" {
			errText = "external election tally failed"
		}
		return w.runner.MarkError(ctx, res.JobID, errText)
	}

	if res.Params == nil {
		res.Params = map[string]any{}
	}
	if res.Metrics == nil {
		res.Metrics = map[string]any{}
	}
	if res.Protocol == nil {
		res.Protocol = map[string]any{}
	}
	if res.Timings == nil {
		res.Timings = map[string]any{}
	}
	if res.Artifacts == nil {
		res.Artifacts = map[string]any{}
	}

	method := res.Method
	if method == "" {
		method = res.TallyRule
	}
	if method == "" {
		method = "ranking"
	}

	_ = w.runner.UpdateProgress(ctx, res.JobID, 70)

	winnersJSON, _ := json.Marshal(res.Winners)
	paramsJSON, _ := json.Marshal(res.Params)
	metricsJSON, _ := json.Marshal(res.Metrics)
	protocolJSON, _ := json.Marshal(res.Protocol)

	tx, err := w.db.Begin(ctx)
	if err != nil {
		return err
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
`, res.ElectionID, method,
		string(paramsJSON),
		string(winnersJSON),
		string(metricsJSON),
		string(protocolJSON),
	).Scan(&resultID, &version)
	if err != nil {
		return err
	}

	_, err = tx.Exec(ctx, `
UPDATE elections
SET status='results_ready'
WHERE id=$1::uuid AND status='closed'
`, res.ElectionID)
	if err != nil {
		return err
	}

	ref := map[string]any{
		"result_id":   resultID,
		"version":     version,
		"election_id": res.ElectionID,
	}
	refJSON, _ := json.Marshal(ref)

	_, err = tx.Exec(ctx, `
UPDATE jobs
SET status='done', progress=100, finished_at=now(), error_text=NULL, result_ref=$2::jsonb
WHERE id=$1::uuid
`, res.JobID, string(refJSON))
	if err != nil {
		return err
	}

	return tx.Commit(ctx)
}
