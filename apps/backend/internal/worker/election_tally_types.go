package worker

import "strings"

const (
	electionTallyTaskKind   = "election_tally"
	electionTallyResultKind = "election_tally_result"
)

type ElectionCandidate struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type ElectionTallyTask struct {
	Kind           string              `json:"kind"`
	JobID          string              `json:"job_id"`
	ElectionID     string              `json:"election_id"`
	TallyRule      string              `json:"tally_rule"`
	BallotFormat   string              `json:"ballot_format"`
	CommitteeSize  *int                `json:"committee_size,omitempty"`
	QuotaType      *string             `json:"quota_type,omitempty"`
	RankingTopK    *int                `json:"ranking_top_k,omitempty"`
	ShowAggregates bool                `json:"show_aggregates,omitempty"`
	Candidates     []ElectionCandidate `json:"candidates"`
}

type ElectionTallyResult struct {
	Kind       string         `json:"kind"`
	JobID      string         `json:"job_id"`
	ElectionID string         `json:"election_id"`
	Status     string         `json:"status"`
	ErrorText  string         `json:"error_text,omitempty"`
	Method     string         `json:"method,omitempty"`
	TallyRule  string         `json:"tally_rule,omitempty"`
	Params     map[string]any `json:"params,omitempty"`
	Winners    []string       `json:"winners,omitempty"`
	Metrics    map[string]any `json:"metrics,omitempty"`
	Protocol   map[string]any `json:"protocol,omitempty"`
	Timings    map[string]any `json:"timings,omitempty"`
	Artifacts  map[string]any `json:"artifacts,omitempty"`
}

func normalizeExternalRankingTallyRule(rule string) string {
	v := strings.ToLower(strings.TrimSpace(rule))
	v = strings.ReplaceAll(v, "_", "-")

	switch v {
	case "plurality",
		"borda",
		"black",
		"copeland-i",
		"copeland-ii",
		"copeland-iii",
		"simpson",
		"hare",
		"nanson",
		"coombs",
		"inverse-borda",
		"inverse-plurality":
		return v
	case "anti-plurality":
		return "inverse-plurality"
	case "minimax", "minmax":
		return "minmax"
	default:
		return ""
	}
}

func supportsExternalElectionTally(ballotFormat, tallyRule string) bool {
	if strings.TrimSpace(ballotFormat) != "ranking" {
		return false
	}
	return normalizeExternalRankingTallyRule(tallyRule) != ""
}
