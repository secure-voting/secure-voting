package main

import "testing"

func TestNormalizeComputeTallyRuleIncludesScore(t *testing.T) {
	tests := map[string]string{
		"score": "score",
		"Score": "score",
		"SCORE": "score",
	}

	for in, want := range tests {
		if got := normalizeComputeTallyRule(in); got != want {
			t.Fatalf("normalizeComputeTallyRule(%q)=%q want %q", in, got, want)
		}
	}
}

func TestNormalizeBallotFormatAcceptsScoringAlias(t *testing.T) {
	tests := map[string]string{
		"score":   "score",
		"scoring": "score",
		"SCORE":   "score",
	}

	for in, want := range tests {
		if got := normalizeBallotFormat(in); got != want {
			t.Fatalf("normalizeBallotFormat(%q)=%q want %q", in, got, want)
		}
	}
}

func TestGRPCBallotFormatNameMapsScoreToScoring(t *testing.T) {
	tests := map[string]string{
		"ranking": "ranking",
		"approval": "approval",
		"score":   "scoring",
		"scoring": "scoring",
	}

	for in, want := range tests {
		if got := grpcBallotFormatName(in); got != want {
			t.Fatalf("grpcBallotFormatName(%q)=%q want %q", in, got, want)
		}
	}
}