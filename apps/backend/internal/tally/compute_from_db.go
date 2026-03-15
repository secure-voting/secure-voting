package tally

import (
	"context"
	"errors"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type electionSettings struct {
	rule          string
	ballotFormat  string
	committeeSize int
	quotaType     any
}

func ComputeFromDB(ctx context.Context, db *pgxpool.Pool, electionID string) (Output, string, error) {
	if _, err := uuid.Parse(strings.TrimSpace(electionID)); err != nil {
		return Output{}, CodeInvalidID, nil
	}

	settings, code, err := loadElectionSettings(ctx, db, electionID)
	if err != nil || code != "" {
		return Output{}, code, err
	}

	cands, code, err := loadCandidateIDs(ctx, db, electionID)
	if err != nil || code != "" {
		return Output{}, code, err
	}

	h, ok := ruleHandlers[settings.rule]
	if !ok {
		return Output{}, CodeUnsupportedTallyRule, nil
	}
	if settings.ballotFormat != h.ballotFormat {
		return Output{}, CodeUnsupportedRuleForFormat, nil
	}

	ballots, code, err := h.loadBallots(ctx, db, electionID)
	if err != nil || code != "" {
		return Output{}, code, err
	}

	out := h.compute(settings.committeeSize, cands, ballots)
	out.Method = settings.rule
	out.Params = map[string]any{
		"committee_size": settings.committeeSize,
		"quota_type":     settings.quotaType,
	}

	return out, "", nil
}

func loadElectionSettings(ctx context.Context, db *pgxpool.Pool, electionID string) (electionSettings, string, error) {
	var rule, format string
	var committeeSize *int
	var quotaType *string

	err := db.QueryRow(ctx, `
SELECT tally_rule, ballot_format, committee_size, quota_type
FROM elections
WHERE id=$1::uuid
`, electionID).Scan(&rule, &format, &committeeSize, &quotaType)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return electionSettings{}, CodeNotFound, nil
		}
		return electionSettings{}, "", err
	}

	rule = normalizeLower(rule)
	format = normalizeLower(format)

	cs := 1
	if committeeSize != nil && *committeeSize > 0 {
		cs = *committeeSize
	}

	var qt any
	if quotaType != nil {
		v := strings.TrimSpace(*quotaType)
		if v != "" {
			qt = v
		}
	}

	return electionSettings{
		rule:          rule,
		ballotFormat:  format,
		committeeSize: cs,
		quotaType:     qt,
	}, "", nil
}

func loadCandidateIDs(ctx context.Context, db *pgxpool.Pool, electionID string) ([]string, string, error) {
	rows, err := db.Query(ctx, `SELECT id::text FROM candidates WHERE election_id=$1::uuid ORDER BY id`, electionID)
	if err != nil {
		return nil, "", err
	}
	defer rows.Close()

	cands := make([]string, 0, 16)
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, "", err
		}
		id = strings.TrimSpace(id)
		if id != "" {
			cands = append(cands, id)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, "", err
	}

	if len(cands) == 0 {
		return nil, CodeNoCandidates, nil
	}
	return cands, "", nil
}

func normalizeLower(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}
