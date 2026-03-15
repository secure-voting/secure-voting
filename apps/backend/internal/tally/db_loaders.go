package tally

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

func loadApprovalBallots(ctx context.Context, db *pgxpool.Pool, electionID string) ([][]string, string, error) {
	return loadStringArrayBallots(ctx, db, `
SELECT approval_set
FROM ballots
WHERE election_id=$1::uuid AND status='accepted'
`, electionID)
}

func loadRankingBallots(ctx context.Context, db *pgxpool.Pool, electionID string) ([][]string, string, error) {
	return loadStringArrayBallots(ctx, db, `
SELECT ranking
FROM ballots
WHERE election_id=$1::uuid AND status='accepted'
`, electionID)
}

func loadStringArrayBallots(ctx context.Context, db *pgxpool.Pool, query string, electionID string) ([][]string, string, error) {
	rows, err := db.Query(ctx, query, electionID)
	if err != nil {
		return nil, "", err
	}
	defer rows.Close()

	out := make([][]string, 0, 128)
	for rows.Next() {
		var raw []byte
		if err := rows.Scan(&raw); err != nil {
			return nil, "", err
		}

		if len(raw) == 0 || string(raw) == "null" {
			continue
		}

		var arr []string
		if err := json.Unmarshal(raw, &arr); err != nil {
			return nil, CodeBadBallotData, nil
		}

		clean := make([]string, 0, len(arr))
		for _, v := range arr {
			v = strings.TrimSpace(v)
			if v != "" {
				clean = append(clean, v)
			}
		}
		if len(clean) > 0 {
			out = append(out, clean)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, "", err
	}

	if len(out) == 0 {
		return nil, CodeNoBallots, nil
	}
	return out, "", nil
}
