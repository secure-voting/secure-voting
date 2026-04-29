package elections

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"secure-voting/apps/backend/internal/computeclient"
)

func (s *Service) Create(ctx context.Context, createdBy string, in CreateElectionInput) (string, string, error) {
	in.Title = strings.TrimSpace(in.Title)
	if in.Title == "" {
		return "", "invalid_title", nil
	}

	var description *string
	if in.Description != nil {
		v := strings.TrimSpace(*in.Description)
		if v != "" {
			description = &v
		}
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

	normalizedCandidates, err := extractNormalizedCandidates(in.Candidates, in.CandidateNames)
	if err != nil {
		return "", candidateNormalizationCode(err), nil
	}

	candidateCount := len(normalizedCandidates)

	var rules []computeclient.TallyRuleInfo
	if s.capabilities != nil {
		rules, err = s.capabilities.ListTallyRules(ctx)
		if err != nil {
			return "", "", err
		}

		if !validateKnownTallyRule(tally, rules) {
			return "", "invalid_tally_rule", nil
		}
	}

	committeeRequired := ruleRequiresCommitteeSize(tally, rules, s.capabilities == nil)
	committeeSize, err := normalizeCommitteeSize(committeeRequired, in.CommitteeSize, candidateCount)
	if err != nil {
		return "", committeeSizeCode(err), nil
	}

	var quotaType *string
	if committeeSize != nil && *committeeSize > 1 {
		if in.QuotaType == nil {
			return "", "quota_type_required", nil
		}

		q := norm(*in.QuotaType)
		if !allowedQuotaTypes[q] {
			return "", "invalid_quota_type", nil
		}
		quotaType = &q
	}

	rankingTopK, err := normalizeRankingTopK(format, in.RankingTopK, candidateCount)
	if err != nil {
		return "", rankingTopKCode(err), nil
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
		}
	}

	if code := validateBallotParams(
		format,
		candidateCount,
		in.ApprovalMaxChoices,
		rankingTopK,
		in.ScoreMin,
		in.ScoreMax,
		in.ScoreStep,
	); code != "" {
		return "", code, nil
	}

	params := map[string]any{
		"committee_size":       committeeSize,
		"quota_type":           quotaType,
		"approval_max_choices": in.ApprovalMaxChoices,
		"ranking_top_k":        rankingTopK,
		"score_min":            in.ScoreMin,
		"score_max":            in.ScoreMax,
		"score_step":           in.ScoreStep,
	}

	if s.capabilities != nil {
		if err := validateRuleCompatibility(
			tally,
			format,
			params,
			rules,
		); err != nil {
			return "", err.Error(), nil
		}
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
			$1, $2, $3, $4, $5, $6,
			$7, $8,
			'draft', $9,
			$10, $11,
			$12, $13,
			$14, $15, $16, $17,
			$18
		)
		RETURNING id::text
	`,
		in.Title,
		description,
		startAt,
		endAt,
		tally,
		format,
		committeeSize,
		quotaType,
		access,
		publishAt,
		in.ShowAggregates,
		in.ApprovalMaxChoices,
		rankingTopK,
		in.ScoreMin,
		in.ScoreMax,
		in.ScoreStep,
		in.ScoreAllowSkip,
		createdBy,
	).Scan(&electionID)
	if err != nil {
		return "", "", err
	}

	for _, c := range normalizedCandidates {
		var metaJSON []byte
		if len(c.Meta) > 0 {
			metaJSON, err = json.Marshal(c.Meta)
			if err != nil {
				return "", "", err
			}
		}

		_, err = tx.Exec(ctx, `
			INSERT INTO candidates (election_id, name, meta)
			VALUES ($1::uuid, $2, $3::jsonb)
		`, electionID, c.Name, nullableJSON(metaJSON))
		if err != nil {
			return "", "", err
		}
	}

	_ = insertAudit(ctx, tx, &createdBy, "election_created", map[string]any{
		"target_type": "election",
		"target_id":   electionID,
		"after": map[string]any{
			"title":         in.Title,
			"tally_rule":    tally,
			"ballot_format": format,
			"status":        "draft",
		},
	})

	if err := tx.Commit(ctx); err != nil {
		return "", "", err
	}

	return electionID, "", nil
}
