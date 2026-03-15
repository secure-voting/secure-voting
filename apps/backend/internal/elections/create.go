package elections

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

func (s *Service) Create(ctx context.Context, createdBy string, in CreateElectionInput) (string, string, error) {
	in.Title = strings.TrimSpace(in.Title)
	if in.Title == "" {
		return "", "invalid_title", nil
	}

	startAt, err := time.Parse(time.RFC3339, strings.TrimSpace(in.StartAt))
	if err != nil {
		return "", "invalid_start_at", nil
	}
	endAt, err := time.Parse(time.RFC3339, strings.TrimSpace(in.EndAt))
	if err != nil {
		return "", "invalid_end_at", nil
	}
	if !startAt.Before(endAt) {
		return "", "invalid_time_range", nil
	}

	tally, ok := validateTallyRule(in.TallyRule)
	if !ok {
		return "", "invalid_tally_rule", nil
	}
	format := norm(in.BallotFormat)
	if !allowedBallotFormats[format] {
		return "", "invalid_ballot_format", nil
	}
	access := norm(in.AccessMode)
	if !allowedAccessModes[access] {
		return "", "invalid_access_mode", nil
	}

	if len(in.Candidates) == 0 {
		return "", "candidates_required", nil
	}
	seen := make(map[string]struct{}, len(in.Candidates))
	for _, c := range in.Candidates {
		name := strings.TrimSpace(c.Name)
		if name == "" {
			return "", "invalid_candidate_name", nil
		}
		key := norm(name)
		if _, exists := seen[key]; exists {
			return "", "duplicate_candidate_name", nil
		}
		seen[key] = struct{}{}
	}

	if in.CommitteeSize != nil && *in.CommitteeSize <= 0 {
		return "", "invalid_committee_size", nil
	}
	if in.CommitteeSize != nil && *in.CommitteeSize > 1 {
		if in.QuotaType == nil {
			return "", "quota_type_required", nil
		}
		qt := norm(*in.QuotaType)
		if !allowedQuotaTypes[qt] {
			return "", "invalid_quota_type", nil
		}
		in.QuotaType = &qt
	} else {
		in.QuotaType = nil
	}

	var publishAt *time.Time
	if in.PublishAt != nil {
		p := strings.TrimSpace(*in.PublishAt)
		if p != "" {
			t, err := time.Parse(time.RFC3339, p)
			if err != nil {
				return "", "invalid_publish_at", nil
			}
			publishAt = &t
		} else {
			publishAt = nil
		}
	}

	code := validateBallotParams(format, len(in.Candidates), in.ApprovalMaxChoices, in.RankingTopK, in.ScoreMin, in.ScoreMax, in.ScoreStep)
	if code != "" {
		return "", code, nil
	}

	tx, err := s.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return "", "", err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var electionID string
	err = tx.QueryRow(ctx, `
		INSERT INTO elections (
			title, description, start_at, end_at, tally_rule, ballot_format,
			committee_size, quota_type,
			status, access_mode,
			publish_at, show_aggregates,
			approval_max_choices, ranking_top_k,
			score_min, score_max, score_step, score_allow_skip,
			created_by
		) VALUES (
			$1,$2,$3,$4,$5,$6,
			$7,$8,
			'draft',$9,
			$10,$11,
			$12,$13,
			$14,$15,$16,$17,
			$18
		)
		RETURNING id::text
	`, in.Title, in.Description, startAt, endAt, tally, format,
		in.CommitteeSize, in.QuotaType,
		access,
		publishAt, in.ShowAggregates,
		in.ApprovalMaxChoices, in.RankingTopK,
		in.ScoreMin, in.ScoreMax, in.ScoreStep, in.ScoreAllowSkip,
		createdBy,
	).Scan(&electionID)
	if err != nil {
		return "", "", err
	}

	for _, c := range in.Candidates {
		var metaJSON []byte
		if c.Meta != nil {
			metaJSON, err = json.Marshal(c.Meta)
			if err != nil {
				return "", "", err
			}
		}
		_, err = tx.Exec(ctx, `
			INSERT INTO candidates (election_id, name, meta)
			VALUES ($1::uuid, $2, $3::jsonb)
		`, electionID, strings.TrimSpace(c.Name), nullableJSON(metaJSON))
		if err != nil {
			return "", "", err
		}
	}

	_ = insertAudit(ctx, tx, &createdBy, "election_created", map[string]any{
		"target_type": "election",
		"target_id":   electionID,
		"after": map[string]any{
			"title": in.Title,
		},
	})

	if err := tx.Commit(ctx); err != nil {
		return "", "", err
	}

	return electionID, "", nil
}
