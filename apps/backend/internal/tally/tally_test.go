package tally

import "testing"

func TestApprovalTopK(t *testing.T) {
	cands := []string{"a", "b", "c"}
	ballots := [][]string{
		{"a", "b"},
		{"a"},
		{"b"},
		{"c"},
		{"a"},
	}
	out := computeApproval(1, cands, ballots)
	if len(out.Winners) != 1 || out.Winners[0] != "a" {
		t.Fatalf("expected winner a, got %+v", out.Winners)
	}
}

func TestPluralityTop2(t *testing.T) {
	cands := []string{"a", "b", "c"}
	ballots := [][]string{
		{"b", "a"},
		{"b"},
		{"a"},
		{"c"},
		{"a"},
	}
	out := computePlurality(2, cands, ballots)
	if len(out.Winners) != 2 {
		t.Fatalf("expected 2 winners, got %+v", out.Winners)
	}
	if out.Winners[0] != "a" && out.Winners[0] != "b" {
		t.Fatalf("unexpected top winner: %+v", out.Winners)
	}
}

func TestBorda(t *testing.T) {
	cands := []string{"a", "b", "c"}
	ballots := [][]string{
		{"a", "b", "c"},
		{"b", "a", "c"},
		{"b", "c", "a"},
	}
	out := computeBorda(1, cands, ballots)
	if len(out.Winners) != 1 || out.Winners[0] != "b" {
		t.Fatalf("expected winner b, got %+v", out.Winners)
	}
}
