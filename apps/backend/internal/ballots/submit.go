package ballots

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func (s *Service) Submit(ctx context.Context, electionID, userID, email, idemKey string, req SubmitReq) (SubmitResp, string, error) {
	if _, err := uuid.Parse(electionID); err != nil {
		return SubmitResp{}, "invalid_id", nil
	}
	idemKey = strings.TrimSpace(idemKey)
	if idemKey == "" {
		return SubmitResp{}, "missing_idempotency_key", nil
	}
	if !validateIdempotencyKey(idemKey) {
		return SubmitResp{}, "invalid_idempotency_key", nil
	}

	cfg, code, err := s.loadElectionVoteCfg(ctx, electionID, email)
	if err != nil {
		return SubmitResp{}, "", err
	}
	if code != "" {
		return SubmitResp{}, code, nil
	}
	if cfg.Status != "active" {
		return SubmitResp{}, "election_not_active", nil
	}

	voterHash := computeVoterHash(electionID, userID)
	rkey := fmt.Sprintf("idem:submit:%s:%s:%s", electionID, voterHash, idemKey)

	if cached, ok := s.tryGetCached(ctx, rkey); ok {
		return cached, "", nil
	}

	var unlock func()
	if s.rdb != nil {
		lockKey := rkey + ":lock"
		ok, lockErr := s.rdb.SetNX(ctx, lockKey, "1", 15*time.Second).Result()
		if lockErr == nil && ok {
			unlock = func() { _ = s.rdb.Del(ctx, lockKey).Err() }
		} else {
			deadline := time.Now().Add(2 * time.Second)
			for time.Now().Before(deadline) {
				if cached, ok := s.tryGetCached(ctx, rkey); ok {
					return cached, "", nil
				}
				select {
				case <-ctx.Done():
					return SubmitResp{}, "", ctx.Err()
				case <-time.After(50 * time.Millisecond):
				}
			}
			return SubmitResp{}, "idempotency_in_progress", nil
		}
	}
	if unlock != nil {
		defer unlock()
	}

	cRows, err := s.db.Query(ctx, `SELECT id::text FROM candidates WHERE election_id=$1::uuid`, electionID)
	if err != nil {
		return SubmitResp{}, "", err
	}
	defer cRows.Close()

	cset := map[string]struct{}{}
	var candidates []string
	for cRows.Next() {
		var cid string
		if err := cRows.Scan(&cid); err != nil {
			return SubmitResp{}, "", err
		}
		cset[cid] = struct{}{}
		candidates = append(candidates, cid)
	}
	if len(candidates) == 0 {
		return SubmitResp{}, "no_candidates", nil
	}

	var approvalJSON, rankingJSON, scoresJSON []byte

	switch cfg.BallotFormat {
	case "approval":
		if vcode := validateApprovalBallot(req.ApprovalSet, cset, cfg.ApprovalMax); vcode != "" {
			return SubmitResp{}, vcode, nil
		}
		approvalJSON, _ = json.Marshal(req.ApprovalSet)

	case "ranking":
		if vcode := validateRankingBallot(req.Ranking, cset, cfg.RankingTopK); vcode != "" {
			return SubmitResp{}, vcode, nil
		}
		rankingJSON, _ = json.Marshal(req.Ranking)

	case "score":
		if vcode := validateScoreBallot(req.Scores, candidates, cset, cfg.ScoreMin, cfg.ScoreMax, cfg.ScoreStep, cfg.ScoreAllowSkip); vcode != "" {
			return SubmitResp{}, vcode, nil
		}
		scoresJSON, _ = json.Marshal(req.Scores)

	default:
		return SubmitResp{}, "invalid_ballot_format", nil
	}

	tx, err := s.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return SubmitResp{}, "", err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var ballotID string
	err = tx.QueryRow(ctx, `
		INSERT INTO ballots (
			election_id, voter_hash, format,
			approval_set, ranking, scores,
			status, submitted_at, updated_at
		)
		VALUES (
			$1::uuid, $2, $3,
			$4::jsonb, $5::jsonb, $6::jsonb,
			'accepted', now(), now()
		)
		ON CONFLICT (election_id, voter_hash)
		DO UPDATE SET
			format = EXCLUDED.format,
			approval_set = EXCLUDED.approval_set,
			ranking = EXCLUDED.ranking,
			scores = EXCLUDED.scores,
			status = 'accepted',
			submitted_at = now(),
			updated_at = now()
		WHERE ballots.status = 'draft'
		RETURNING id::text
	`, electionID, voterHash, cfg.BallotFormat,
		toJSONBOrNull(approvalJSON),
		toJSONBOrNull(rankingJSON),
		toJSONBOrNull(scoresJSON),
	).Scan(&ballotID)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return SubmitResp{}, "already_submitted", nil
		}
		return SubmitResp{}, "", err
	}

	_ = insertAuditTx(ctx, tx, userID, "ballot_submitted", map[string]any{
		"target_type": "ballot",
		"target_id":   ballotID,
		"election_id": electionID,
		"status":      "accepted",
	})

	if err := tx.Commit(ctx); err != nil {
		return SubmitResp{}, "", err
	}

	resp := SubmitResp{Ok: true, BallotID: ballotID, Status: "accepted"}
	s.cacheResp(ctx, rkey, resp)

	return resp, "", nil
}
