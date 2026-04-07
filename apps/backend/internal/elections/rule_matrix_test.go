package elections

import (
	"testing"

	"secure-voting/apps/backend/internal/computeclient"
)

func mockRules() []computeclient.TallyRuleInfo {
	return []computeclient.TallyRuleInfo{
		{
			ID:                       "plurality",
			BallotFormats:            []string{"ranking"},
			RequiresCommitteeSize:    true,
			SupportsQuotaType:        false,
			RequiresApprovalMaxChoices: false,
			SupportsRankingTopK:      true,
			RequiresScoreRange:       false,
		},
		{
			ID:                       "approval",
			BallotFormats:            []string{"approval"},
			RequiresCommitteeSize:    true,
			SupportsQuotaType:        false,
			RequiresApprovalMaxChoices: true,
			SupportsRankingTopK:      false,
			RequiresScoreRange:       false,
		},
		{
			ID:                       "score_rule",
			BallotFormats:            []string{"score"},
			RequiresCommitteeSize:    true,
			SupportsQuotaType:        false,
			RequiresApprovalMaxChoices: false,
			SupportsRankingTopK:      false,
			RequiresScoreRange:       true,
		},
	}
}

func TestValidateRuleCompatibility_OK(t *testing.T) {
	rules := mockRules()

	err := validateRuleCompatibility(
		"plurality",
		"ranking",
		map[string]any{
			"committee_size": 3,
		},
		rules,
	)

	if err != nil {
		t.Fatalf("expected ok, got %v", err)
	}
}

func TestValidateRuleCompatibility_InvalidFormat(t *testing.T) {
	rules := mockRules()

	err := validateRuleCompatibility(
		"plurality",
		"score",
		map[string]any{
			"committee_size": 3,
		},
		rules,
	)

	if err == nil {
		t.Fatalf("expected error")
	}
}

func TestValidateRuleCompatibility_MissingApprovalParam(t *testing.T) {
	rules := mockRules()

	err := validateRuleCompatibility(
		"approval",
		"approval",
		map[string]any{
			"committee_size": 3,
		},
		rules,
	)

	if err == nil {
		t.Fatalf("expected error")
	}
}

func TestValidateRuleCompatibility_ScoreMissingRange(t *testing.T) {
	rules := mockRules()

	err := validateRuleCompatibility(
		"score_rule",
		"score",
		map[string]any{
			"committee_size": 3,
		},
		rules,
	)

	if err == nil {
		t.Fatalf("expected error")
	}
}

func TestValidateRuleCompatibility_UnsupportedTopK(t *testing.T) {
	rules := mockRules()

	err := validateRuleCompatibility(
		"approval",
		"approval",
		map[string]any{
			"committee_size": 3,
			"ranking_top_k":  2,
			"approval_max_choices": 2,
		},
		rules,
	)

	if err == nil {
		t.Fatalf("expected error")
	}
}