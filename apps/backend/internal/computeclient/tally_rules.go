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
			f = strings.TrimSpace(f)
			if f != "" {
				formats = append(formats, f)
			}
		}

		out = append(out, TallyRuleInfo{
			ID:                         strings.TrimSpace(item.GetId()),
			Label:                      strings.TrimSpace(item.GetLabel()),
			BallotFormats:              formats,
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