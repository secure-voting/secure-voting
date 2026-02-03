package ballots

import (
	"testing"

	"github.com/google/uuid"
)

func TestValidateIdempotencyKey(t *testing.T) {
	if validateIdempotencyKey("") {
		t.Fatalf("expected empty key to be invalid")
	}
	if validateIdempotencyKey("   ") {
		t.Fatalf("expected spaces-only key to be invalid")
	}
	if !validateIdempotencyKey("abc-DEF_123:ok.test@x") {
		t.Fatalf("expected key to be valid")
	}

	long := make([]byte, 201)
	for i := range long {
		long[i] = 'a'
	}
	if validateIdempotencyKey(string(long)) {
		t.Fatalf("expected too long key to be invalid")
	}

	if validateIdempotencyKey("bad key with spaces") {
		t.Fatalf("expected key with spaces to be invalid")
	}
	if validateIdempotencyKey("bad/char") {
		t.Fatalf("expected key with slash to be invalid")
	}
}

func TestValidateApprovalBallot(t *testing.T) {
	c1 := uuid.NewString()
	c2 := uuid.NewString()
	c3 := uuid.NewString()

	cset := map[string]struct{}{c1: {}, c2: {}, c3: {}}
	max := 2

	if code := validateApprovalBallot(nil, cset, &max); code == "" {
		t.Fatalf("expected invalid ballot")
	}
	if code := validateApprovalBallot([]string{c1, c2, c3}, cset, &max); code != "too_many_choices" {
		t.Fatalf("expected too_many_choices, got %s", code)
	}
	if code := validateApprovalBallot([]string{c1, c1}, cset, &max); code != "duplicate_candidate" {
		t.Fatalf("expected duplicate_candidate, got %s", code)
	}
	if code := validateApprovalBallot([]string{uuid.NewString()}, cset, &max); code != "invalid_candidate_id" {
		t.Fatalf("expected invalid_candidate_id, got %s", code)
	}
	if code := validateApprovalBallot([]string{c1, c2}, cset, &max); code != "" {
		t.Fatalf("expected ok, got %s", code)
	}
}

func TestValidateRankingBallot(t *testing.T) {
	c1 := uuid.NewString()
	c2 := uuid.NewString()
	c3 := uuid.NewString()

	cset := map[string]struct{}{c1: {}, c2: {}, c3: {}}
	topK := 2

	if code := validateRankingBallot(nil, cset, &topK); code == "" {
		t.Fatalf("expected invalid ballot")
	}
	if code := validateRankingBallot([]string{c1, c2, c3}, cset, &topK); code != "too_many_choices" {
		t.Fatalf("expected too_many_choices, got %s", code)
	}
	if code := validateRankingBallot([]string{c1, c1}, cset, &topK); code != "duplicate_candidate" {
		t.Fatalf("expected duplicate_candidate, got %s", code)
	}
	if code := validateRankingBallot([]string{uuid.NewString()}, cset, &topK); code != "invalid_candidate_id" {
		t.Fatalf("expected invalid_candidate_id, got %s", code)
	}
	if code := validateRankingBallot([]string{c2, c1}, cset, &topK); code != "" {
		t.Fatalf("expected ok, got %s", code)
	}
}

func TestValidateScoreBallot(t *testing.T) {
	c1 := uuid.NewString()
	c2 := uuid.NewString()

	candidates := []string{c1, c2}
	cset := map[string]struct{}{c1: {}, c2: {}}

	min := 1
	max := 5
	step := 1

	if code := validateScoreBallot(nil, candidates, cset, &min, &max, &step, true); code == "" {
		t.Fatalf("expected invalid ballot")
	}

	scores := map[string]int{c1: 3}
	if code := validateScoreBallot(scores, candidates, cset, &min, &max, &step, false); code != "score_missing_candidate" {
		t.Fatalf("expected score_missing_candidate, got %s", code)
	}

	scores2 := map[string]int{c1: 6}
	if code := validateScoreBallot(scores2, candidates, cset, &min, &max, &step, true); code != "score_out_of_range" {
		t.Fatalf("expected score_out_of_range, got %s", code)
	}

	step2 := 2
	scores3 := map[string]int{c1: 4} // 4 не попадает в шкалу 1,3,5 при step=2
	if code := validateScoreBallot(scores3, candidates, cset, &min, &max, &step2, true); code != "score_invalid_step" {
		t.Fatalf("expected score_invalid_step, got %s", code)
	}


	scores4 := map[string]int{c1: 3, c2: 5}
	if code := validateScoreBallot(scores4, candidates, cset, &min, &max, &step, false); code != "" {
		t.Fatalf("expected ok, got %s", code)
	}
}
