package datasets

import "testing"

func TestNormalizeFormatAndIsValidFormat_More(t *testing.T) {
	if got := normalizeFormat("  RANKING "); got != "ranking" {
		t.Fatalf("unexpected normalizeFormat: %q", got)
	}

	valid := []string{"approval", "ranking", "score", "  score "}
	for _, v := range valid {
		if !isValidFormat(v) {
			t.Fatalf("expected valid format for %q", v)
		}
	}

	invalid := []string{"", "unknown", "rank"}
	for _, v := range invalid {
		if isValidFormat(v) {
			t.Fatalf("expected invalid format for %q", v)
		}
	}
}

func TestValidateCandidates_More(t *testing.T) {
	if code := validateCandidates(nil); code != "invalid_candidates" {
		t.Fatalf("unexpected code: %q", code)
	}

	if code := validateCandidates([]Candidate{{ID: "", Name: "Alice"}}); code != "invalid_candidate_id" {
		t.Fatalf("unexpected code: %q", code)
	}

	if code := validateCandidates([]Candidate{{ID: "c1", Name: ""}}); code != "invalid_candidate_name" {
		t.Fatalf("unexpected code: %q", code)
	}

	if code := validateCandidates([]Candidate{
		{ID: "c1", Name: "Alice"},
		{ID: "c1", Name: "Bob"},
	}); code != "duplicate_candidate_id" {
		t.Fatalf("unexpected code: %q", code)
	}

	if code := validateCandidates([]Candidate{
		{ID: "c1", Name: "Alice"},
		{ID: "c2", Name: "Bob"},
	}); code != "" {
		t.Fatalf("unexpected code: %q", code)
	}
}

func TestIntFromAny_More(t *testing.T) {
	casesOK := []struct {
		in   any
		want int
	}{
		{int(1), 1},
		{int8(2), 2},
		{int16(3), 3},
		{int32(4), 4},
		{int64(5), 5},
		{float64(6), 6},
	}

	for _, tc := range casesOK {
		got, ok := intFromAny(tc.in)
		if !ok || got != tc.want {
			t.Fatalf("intFromAny(%#v) = (%d,%v), want (%d,true)", tc.in, got, ok, tc.want)
		}
	}

	if _, ok := intFromAny(float64(1.5)); ok {
		t.Fatal("expected float64(1.5) to be rejected")
	}
	if _, ok := intFromAny("7"); ok {
		t.Fatal("expected string to be rejected")
	}
}

func TestValidateDatasetParams_Approval_More(t *testing.T) {
	if code := validateDatasetParams("approval", map[string]any{
		"approval_max_choices": 0,
	}, 3, false); code != "invalid_approval_max_choices" {
		t.Fatalf("unexpected code: %q", code)
	}

	if code := validateDatasetParams("approval", map[string]any{
		"approval_max_choices": 5,
	}, 3, false); code != "invalid_approval_max_choices" {
		t.Fatalf("unexpected code: %q", code)
	}

	if code := validateDatasetParams("approval", map[string]any{
		"approval_max_choices": 2,
	}, 3, false); code != "" {
		t.Fatalf("unexpected code: %q", code)
	}
}

func TestValidateDatasetParams_Ranking_More(t *testing.T) {
	if code := validateDatasetParams("ranking", map[string]any{
		"ranking_top_k": 0,
	}, 3, false); code != "invalid_ranking_top_k" {
		t.Fatalf("unexpected code: %q", code)
	}

	if code := validateDatasetParams("ranking", map[string]any{
		"ranking_top_k": 4,
	}, 3, false); code != "invalid_ranking_top_k" {
		t.Fatalf("unexpected code: %q", code)
	}

	if code := validateDatasetParams("ranking", map[string]any{
		"ranking_top_k": 2,
	}, 3, false); code != "" {
		t.Fatalf("unexpected code: %q", code)
	}
}

func TestValidateDatasetParams_Score_More(t *testing.T) {
	if code := validateDatasetParams("score", nil, 3, true); code != "score_rules_missing" {
		t.Fatalf("unexpected code: %q", code)
	}

	if code := validateDatasetParams("score", map[string]any{
		"score_min":  0,
		"score_max":  5,
		"score_step": 0,
	}, 3, true); code != "score_rules_invalid_step" {
		t.Fatalf("unexpected code: %q", code)
	}

	if code := validateDatasetParams("score", map[string]any{
		"score_min":  5,
		"score_max":  0,
		"score_step": 1,
	}, 3, true); code != "score_rules_invalid_range" {
		t.Fatalf("unexpected code: %q", code)
	}

	if code := validateDatasetParams("score", map[string]any{
		"score_min":  0,
		"score_max":  5,
		"score_step": 2,
	}, 3, true); code != "score_rules_invalid_step" {
		t.Fatalf("unexpected code: %q", code)
	}

	if code := validateDatasetParams("score", map[string]any{
		"score_min":  0,
		"score_max":  6,
		"score_step": 2,
	}, 3, true); code != "" {
		t.Fatalf("unexpected code: %q", code)
	}

	if code := validateDatasetParams("score", nil, 3, false); code != "" {
		t.Fatalf("unexpected code when rules not required: %q", code)
	}
}
