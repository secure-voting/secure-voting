package ballots

import (
	"strings"

	"github.com/google/uuid"
)

func validateIdempotencyKey(k string) bool {
	k = strings.TrimSpace(k)
	if k == "" {
		return false
	}
	if len(k) > 200 {
		return false
	}
	for _, r := range k {
		switch {
		case r >= 'a' && r <= 'z':
		case r >= 'A' && r <= 'Z':
		case r >= '0' && r <= '9':
		case r == '-' || r == '_' || r == ':' || r == '.' || r == '@':
		default:
			return false
		}
	}
	return true
}

func validateApprovalBallot(approvalSet []string, cset map[string]struct{}, maxChoices *int) string {
	if len(approvalSet) == 0 {
		return "invalid_ballot"
	}
	if maxChoices != nil && *maxChoices > 0 && len(approvalSet) > *maxChoices {
		return "too_many_choices"
	}

	seen := make(map[string]struct{}, len(approvalSet))
	for _, cid := range approvalSet {
		cid = strings.TrimSpace(cid)
		if _, err := uuid.Parse(cid); err != nil {
			return "invalid_candidate_id"
		}
		if _, ok := cset[cid]; !ok {
			return "invalid_candidate_id"
		}
		if _, ok := seen[cid]; ok {
			return "duplicate_candidate"
		}
		seen[cid] = struct{}{}
	}
	return ""
}

func validateRankingBallot(ranking []string, cset map[string]struct{}, topK *int) string {
	if len(ranking) == 0 {
		return "invalid_ballot"
	}
	if topK != nil && *topK > 0 && len(ranking) > *topK {
		return "too_many_choices"
	}

	seen := make(map[string]struct{}, len(ranking))
	for _, cid := range ranking {
		cid = strings.TrimSpace(cid)
		if _, err := uuid.Parse(cid); err != nil {
			return "invalid_candidate_id"
		}
		if _, ok := cset[cid]; !ok {
			return "invalid_candidate_id"
		}
		if _, ok := seen[cid]; ok {
			return "duplicate_candidate"
		}
		seen[cid] = struct{}{}
	}
	return ""
}

func validateScoreBallot(scores map[string]int, candidates []string, cset map[string]struct{}, scoreMin, scoreMax, scoreStep *int, allowSkip bool) string {
	if scores == nil || len(scores) == 0 {
		return "invalid_ballot"
	}
	if scoreMin == nil || scoreMax == nil || scoreStep == nil || *scoreStep <= 0 {
		return "score_rules_missing"
	}

	for cid, v := range scores {
		cid = strings.TrimSpace(cid)
		if _, err := uuid.Parse(cid); err != nil {
			return "invalid_candidate_id"
		}
		if _, ok := cset[cid]; !ok {
			return "invalid_candidate_id"
		}
		if v < *scoreMin || v > *scoreMax {
			return "score_out_of_range"
		}
		if (v-*scoreMin)%*scoreStep != 0 {
			return "score_invalid_step"
		}
	}

	if !allowSkip {
		for _, cid := range candidates {
			if _, ok := scores[cid]; !ok {
				return "score_missing_candidate"
			}
		}
	}

	return ""
}
