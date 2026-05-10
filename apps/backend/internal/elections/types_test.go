package elections

import (
	"testing"

	"secure-voting/apps/backend/internal/computeclient"
)

func TestValidateTallyRuleCanonicalValues(t *testing.T) {
	t.Parallel()

	cases := []struct {
		in   string
		want string
	}{
		{in: "plurality", want: "plurality"},
		{in: "approval", want: "approval"},
		{in: "inverse_plurality", want: "inverse_plurality"},
		{in: "borda", want: "borda"},
		{in: "black", want: "black"},
		{in: "copeland_i", want: "copeland_i"},
		{in: "copeland_ii", want: "copeland_ii"},
		{in: "copeland_iii", want: "copeland_iii"},
		{in: "simpson", want: "simpson"},
		{in: "minmax", want: "minmax"},
		{in: "hare", want: "hare"},
		{in: "inverse_borda", want: "inverse_borda"},
		{in: "nanson", want: "nanson"},
		{in: "coombs", want: "coombs"},
		{in: "practical_condorcet", want: "practical_condorcet"},
		{in: "threshold", want: "threshold"},
		{in: "strong_q_paretian_plurality", want: "strong_q_paretian_plurality"},
		{in: "strongest_q_paretian_simple_majority", want: "strongest_q_paretian_simple_majority"},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.in, func(t *testing.T) {
			t.Parallel()

			got, ok := validateTallyRule(tc.in)
			if !ok {
				t.Fatalf("validateTallyRule(%q) returned ok=false", tc.in)
			}
			if got != tc.want {
				t.Fatalf("validateTallyRule(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestValidateTallyRuleAliases(t *testing.T) {
	t.Parallel()

	cases := []struct {
		in   string
		want string
	}{
		{in: "minimax", want: "minmax"},
		{in: "condorcet_practical", want: "practical_condorcet"},
		{in: "condorcet-practical", want: "practical_condorcet"},
		{in: "  minimax  ", want: "minmax"},
		{in: "  condorcet_practical  ", want: "practical_condorcet"},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.in, func(t *testing.T) {
			t.Parallel()

			got, ok := validateTallyRule(tc.in)
			if !ok {
				t.Fatalf("validateTallyRule(%q) returned ok=false", tc.in)
			}
			if got != tc.want {
				t.Fatalf("validateTallyRule(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestValidateTallyRuleRejectsInvalidSyntax(t *testing.T) {
	t.Parallel()

	cases := []string{
		"",
		"   ",
		"bad rule",
		"bad-rule!",
		"1plurality",
		"_plurality",
		"rule.with.dot",
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc, func(t *testing.T) {
			t.Parallel()

			got, ok := validateTallyRule(tc)
			if ok {
				t.Fatalf("validateTallyRule(%q) = (%q, true), want rejected", tc, got)
			}
			if got != "" {
				t.Fatalf("validateTallyRule(%q) returned %q, want empty string", tc, got)
			}
		})
	}
}

func TestValidateKnownTallyRule_UsesCapabilitiesAsSourceOfTruth(t *testing.T) {
	t.Parallel()

	rules := []computeclient.TallyRuleInfo{
		{ID: "plurality"},
		{ID: "minmax"},
		{ID: "practical_condorcet"},
		{ID: "strong_q_paretian_plurality"},
	}

	cases := []struct {
		rule string
		want bool
	}{
		{rule: "plurality", want: true},
		{rule: "minimax", want: true},
		{rule: "condorcet_practical", want: true},
		{rule: "strong_q_paretian_plurality", want: true},
		{rule: "unknown_rule", want: false},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.rule, func(t *testing.T) {
			t.Parallel()

			got := validateKnownTallyRule(tc.rule, rules)
			if got != tc.want {
				t.Fatalf("validateKnownTallyRule(%q) = %v, want %v", tc.rule, got, tc.want)
			}
		})
	}
}

func TestValidateRuleCompatibility_AcceptsRuleKnownOnlyFromCapabilities(t *testing.T) {
	t.Parallel()

	committeeSize := 1
	rules := []computeclient.TallyRuleInfo{
		{
			ID:                    "new_dynamic_rule",
			Label:                 "New Dynamic Rule",
			BallotFormats:         []string{"ranking"},
			SupportsElectionTally: true,
			RequiresCommitteeSize: true,
			SupportsRankingTopK:   true,
		},
	}

	params := map[string]any{
		"committee_size": committeeSize,
		"ranking_top_k":  nil,
	}

	if err := validateRuleCompatibility(
		"new_dynamic_rule",
		"ranking",
		params,
		rules,
	); err != nil {
		t.Fatalf("expected dynamic rule from capabilities to be accepted, got %v", err)
	}
}

func TestNormalizeCommitteeSize_UsesCapabilityFlag(t *testing.T) {
	t.Parallel()

	size := 1

	got, err := normalizeCommitteeSize(true, &size, 3)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got == nil || *got != 1 {
		t.Fatalf("expected committee size 1, got %v", got)
	}

	got, err = normalizeCommitteeSize(false, &size, 3)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != nil {
		t.Fatalf("expected nil committee size when rule does not require it, got %v", *got)
	}
}

func TestNormalizeRankingTopK(t *testing.T) {
	t.Parallel()

	t.Run("non ranking clears value", func(t *testing.T) {
		t.Parallel()

		v := 5
		got, err := normalizeRankingTopK("approval", &v, 3)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != nil {
			t.Fatalf("expected nil, got %v", *got)
		}
	})

	t.Run("ranking nil is allowed", func(t *testing.T) {
		t.Parallel()

		got, err := normalizeRankingTopK("ranking", nil, 3)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != nil {
			t.Fatalf("expected nil, got %v", *got)
		}
	})

	t.Run("ranking clamps to candidates count", func(t *testing.T) {
		t.Parallel()

		v := 10
		got, err := normalizeRankingTopK("ranking", &v, 4)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got == nil || *got != 4 {
			t.Fatalf("expected 4, got %v", got)
		}
	})
}

func TestCanOpenElection(t *testing.T) {
	t.Parallel()

	if !canOpenElection("draft") {
		t.Fatal("draft should be openable")
	}
	if !canOpenElection("scheduled") {
		t.Fatal("scheduled should be openable")
	}
	if canOpenElection("active") {
		t.Fatal("active should not be openable")
	}
}
