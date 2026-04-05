package experiments

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

var allowedTypes = map[string]bool{
	"algo":     true,
	"behavior": true,
}

var allowedBallotFormats = map[string]bool{
	"approval": true,
	"ranking":  true,
	"score":    true,
}

var allowedQuotaTypes = map[string]bool{
	"hare":  true,
	"droop": true,
}

var allowedTallyRules = map[string]bool{
	"plurality":           true,
	"approval":            true,
	"inverse_plurality":   true,
	"borda":               true,
	"black":               true,
	"copeland_i":          true,
	"copeland_ii":         true,
	"copeland_iii":        true,
	"simpson":             true,
	"minmax":              true,
	"minimax":             true,
	"hare":                true,
	"inverse_borda":       true,
	"nanson":              true,
	"coombs":              true,
	"practical_condorcet": true,
	"condorcet_practical": true,
	"threshold":           true,
}

func norm(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}

func asInt64(v any) (int64, bool) {
	switch n := v.(type) {
	case int:
		return int64(n), true
	case int8:
		return int64(n), true
	case int16:
		return int64(n), true
	case int32:
		return int64(n), true
	case int64:
		return n, true
	case float64:
		if float64(int64(n)) == n {
			return int64(n), true
		}
		return 0, false
	default:
		return 0, false
	}
}

func validateParams(params map[string]any) string {
	if params == nil {
		return ""
	}

	if v, ok := params["ballot_format"]; ok {
		s, ok := v.(string)
		if !ok || !allowedBallotFormats[norm(s)] {
			return "invalid_ballot_format"
		}
	}

	if v, ok := params["quota_type"]; ok {
		s, ok := v.(string)
		if !ok || !allowedQuotaTypes[norm(s)] {
			return "invalid_quota_type"
		}
	}

	if v, ok := params["show_aggregates"]; ok {
		if _, ok := v.(bool); !ok {
			return "invalid_show_aggregates"
		}
	}

	ruleValue, hasRule := params["tally_rule"]
	if !hasRule {
		ruleValue, hasRule = params["rule"]
	}
	if hasRule {
		s, ok := ruleValue.(string)
		if !ok || !allowedTallyRules[norm(s)] {
			return "invalid_tally_rule"
		}
	}

	for _, key := range []string{"committee_size", "approval_max_choices", "ranking_top_k", "score_min", "score_max", "score_step"} {
		if v, ok := params[key]; ok {
			n, ok := asInt64(v)
			if !ok {
				return "invalid_params"
			}
			switch key {
			case "committee_size", "approval_max_choices", "ranking_top_k", "score_step":
				if n <= 0 {
					return "invalid_" + key
				}
			}
		}
	}

	if minV, okMin := params["score_min"]; okMin {
		if maxV, okMax := params["score_max"]; okMax {
			minN, ok1 := asInt64(minV)
			maxN, ok2 := asInt64(maxV)
			if !ok1 || !ok2 {
				return "invalid_params"
			}
			if minN > maxN {
				return "invalid_score_range"
			}
		}
	}

	return ""
}

func insertAudit(ctx context.Context, db *pgxpool.Pool, actorUserID, eventType string, details map[string]any) error {
	if strings.TrimSpace(actorUserID) == "" {
		return nil
	}
	if details == nil {
		details = map[string]any{}
	}

	b, err := json.Marshal(details)
	if err != nil {
		return err
	}

	_, err = db.Exec(ctx, `
		INSERT INTO audit_log (actor_user_id, event_type, details)
		VALUES ($1::uuid, $2, $3::jsonb)
	`, actorUserID, eventType, string(b))

	return err
}
