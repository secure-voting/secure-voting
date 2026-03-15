package experiments

import "testing"

func TestValidateParams_OK(t *testing.T) {
	params := map[string]any{
		"ballot_format":        "ranking",
		"tally_rule":           "borda",
		"committee_size":       float64(3),
		"ranking_top_k":        float64(3),
		"show_aggregates":      true,
		"score_min":            float64(0),
		"score_max":            float64(5),
		"score_step":           float64(1),
	}

	if code := validateParams(params); code != "" {
		t.Fatalf("expected empty code, got %q", code)
	}
}

func TestValidateParams_InvalidBallotFormat(t *testing.T) {
	params := map[string]any{
		"ballot_format": "weird-format",
	}

	if code := validateParams(params); code != "invalid_ballot_format" {
		t.Fatalf("expected invalid_ballot_format, got %q", code)
	}
}

func TestValidateParams_InvalidTallyRule(t *testing.T) {
	params := map[string]any{
		"tally_rule": "totally-unknown-rule",
	}

	if code := validateParams(params); code != "invalid_tally_rule" {
		t.Fatalf("expected invalid_tally_rule, got %q", code)
	}
}

func TestValidateParams_InvalidScoreRange(t *testing.T) {
	params := map[string]any{
		"score_min": float64(5),
		"score_max": float64(1),
	}

	if code := validateParams(params); code != "invalid_score_range" {
		t.Fatalf("expected invalid_score_range, got %q", code)
	}
}

func TestValidateParams_InvalidPositiveIntegerField(t *testing.T) {
	params := map[string]any{
		"committee_size": float64(0),
	}

	if code := validateParams(params); code != "invalid_committee_size" {
		t.Fatalf("expected invalid_committee_size, got %q", code)
	}
}