package datasets

import (
	"context"
	"testing"
)

func TestGenerate_ScoreRulesMissing_ReturnsValidationCodeBeforeDBAccess(t *testing.T) {
	svc := &Service{}

	_, code, err := svc.Generate(context.Background(), GenerateReq{
		Name:   "score-dataset",
		Format: "score",
		Voters: 3,
		Candidates: []Candidate{
			{ID: "c1", Name: "A"},
			{ID: "c2", Name: "B"},
		},
	})

	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if code != "score_rules_missing" {
		t.Fatalf("expected score_rules_missing, got %q", code)
	}
}

func TestGenerate_InvalidCandidateName_ReturnsValidationCodeBeforeDBAccess(t *testing.T) {
	svc := &Service{}

	_, code, err := svc.Generate(context.Background(), GenerateReq{
		Name:   "ranking-dataset",
		Format: "ranking",
		Voters: 2,
		Candidates: []Candidate{
			{ID: "c1", Name: ""},
		},
	})

	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if code != "invalid_candidate_name" {
		t.Fatalf("expected invalid_candidate_name, got %q", code)
	}
}
