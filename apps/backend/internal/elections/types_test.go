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