package jobs

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Runner struct {
	db *pgxpool.Pool
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
