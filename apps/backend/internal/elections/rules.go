package elections

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func (s *Service) UpdateRules(ctx context.Context, electionID, adminUserID string, in UpdateRulesInput) (string, error) {
	if _, err := uuid.Parse(electionID); err != nil {
		return "invalid_id", nil
	}

	var curStatus string
	var curTally, curFormat, curAccess string
	var curCommittee *int
	var curQuota *string
	var curPublishAt *time.Time
	var curShowAgg bool
	var curApproval *int
	var curTopK *int
	var curScoreMin *int
	var curScoreMax *int
	var curScoreStep *int
	var curScoreAllowSkip bool

	err := s.db.QueryRow(ctx, `
		SELECT status, tally_rule, ballot_format,
		       committee_size, quota_type,
		       access_mode, publish_at, show_aggregates,
		       approval_max_choices, ranking_top_k,
		       score_min, score_max, score_step, score_allow_skip
		FROM elections
		WHERE id=$1::uuid AND created_by=$2::uuid
	`, electionID, adminUserID).Scan(
		&curStatus, &curTally, &curFormat,
		&curCommittee, &curQuota,
		&curAccess, &curPublishAt, &curShowAgg,
		&curApproval, &curTopK,
		&curScoreMin, &curScoreMax, &curScoreStep, &curScoreAllowSkip,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "not_found", nil
		}
		return "", err
	}

	if curStatus != "draft" && curStatus != "scheduled" {
		return "invalid_status", nil
	}

	var candCount int
	if err := s.db.QueryRow(ctx, `SELECT count(*) FROM candidates WHERE election_id=$1::uuid`, electionID).Scan(&candCount); err != nil {
		return "", err
	}

	finalTally := curTally
	if in.TallyRule != nil {
		t, ok := validateTallyRule(*in.TallyRule)
		if !ok {
			return "invalid_tally_rule", nil
		}
		finalTally = t
	}

	finalFormat := curFormat
	if in.BallotFormat != nil {
		f := norm(*in.BallotFormat)
		if !allowedBallotFormats[f] {
			return "invalid_ballot_format", nil
		}
		finalFormat = f
	}

	finalCommittee := curCommittee
	if in.CommitteeSize != nil {
		if *in.CommitteeSize <= 0 {
			return "invalid_committee_size", nil
		}
		v := *in.CommitteeSize
		finalCommittee = &v
	}

	finalQuota := curQuota
	if in.QuotaType != nil {
		q := norm(*in.QuotaType)
		if q == "" || !allowedQuotaTypes[q] {
			return "invalid_quota_type", nil
		}
		finalQuota = &q
	}

	cs := 1
	if finalCommittee != nil {
		cs = *finalCommittee
	}
	if cs > 1 {
		if finalQuota == nil {
			return "quota_type_required", nil
		}
	} else {
		finalQuota = nil
	}

	finalAccess := curAccess
	if in.AccessMode != nil {
		a := norm(*in.AccessMode)
		if !allowedAccessModes[a] {
			return "invalid_access_mode", nil
		}
		finalAccess = a
	}

	finalPublishAt := curPublishAt
	if in.PublishAt != nil {
		p := strings.TrimSpace(*in.PublishAt)
		if p == "" {
			finalPublishAt = nil
		} else {
			t, err := time.Parse(time.RFC3339, p)
			if err != nil {
				return "invalid_publish_at", nil
			}
			finalPublishAt = &t
		}
	}

	finalShowAgg := curShowAgg
	if in.ShowAggregates != nil {
		finalShowAgg = *in.ShowAggregates
	}

	finalApproval := curApproval
	if in.ApprovalMaxChoices != nil {
		v := *in.ApprovalMaxChoices
		finalApproval = &v
	}
	finalTopK := curTopK
	if in.RankingTopK != nil {
		v := *in.RankingTopK
		finalTopK = &v
	}
	finalScoreMin := curScoreMin
	if in.ScoreMin != nil {
		v := *in.ScoreMin
		finalScoreMin = &v
	}
	finalScoreMax := curScoreMax
	if in.ScoreMax != nil {
		v := *in.ScoreMax
		finalScoreMax = &v
	}
	finalScoreStep := curScoreStep
	if in.ScoreStep != nil {
		v := *in.ScoreStep
		finalScoreStep = &v
	}
	finalScoreAllowSkip := curScoreAllowSkip
	if in.ScoreAllowSkip != nil {
		finalScoreAllowSkip = *in.ScoreAllowSkip
	}

	if code := validateBallotParams(finalFormat, candCount, finalApproval, finalTopK, finalScoreMin, finalScoreMax, finalScoreStep); code != "" {
		return code, nil
	}

	_, err = s.db.Exec(ctx, `
	UPDATE elections SET
	tally_rule = $2,
	ballot_format = $3,
	committee_size = $4,
	quota_type = $5,
	access_mode = $6,
	publish_at = $7,
	show_aggregates = $8,

	approval_max_choices = CASE WHEN $3 = 'approval' THEN $9  ELSE NULL END,
	ranking_top_k        = CASE WHEN $3 = 'ranking'  THEN $10 ELSE NULL END,

	score_min        = CASE WHEN $3 = 'score' THEN $11 ELSE NULL END,
	score_max        = CASE WHEN $3 = 'score' THEN $12 ELSE NULL END,
	score_step       = CASE WHEN $3 = 'score' THEN $13 ELSE NULL END,
	score_allow_skip = CASE WHEN $3 = 'score' THEN $14 ELSE false END
	WHERE id=$1::uuid AND created_by=$15::uuid
	`, electionID,
		finalTally, finalFormat,
		finalCommittee, finalQuota,
		finalAccess,
		finalPublishAt,
		finalShowAgg,
		finalApproval, finalTopK,
		finalScoreMin, finalScoreMax, finalScoreStep,
		finalScoreAllowSkip,
		adminUserID,
	)

	if err != nil {
		return "", err
	}

	return "", nil
}
