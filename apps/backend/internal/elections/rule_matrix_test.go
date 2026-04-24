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
func TestBuildRuleMatrix_SkipsEmptyAndNormalizesAliases(t *testing.T) {
	rules := []computeclient.TallyRuleInfo{
		{ID: "plurality"},
		{ID: "  "},
		{ID: "minmax"},
		{ID: "practical_condorcet"},
	}

	m := buildRuleMatrix(rules)

	if _, ok := m.get("plurality"); !ok {
		t.Fatalf("expected plurality to be present")
	}
	if _, ok := m.get("minimax"); !ok {
		t.Fatalf("expected minimax alias to resolve to minmax")
	}
	if _, ok := m.get("condorcet_practical"); !ok {
		t.Fatalf("expected condorcet_practical alias to resolve to practical_condorcet")
	}
	if _, ok := m.get(""); ok {
		t.Fatalf("expected empty normalized rule to be skipped")
	}
}

func TestValidateRuleCompatibility_RuleMatrixContract_RankingRules(t *testing.T) {
	rules := []computeclient.TallyRuleInfo{
		{
			ID:                         "plurality",
			BallotFormats:              []string{"ranking"},
			SupportsElectionTally:      true,
			SupportsExperimentRuns:     true,
			RequiresCommitteeSize:      true,
			SupportsQuotaType:          false,
			RequiresApprovalMaxChoices: false,
			SupportsRankingTopK:        true,
			RequiresScoreRange:         false,
		},
		{
			ID:                         "hare",
			BallotFormats:              []string{"ranking"},
			SupportsElectionTally:      true,
			SupportsExperimentRuns:     true,
			RequiresCommitteeSize:      true,
			SupportsQuotaType:          true,
			RequiresApprovalMaxChoices: false,
			SupportsRankingTopK:        true,
			RequiresScoreRange:         false,
		},
		{
			ID:                         "minmax",
			BallotFormats:              []string{"ranking"},
			SupportsElectionTally:      true,
			SupportsExperimentRuns:     true,
			RequiresCommitteeSize:      true,
			SupportsQuotaType:          false,
			RequiresApprovalMaxChoices: false,
			SupportsRankingTopK:        true,
			RequiresScoreRange:         false,
		},
	}

	cases := []struct {
		name   string
		rule   string
		params map[string]any
	}{
		{
			name: "plurality with ranking top k",
			rule: "plurality",
			params: map[string]any{
				"committee_size": intPtr(3),
				"ranking_top_k":  intPtr(2),
			},
		},
		{
			name: "hare with quota",
			rule: "hare",
			params: map[string]any{
				"committee_size": intPtr(3),
				"ranking_top_k":  intPtr(3),
				"quota_type":     strPtr("hare"),
			},
		},
		{
			name: "minimax alias",
			rule: "minimax",
			params: map[string]any{
				"committee_size": intPtr(2),
				"ranking_top_k":  intPtr(2),
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := validateRuleCompatibility(tc.rule, "ranking", tc.params, rules); err != nil {
				t.Fatalf("expected ok, got %v", err)
			}
		})
	}
}

func TestValidateRuleCompatibility_RuleMatrixContract_RejectsWrongBallotFormat(t *testing.T) {
	rules := []computeclient.TallyRuleInfo{
		{
			ID:                         "plurality",
			BallotFormats:              []string{"ranking"},
			SupportsElectionTally:      true,
			SupportsExperimentRuns:     true,
			RequiresCommitteeSize:      true,
			SupportsQuotaType:          false,
			RequiresApprovalMaxChoices: false,
			SupportsRankingTopK:        true,
			RequiresScoreRange:         false,
		},
		{
			ID:                         "hare",
			BallotFormats:              []string{"ranking"},
			SupportsElectionTally:      true,
			SupportsExperimentRuns:     true,
			RequiresCommitteeSize:      true,
			SupportsQuotaType:          true,
			RequiresApprovalMaxChoices: false,
			SupportsRankingTopK:        true,
			RequiresScoreRange:         false,
		},
	}

	cases := []struct {
		name   string
		rule   string
		format string
		params map[string]any
	}{
		{
			name:   "plurality rejects approval",
			rule:   "plurality",
			format: "approval",
			params: map[string]any{
				"committee_size": intPtr(3),
			},
		},
		{
			name:   "hare rejects score",
			rule:   "hare",
			format: "score",
			params: map[string]any{
				"committee_size": intPtr(3),
				"score_min":      intPtr(0),
				"score_max":      intPtr(5),
				"score_step":     intPtr(1),
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateRuleCompatibility(tc.rule, tc.format, tc.params, rules)
			if err == nil {
				t.Fatalf("expected error")
			}
			if err != ErrIncompatibleBallotFormat {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func strPtr(v string) *string { return &v }