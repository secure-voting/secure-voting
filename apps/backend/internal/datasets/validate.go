package datasets

import "strings"

func normalizeFormat(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}

func isValidFormat(s string) bool {
	switch normalizeFormat(s) {
	case "approval", "ranking", "score":
		return true
	default:
		return false
	}
}

func validateCandidates(candidates []Candidate) string {
	if len(candidates) == 0 {
		return "invalid_candidates"
	}

	seen := map[string]struct{}{}
	for _, c := range candidates {
		id := strings.TrimSpace(c.ID)
		name := strings.TrimSpace(c.Name)

		if id == "" {
			return "invalid_candidate_id"
		}
		if name == "" {
			return "invalid_candidate_name"
		}
		if _, ok := seen[id]; ok {
			return "duplicate_candidate_id"
		}
		seen[id] = struct{}{}
	}

	return ""
}

func intFromAny(v any) (int, bool) {
	switch n := v.(type) {
	case int:
		return n, true
	case int8:
		return int(n), true
	case int16:
		return int(n), true
	case int32:
		return int(n), true
	case int64:
		return int(n), true
	case float64:
		if float64(int(n)) == n {
			return int(n), true
		}
		return 0, false
	default:
		return 0, false
	}
}

func validateDatasetParams(format string, params map[string]any, candidateCount int, requireScoreRules bool) string {
	format = normalizeFormat(format)
	if params == nil {
		params = map[string]any{}
	}

	switch format {
	case "approval":
		if v, ok := params["approval_max_choices"]; ok {
			n, ok := intFromAny(v)
			if !ok || n <= 0 || n > candidateCount {
				return "invalid_approval_max_choices"
			}
		}

	case "ranking":
		if v, ok := params["ranking_top_k"]; ok {
			n, ok := intFromAny(v)
			if !ok || n <= 0 || n > candidateCount {
				return "invalid_ranking_top_k"
			}
		}

	case "score":
		minV, hasMin := params["score_min"]
		maxV, hasMax := params["score_max"]
		stepV, hasStep := params["score_step"]

		if requireScoreRules && (!hasMin || !hasMax || !hasStep) {
			return "score_rules_missing"
		}
		if !hasMin && !hasMax && !hasStep {
			return ""
		}
		if !hasMin || !hasMax || !hasStep {
			return "score_rules_missing"
		}

		minN, ok1 := intFromAny(minV)
		maxN, ok2 := intFromAny(maxV)
		stepN, ok3 := intFromAny(stepV)
		if !ok1 || !ok2 || !ok3 {
			return "score_rules_invalid"
		}
		if stepN <= 0 {
			return "score_rules_invalid_step"
		}
		if minN > maxN {
			return "score_rules_invalid_range"
		}
		if ((maxN - minN) % stepN) != 0 {
			return "score_rules_invalid_step"
		}
	}

	return ""
}