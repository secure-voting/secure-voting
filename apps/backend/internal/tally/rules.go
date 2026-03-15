package tally

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

type ruleHandler struct {
	ballotFormat string
	loadBallots  func(ctx context.Context, db *pgxpool.Pool, electionID string) ([][]string, string, error)
	compute      func(committeeSize int, candidates []string, ballots [][]string) Output
}

var ruleHandlers = map[string]ruleHandler{
	"approval": {
		ballotFormat: "approval",
		loadBallots:  loadApprovalBallots,
		compute:      computeApproval,
	},
	"plurality": {
		ballotFormat: "ranking",
		loadBallots:  loadRankingBallots,
		compute:      computePlurality,
	},
	"borda": {
		ballotFormat: "ranking",
		loadBallots:  loadRankingBallots,
		compute:      computeBorda,
	},
}
