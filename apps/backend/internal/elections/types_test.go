package elections

import "testing"

func TestValidateTallyRuleCanonicalValues(t *testing.T) {
	t.Parallel()

	cases := []string{
		"plurality",
		"approval",
		"inverse_plurality",
		"borda",
		"black",
		"copeland_i",
		"copeland_ii",
		"copeland_iii",
		"simpson",
		"minmax",
		"hare",
		"inverse_borda",
		"nanson",
		"coombs",
		"practical_condorcet",
		"threshold",
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc, func(t *testing.T) {
			t.Parallel()

			got, ok := validateTallyRule(tc)
			if !ok {
				t.Fatalf("validateTallyRule(%q) returned ok=false", tc)
			}
			if got != tc {
				t.Fatalf("validateTallyRule(%q) = %q, want %q", tc, got, tc)
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

func TestValidateTallyRuleRejectsUnknown(t *testing.T) {
	t.Parallel()

	cases := []string{
		"",
		"   ",
		"minimaxx",
		"condorcet",
		"strong_q_paretian_plurality",
		"totally_unknown_rule",
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
