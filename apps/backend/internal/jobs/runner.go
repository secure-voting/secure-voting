package jobs

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Runner struct {
	db *pgxpool.Pool
}

func NewRunner(db *pgxpool.Pool) *Runner {
	return &Runner{db: db}
}

type ClaimedJob struct {
	ID              string          `json:"id"`
	Kind            string          `json:"kind"`
	Status          string          `json:"status"`
	Progress        int             `json:"progress"`
	CreatedBy       string          `json:"created_by"`
	ElectionID      *string         `json:"election_id,omitempty"`
	ExperimentID    *string         `json:"experiment_id,omitempty"`
	ExperimentRunID *string         `json:"experiment_run_id,omitempty"`
	Payload         json.RawMessage `json:"payload,omitempty"`
	CreatedAt       time.Time       `json:"-"`
}

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

func (r *Runner) UpdateProgress(ctx context.Context, jobID string, progress int) error {
	if progress < 0 {
		progress = 0
	}
	if progress > 100 {
		progress = 100
	}

	_, err := r.db.Exec(ctx, `
UPDATE jobs
SET progress = $2
WHERE id = $1::uuid
  AND status = 'running'
  AND progress <> $2
`, jobID, progress)
	return err
}

func (r *Runner) MarkDone(ctx context.Context, jobID string, resultRef map[string]any) error {
	var ref any = nil
	if resultRef != nil {
		b, err := json.Marshal(resultRef)
		if err != nil {
			return err
		}
		ref = string(b)
	}

	_, err := r.db.Exec(ctx, `
UPDATE jobs
SET status='done', progress=100, finished_at=now(), error_text=NULL, result_ref=$2::jsonb
WHERE id=$1::uuid
`, jobID, ref)
	return err
}

func (r *Runner) MarkError(ctx context.Context, jobID string, errorText string) error {
	errorText = strings.TrimSpace(errorText)
	if errorText == "" {
		errorText = "job failed"
	}

	_, err := r.db.Exec(ctx, `
UPDATE jobs
SET status='error', finished_at=now(), error_text=$2
WHERE id=$1::uuid
`, jobID, errorText)
	return err
}

func normalizeKinds(in []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(in))
	for _, k := range in {
		k = strings.TrimSpace(k)
		if k == "" {
			continue
		}
		if _, ok := seen[k]; ok {
			continue
		}
		seen[k] = struct{}{}
		out = append(out, k)
	}
	return out
}
