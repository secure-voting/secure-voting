package computeclient

import (
	"context"
	"strings"

	"google.golang.org/protobuf/types/known/emptypb"
)

type TallyRuleInfo struct {
	ID                         string   `json:"id"`
	Label                      string   `json:"label"`
	BallotFormats              []string `json:"ballot_formats"`
	SupportsElectionTally      bool     `json:"supports_election_tally"`
	SupportsExperimentRuns     bool     `json:"supports_experiment_runs"`
	RequiresCommitteeSize      bool     `json:"requires_committee_size"`
	SupportsQuotaType          bool     `json:"supports_quota_type"`
	RequiresApprovalMaxChoices bool     `json:"requires_approval_max_choices"`
	SupportsRankingTopK        bool     `json:"supports_ranking_top_k"`
	RequiresScoreRange         bool     `json:"requires_score_range"`
}

func normalizeBallotFormat(value string) string {
	v := strings.ToLower(strings.TrimSpace(value))
	switch v {
	case "scoring":
		return "score"
	default:
		return v
	}
}

func uniqueNonEmpty(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))

	for _, value := range values {
		v := strings.TrimSpace(value)
		if v == "" {
			continue
		}

		if _, ok := seen[v]; ok {
			continue
		}

		seen[v] = struct{}{}
		out = append(out, v)
	}

	return out
}

func (c *Client) ListTallyRules(ctx context.Context) ([]TallyRuleInfo, error) {
	resp, err := c.client.ListTallyRules(ctx, &emptypb.Empty{})
	if err != nil {
		return nil, err
	}

	out := make([]TallyRuleInfo, 0, len(resp.GetRules()))
	for _, item := range resp.GetRules() {
		if item == nil {
			continue
		}

		formats := make([]string, 0, len(item.GetBallotFormats()))
		for _, f := range item.GetBallotFormats() {
			format := normalizeBallotFormat(f)
			if format != "" {
				formats = append(formats, format)
			}
		}

		out = append(out, TallyRuleInfo{
			ID:                         strings.TrimSpace(item.GetId()),
			Label:                      strings.TrimSpace(item.GetLabel()),
			BallotFormats:              uniqueNonEmpty(formats),
			SupportsElectionTally:      item.GetSupportsElectionTally(),
			SupportsExperimentRuns:     item.GetSupportsExperimentRuns(),
			RequiresCommitteeSize:      item.GetRequiresCommitteeSize(),
			SupportsQuotaType:          item.GetSupportsQuotaType(),
			RequiresApprovalMaxChoices: item.GetRequiresApprovalMaxChoices(),
			SupportsRankingTopK:        item.GetSupportsRankingTopK(),
			RequiresScoreRange:         item.GetRequiresScoreRange(),
		})
	}

	return out, nil
}
