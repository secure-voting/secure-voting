package elections

import (
	"testing"

	"secure-voting/apps/backend/internal/computeclient"
)

func mockRules() []computeclient.TallyRuleInfo {
	return []computeclient.TallyRuleInfo{
		{
			ID:                         "plurality",
			BallotFormats:              []string{"ranking"},
			SupportsElectionTally:      true,
			RequiresCommitteeSize:      true,
			SupportsQuotaType:          false,
			RequiresApprovalMaxChoices: false,
			SupportsRankingTopK:        true,
			RequiresScoreRange:         false,
		},
		{
			ID:                         "approval",
			BallotFormats:              []string{"approval"},
			SupportsElectionTally:      true,
			RequiresCommitteeSize:      true,
			SupportsQuotaType:          false,
			RequiresApprovalMaxChoices: true,
			SupportsRankingTopK:        false,
			RequiresScoreRange:         false,
		},
		{
			ID:                         "score_rule",
			BallotFormats:              []string{"score"},
			SupportsElectionTally:      true,
			RequiresCommitteeSize:      true,
			SupportsQuotaType:          false,
			RequiresApprovalMaxChoices: false,
			SupportsRankingTopK:        false,
			RequiresScoreRange:         true,
		},
		{
			ID:                         "experiment_only_rule",
			BallotFormats:              []string{"ranking"},
			SupportsElectionTally:      false,
			RequiresCommitteeSize:      true,
			SupportsQuotaType:          false,
			RequiresApprovalMaxChoices: false,
			SupportsRankingTopK:        true,
			RequiresScoreRange:         false,
		},
		{
			ID:                         "minmax",
			BallotFormats:              []string{"ranking"},
			SupportsElectionTally:      true,
			RequiresCommitteeSize:      true,
			SupportsQuotaType:          false,
			RequiresApprovalMaxChoices: false,
			SupportsRankingTopK:        true,
			RequiresScoreRange:         false,
		},
	}
}

func TestValidateRuleCompatibility_OK_WithPointerParams(t *testing.T) {
	rules := mockRules()

	err := validateRuleCompatibility(
		"plurality",
		"ranking",
		map[string]any{
			"committee_size": intPtr(3),
			"ranking_top_k":  intPtr(2),
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
			"committee_size": intPtr(3),
		},
		rules,
	)

	if err == nil {
		t.Fatalf("expected error")
	}
	if err != ErrIncompatibleBallotFormat {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateRuleCompatibility_MissingApprovalParam(t *testing.T) {
	rules := mockRules()

	err := validateRuleCompatibility(
		"approval",
		"approval",
		map[string]any{
			"committee_size": intPtr(3),
		},
		rules,
	)

	if err == nil {
		t.Fatalf("expected error")
	}
	if err != ErrMissingApprovalMaxChoices {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateRuleCompatibility_ScoreMissingRange(t *testing.T) {
	rules := mockRules()

	err := validateRuleCompatibility(
		"score_rule",
		"score",
		map[string]any{
			"committee_size": intPtr(3),
		},
		rules,
	)

	if err == nil {
		t.Fatalf("expected error")
	}
	if err != ErrInvalidScoreRange {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateRuleCompatibility_UnsupportedTopK_WithPointerParam(t *testing.T) {
	rules := mockRules()

	err := validateRuleCompatibility(
		"approval",
		"approval",
		map[string]any{
			"committee_size":       intPtr(3),
			"approval_max_choices": intPtr(2),
			"ranking_top_k":        intPtr(2),
		},
		rules,
	)

	if err == nil {
		t.Fatalf("expected error")
	}
	if err != ErrUnsupportedTopK {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateRuleCompatibility_UnsupportedElectionTally(t *testing.T) {
	rules := mockRules()

	err := validateRuleCompatibility(
		"experiment_only_rule",
		"ranking",
		map[string]any{
			"committee_size": intPtr(3),
		},
		rules,
	)

	if err == nil {
		t.Fatalf("expected error")
	}
	if err != ErrUnsupportedElectionTally {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateRuleCompatibility_AliasLookupUsesCanonicalMatrix(t *testing.T) {
	rules := mockRules()

	err := validateRuleCompatibility(
		"minimax",
		"ranking",
		map[string]any{
			"committee_size": intPtr(2),
			"ranking_top_k":  intPtr(2),
		},
		rules,
	)

	if err != nil {
		t.Fatalf("expected ok, got %v", err)
	}
}