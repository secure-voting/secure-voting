package elections

import (
	"testing"
)

func intPtr(v int) *int { return &v }

func TestNormalizeRuleName_More(t *testing.T) {
	cases := map[string]string{
		" minimax ":           "minmax",
		"condorcet_practical": "practical_condorcet",
		"plurality":           "plurality",
		"inverse-plurality":   "inverse_plurality",
	}

	for in, want := range cases {
		if got := normalizeRuleName(in); got != want {
			t.Fatalf("normalizeRuleName(%q)=%q want %q", in, got, want)
		}
	}
}

func TestNormalizeCommitteeSize_More(t *testing.T) {
	if _, err := normalizeCommitteeSize(true, nil, 3); err == nil {
		t.Fatal("expected required committee_size error")
	}
	if _, err := normalizeCommitteeSize(true, intPtr(0), 3); err == nil {
		t.Fatal("expected invalid committee_size error")
	}
	if _, err := normalizeCommitteeSize(true, intPtr(5), 3); err == nil {
		t.Fatal("expected too large committee_size error")
	}

	got, err := normalizeCommitteeSize(true, intPtr(2), 3)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got == nil || *got != 2 {
		t.Fatalf("unexpected result: %#v", got)
	}

	got, err = normalizeCommitteeSize(false, intPtr(2), 3)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != nil {
		t.Fatalf("expected nil when committee_size is not required, got %#v", got)
	}
}

func TestNormalizeRankingTopK_More(t *testing.T) {
	got, err := normalizeRankingTopK("approval", intPtr(2), 3)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != nil {
		t.Fatalf("expected nil for non-ranking, got %#v", got)
	}

	if _, err := normalizeRankingTopK("ranking", intPtr(0), 3); err == nil {
		t.Fatal("expected invalid ranking_top_k")
	}

	got, err = normalizeRankingTopK("ranking", intPtr(10), 3)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got == nil || *got != 3 {
		t.Fatalf("expected capped top_k=3, got %#v", got)
	}
}

func TestNormalizeCandidateNameAndDescription(t *testing.T) {
	if got := normalizeCandidateName("   Alice   Bob   "); got != "Alice Bob" {
		t.Fatalf("unexpected normalized name: %q", got)
	}
	if got := normalizeCandidateDescription("   hello  "); got != "hello" {
		t.Fatalf("unexpected normalized description: %q", got)
	}
}

func TestCandidateNormalizationCode(t *testing.T) {
	cases := map[string]string{
		"at least 2 candidates required":        "candidates_required",
		"candidate #1: empty name":              "invalid_candidate_name",
		"duplicate candidate name: Alice":       "duplicate_candidate_name",
		"candidate Alice: description too long": "invalid_candidate_description",
		"other":                                 "invalid_candidates",
	}

	for msg, want := range cases {
		if got := candidateNormalizationCode(assertErr(msg)); got != want {
			t.Fatalf("candidateNormalizationCode(%q)=%q want %q", msg, got, want)
		}
	}
}

func TestCommitteeSizeCode(t *testing.T) {
	cases := map[string]string{
		"at least 2 candidates required":                     "candidates_required",
		"committee_size is required for selected tally rule": "committee_size_required",
		"committee_size must be >= 1":                        "invalid_committee_size",
		"committee_size must be <= candidates count (3)":     "committee_size_too_large",
		"other": "invalid_committee_size",
	}

	for msg, want := range cases {
		if got := committeeSizeCode(assertErr(msg)); got != want {
			t.Fatalf("committeeSizeCode(%q)=%q want %q", msg, got, want)
		}
	}
}

func TestRankingTopKCode(t *testing.T) {
	cases := map[string]string{
		"candidate count must be >= 1": "candidates_required",
		"ranking_top_k must be >= 1":   "invalid_ranking_top_k",
		"other":                        "invalid_ranking_top_k",
	}

	for msg, want := range cases {
		if got := rankingTopKCode(assertErr(msg)); got != want {
			t.Fatalf("rankingTopKCode(%q)=%q want %q", msg, got, want)
		}
	}
}

func TestCanOpenElection_More(t *testing.T) {
	if !canOpenElection("draft") {
		t.Fatal("draft should be openable")
	}
	if !canOpenElection("scheduled") {
		t.Fatal("scheduled should be openable")
	}
	if canOpenElection("closed") {
		t.Fatal("closed should not be openable")
	}
}

func TestValidateBallotParams_ApprovalAndRanking(t *testing.T) {
	if got := validateBallotParams("approval", 0, intPtr(1), nil, nil, nil, nil); got != "candidates_required" {
		t.Fatalf("unexpected code: %q", got)
	}
	if got := validateBallotParams("approval", 3, nil, nil, nil, nil, nil); got != "approval_max_choices_required" {
		t.Fatalf("unexpected code: %q", got)
	}
	if got := validateBallotParams("approval", 3, intPtr(0), nil, nil, nil, nil); got != "invalid_approval_max_choices" {
		t.Fatalf("unexpected code: %q", got)
	}
	if got := validateBallotParams("approval", 3, intPtr(4), nil, nil, nil, nil); got != "approval_max_choices_too_large" {
		t.Fatalf("unexpected code: %q", got)
	}
	if got := validateBallotParams("approval", 3, intPtr(2), nil, nil, nil, nil); got != "" {
		t.Fatalf("unexpected code: %q", got)
	}

	if got := validateBallotParams("ranking", 3, nil, nil, nil, nil, nil); got != "" {
		t.Fatalf("unexpected code: %q", got)
	}
	if got := validateBallotParams("ranking", 3, nil, intPtr(0), nil, nil, nil); got != "invalid_ranking_top_k" {
		t.Fatalf("unexpected code: %q", got)
	}
	if got := validateBallotParams("ranking", 3, nil, intPtr(4), nil, nil, nil); got != "ranking_top_k_too_large" {
		t.Fatalf("unexpected code: %q", got)
	}
	if got := validateBallotParams("ranking", 3, nil, intPtr(2), nil, nil, nil); got != "" {
		t.Fatalf("unexpected code: %q", got)
	}
}

func assertErr(msg string) error {
	return testErr(msg)
}

type testErr string

func (e testErr) Error() string { return string(e) }
