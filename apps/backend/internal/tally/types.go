package tally

type Output struct {
	Method   string         `json:"method"`
	Params   map[string]any `json:"params,omitempty"`
	Winners  []string       `json:"winners"`
	Metrics  map[string]any `json:"metrics,omitempty"`
	Protocol map[string]any `json:"protocol,omitempty"`
}

const (
	CodeInvalidID                = "invalid_id"
	CodeNotFound                 = "not_found"
	CodeNoCandidates             = "no_candidates"
	CodeNoBallots                = "no_ballots"
	CodeBadBallotData            = "bad_ballot_data"
	CodeUnsupportedRuleForFormat = "unsupported_rule_for_format"
	CodeUnsupportedTallyRule     = "unsupported_tally_rule"
)
