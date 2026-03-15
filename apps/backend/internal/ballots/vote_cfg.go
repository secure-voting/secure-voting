package ballots

import (
	"context"
	"errors"
	"strings"

	"github.com/jackc/pgx/v5"
)

func (s *Service) loadElectionVoteCfg(ctx context.Context, electionID, email string) (electionVoteCfg, string, error) {
	email = strings.TrimSpace(strings.ToLower(email))

	var cfg electionVoteCfg
	err := s.db.QueryRow(ctx, `
		SELECT e.ballot_format, e.status, e.access_mode,
		       e.approval_max_choices, e.ranking_top_k,
		       e.score_min, e.score_max, e.score_step, e.score_allow_skip,
		       CASE
		         WHEN e.access_mode = 'open' THEN true
		         WHEN EXISTS (
		           SELECT 1 FROM election_invites i
		           WHERE i.election_id = e.id
		             AND lower(i.email) = lower($2)
		             AND i.status = 'accepted'
		         ) THEN true
		         ELSE false
		       END AS allowed
		FROM elections e
		WHERE e.id = $1::uuid
	`, electionID, email).Scan(
		&cfg.BallotFormat, &cfg.Status, &cfg.AccessMode,
		&cfg.ApprovalMax, &cfg.RankingTopK,
		&cfg.ScoreMin, &cfg.ScoreMax, &cfg.ScoreStep, &cfg.ScoreAllowSkip,
		&cfg.Allowed,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return electionVoteCfg{}, "not_found", nil
		}
		return electionVoteCfg{}, "", err
	}
	if !cfg.Allowed {
		return electionVoteCfg{}, "not_found", nil
	}
	return cfg, "", nil
}
