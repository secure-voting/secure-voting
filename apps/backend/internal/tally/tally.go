package tally

import (
	"context"
	"encoding/json"
	"errors"
	"sort"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Output struct {
	Method   string                 `json:"method"`
	Params   map[string]any         `json:"params,omitempty"`
	Winners  []string               `json:"winners"`
	Metrics  map[string]any         `json:"metrics,omitempty"`
	Protocol map[string]any         `json:"protocol,omitempty"`
}

func ComputeFromDB(ctx context.Context, db *pgxpool.Pool, electionID string) (Output, string, error) {
	if _, err := uuid.Parse(electionID); err != nil {
		return Output{}, "invalid_id", nil
	}

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
			return Output{}, "not_found", nil
		}
		return Output{}, "", err
	}

	rule = strings.TrimSpace(strings.ToLower(rule))
	format = strings.TrimSpace(strings.ToLower(format))

	cs := 1
	if committeeSize != nil && *committeeSize > 0 {
		cs = *committeeSize
	}

	rows, err := db.Query(ctx, `SELECT id::text FROM candidates WHERE election_id=$1::uuid ORDER BY id`, electionID)
	if err != nil {
		return Output{}, "", err
	}
	defer rows.Close()

	cands := make([]string, 0, 16)
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return Output{}, "", err
		}
		cands = append(cands, id)
	}
	if len(cands) == 0 {
		return Output{}, "no_candidates", nil
	}

	switch rule {
	case "approval":
		if format != "approval" {
			return Output{}, "unsupported_rule_for_format", nil
		}
		ballots, code, err := loadApprovalBallots(ctx, db, electionID)
		if err != nil || code != "" {
			return Output{}, code, err
		}
		out := computeApproval(cs, cands, ballots)
		out.Method = "approval"
		out.Params = map[string]any{"committee_size": cs, "quota_type": quotaType}
		return out, "", nil

	case "plurality":
		if format != "ranking" {
			return Output{}, "unsupported_rule_for_format", nil
		}
		ballots, code, err := loadRankingBallots(ctx, db, electionID)
		if err != nil || code != "" {
			return Output{}, code, err
		}
		out := computePlurality(cs, cands, ballots)
		out.Method = "plurality"
		out.Params = map[string]any{"committee_size": cs, "quota_type": quotaType}
		return out, "", nil

	case "borda":
		if format != "ranking" {
			return Output{}, "unsupported_rule_for_format", nil
		}
		ballots, code, err := loadRankingBallots(ctx, db, electionID)
		if err != nil || code != "" {
			return Output{}, code, err
		}
		out := computeBorda(cs, cands, ballots)
		out.Method = "borda"
		out.Params = map[string]any{"committee_size": cs, "quota_type": quotaType}
		return out, "", nil

	default:
		return Output{}, "unsupported_tally_rule", nil
	}
}

func loadApprovalBallots(ctx context.Context, db *pgxpool.Pool, electionID string) ([][]string, string, error) {
	rows, err := db.Query(ctx, `
		SELECT approval_set
		FROM ballots
		WHERE election_id=$1::uuid AND status='accepted'
	`, electionID)
	if err != nil {
		return nil, "", err
	}
	defer rows.Close()

	var out [][]string
	for rows.Next() {
		var raw []byte
		if err := rows.Scan(&raw); err != nil {
			return nil, "", err
		}
		var arr []string
		if len(raw) > 0 && string(raw) != "null" {
			if err := json.Unmarshal(raw, &arr); err != nil {
				return nil, "bad_ballot_data", nil
			}
		}
		if len(arr) > 0 {
			out = append(out, arr)
		}
	}
	if len(out) == 0 {
		return nil, "no_ballots", nil
	}
	return out, "", nil
}

func loadRankingBallots(ctx context.Context, db *pgxpool.Pool, electionID string) ([][]string, string, error) {
	rows, err := db.Query(ctx, `
		SELECT ranking
		FROM ballots
		WHERE election_id=$1::uuid AND status='accepted'
	`, electionID)
	if err != nil {
		return nil, "", err
	}
	defer rows.Close()

	var out [][]string
	for rows.Next() {
		var raw []byte
		if err := rows.Scan(&raw); err != nil {
			return nil, "", err
		}
		var arr []string
		if len(raw) > 0 && string(raw) != "null" {
			if err := json.Unmarshal(raw, &arr); err != nil {
				return nil, "bad_ballot_data", nil
			}
		}
		if len(arr) > 0 {
			out = append(out, arr)
		}
	}
	if len(out) == 0 {
		return nil, "no_ballots", nil
	}
	return out, "", nil
}

func computeApproval(committeeSize int, candidates []string, ballots [][]string) Output {
	score := make(map[string]int, len(candidates))
	for _, cid := range candidates {
		score[cid] = 0
	}
	for _, b := range ballots {
		for _, cid := range b {
			if _, ok := score[cid]; ok {
				score[cid]++
			}
		}
	}
	winners := topK(score, committeeSize)
	return Output{
		Winners: winners,
		Metrics: map[string]any{
			"ballots": len(ballots),
		},
		Protocol: map[string]any{
			"scores": score,
		},
	}
}

func computePlurality(committeeSize int, candidates []string, ballots [][]string) Output {
	score := make(map[string]int, len(candidates))
	for _, cid := range candidates {
		score[cid] = 0
	}
	for _, b := range ballots {
		if len(b) == 0 {
			continue
		}
		first := b[0]
		if _, ok := score[first]; ok {
			score[first]++
		}
	}
	winners := topK(score, committeeSize)
	return Output{
		Winners: winners,
		Metrics: map[string]any{
			"ballots": len(ballots),
		},
		Protocol: map[string]any{
			"scores": score,
		},
	}
}

func computeBorda(committeeSize int, candidates []string, ballots [][]string) Output {
	n := len(candidates)
	score := make(map[string]int, n)
	for _, cid := range candidates {
		score[cid] = 0
	}
	for _, b := range ballots {
		for i, cid := range b {
			if _, ok := score[cid]; !ok {
				continue
			}
			points := (n - 1) - i
			if points < 0 {
				points = 0
			}
			score[cid] += points
		}
	}
	winners := topK(score, committeeSize)
	return Output{
		Winners: winners,
		Metrics: map[string]any{
			"ballots": len(ballots),
		},
		Protocol: map[string]any{
			"scores": score,
		},
	}
}

func topK(score map[string]int, k int) []string {
	type pair struct {
		ID    string
		Score int
	}
	items := make([]pair, 0, len(score))
	for id, sc := range score {
		items = append(items, pair{ID: id, Score: sc})
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].Score != items[j].Score {
			return items[i].Score > items[j].Score
		}
		return items[i].ID < items[j].ID
	})
	if k <= 0 {
		k = 1
	}
	if k > len(items) {
		k = len(items)
	}
	out := make([]string, 0, k)
	for i := 0; i < k; i++ {
		out = append(out, items[i].ID)
	}
	return out
}
