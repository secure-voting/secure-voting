package jobs

import (
	"context"
	"errors"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"
)

func (r *Runner) ClaimNext(ctx context.Context, kinds []string) (ClaimedJob, bool, error) {
	kinds = normalizeKinds(kinds)
	if len(kinds) == 0 {
		return ClaimedJob{}, false, errors.New("jobs: kinds required")
	}

	placeholders := make([]string, 0, len(kinds))
	args := make([]any, 0, len(kinds))
	for i, k := range kinds {
		placeholders = append(placeholders, "$"+strconv.Itoa(i+1))
		args = append(args, k)
	}

	q := `
WITH cte AS (
	SELECT id
	FROM jobs
	WHERE status = 'queued'
	  AND kind IN (` + strings.Join(placeholders, ",") + `)
	ORDER BY created_at ASC
	FOR UPDATE SKIP LOCKED
	LIMIT 1
)
UPDATE jobs j
SET status='running', started_at=now(), progress=0
FROM cte
WHERE j.id = cte.id
RETURNING
	j.id::text, j.kind, j.status, j.progress,
	j.created_by::text,
	j.election_id::text, j.experiment_id::text, j.experiment_run_id::text,
	COALESCE(j.payload,'{}'::jsonb),
	j.created_at
`

	var out ClaimedJob
	var payload []byte

	err := r.db.QueryRow(ctx, q, args...).Scan(
		&out.ID, &out.Kind, &out.Status, &out.Progress,
		&out.CreatedBy,
		&out.ElectionID, &out.ExperimentID, &out.ExperimentRunID,
		&payload,
		&out.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ClaimedJob{}, false, nil
		}
		return ClaimedJob{}, false, err
	}

	out.Payload = payload
	return out, true, nil
}
