package ballots

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func (s *Service) MyBallot(ctx context.Context, electionID, userID, email string) (MyBallotResp, string, error) {
	if _, err := uuid.Parse(electionID); err != nil {
		return MyBallotResp{}, "invalid_id", nil
	}

	_, code, err := s.loadElectionVoteCfg(ctx, electionID, email)
	if err != nil {
		return MyBallotResp{}, "", err
	}
	if code != "" {
		return MyBallotResp{}, code, nil
	}

	voterHash := computeVoterHash(electionID, userID)

	var st string
	var sub *time.Time
	var upd *time.Time

	err = s.db.QueryRow(ctx, `
		SELECT status, submitted_at, updated_at
		FROM ballots
		WHERE election_id=$1::uuid AND voter_hash=$2
		LIMIT 1
	`, electionID, voterHash).Scan(&st, &sub, &upd)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return MyBallotResp{Status: "none"}, "", nil
		}
		return MyBallotResp{}, "", err
	}

	var subS *string
	if sub != nil {
		v := sub.UTC().Format(time.RFC3339)
		subS = &v
	}

	var updS *string
	if upd != nil {
		v := upd.UTC().Format(time.RFC3339)
		updS = &v
	}

	return MyBallotResp{
		Status:      st,
		SubmittedAt: subS,
		UpdatedAt:   updS,
	}, "", nil
}
