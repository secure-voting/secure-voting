package elections

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func (s *Service) GetBallotMeta(ctx context.Context, electionID, userID, email, role string) (BallotMeta, string, error) {
	if _, err := uuid.Parse(electionID); err != nil {
		return BallotMeta{}, "invalid_id", nil
	}

	allowed, err := s.isAccessible(ctx, electionID, userID, email, role)
	if err != nil {
		return BallotMeta{}, "", err
	}
	if !allowed {
		return BallotMeta{}, "not_found", nil
	}

	var meta BallotMeta
	meta.ElectionID = electionID

	err = s.db.QueryRow(ctx, `
		SELECT tally_rule, ballot_format,
		       approval_max_choices, ranking_top_k,
		       score_min, score_max, score_step, score_allow_skip
		FROM elections
		WHERE id=$1::uuid
	`, electionID).Scan(
		&meta.TallyRule, &meta.BallotFormat,
		&meta.ApprovalMaxChoices, &meta.RankingTopK,
		&meta.ScoreMin, &meta.ScoreMax, &meta.ScoreStep, &meta.ScoreAllowSkip,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return BallotMeta{}, "not_found", nil
		}
		return BallotMeta{}, "", err
	}

	rows, err := s.db.Query(ctx, `SELECT id::text, name, meta FROM candidates WHERE election_id=$1::uuid ORDER BY name`, electionID)
	if err != nil {
		return BallotMeta{}, "", err
	}
	defer rows.Close()

	for rows.Next() {
		var c Candidate
		var metaJSON []byte
		if err := rows.Scan(&c.ID, &c.Name, &metaJSON); err != nil {
			return BallotMeta{}, "", err
		}
		if len(metaJSON) > 0 && string(metaJSON) != "null" {
			_ = json.Unmarshal(metaJSON, &c.Meta)
		}
		meta.Candidates = append(meta.Candidates, c)
	}

	return meta, "", nil
}
