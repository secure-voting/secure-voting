package ballots

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

type Service struct {
	db      *pgxpool.Pool
	rdb     *redis.Client
	idemTTL time.Duration
}

func NewService(db *pgxpool.Pool, rdb *redis.Client, idemTTL time.Duration) *Service {
	return &Service{db: db, rdb: rdb, idemTTL: idemTTL}
}

type SubmitReq struct {
	ApprovalSet []string         `json:"approval_set,omitempty"`
	Ranking     []string         `json:"ranking,omitempty"`
	Scores      map[string]int   `json:"scores,omitempty"`
}

type SubmitResp struct {
	Ok       bool   `json:"ok"`
	BallotID string `json:"ballot_id"`
	Status   string `json:"status"`
}

type MyBallotResp struct {
	Status     string  `json:"status"`
	SubmittedAt *string `json:"submitted_at,omitempty"`
	UpdatedAt   *string `json:"updated_at,omitempty"`
}

func (s *Service) Submit(ctx context.Context, electionID, userID, idemKey string, req SubmitReq) (SubmitResp, string, error) {
	if _, err := uuid.Parse(electionID); err != nil {
		return SubmitResp{}, "invalid_id", nil
	}
	if idemKey == "" {
		return SubmitResp{}, "missing_idempotency_key", nil
	}

	voterHash := computeVoterHash(electionID, userID)
	rkey := fmt.Sprintf("idem:submit:%s:%s:%s", electionID, voterHash, idemKey)

	// idempotency hit
	if s.rdb != nil {
		if val, err := s.rdb.Get(ctx, rkey).Result(); err == nil && val != "" {
			var cached SubmitResp
			if json.Unmarshal([]byte(val), &cached) == nil {
				return cached, "", nil
			}
		}
	}

	var ballotFormat, status string
	var approvalMaxChoices *int
	var rankingTopK *int
	var scoreMin, scoreMax, scoreStep *int
	var scoreAllowSkip bool

	err := s.db.QueryRow(ctx, `
		SELECT ballot_format, status,
		       approval_max_choices, ranking_top_k,
		       score_min, score_max, score_step, score_allow_skip
		FROM elections
		WHERE id=$1::uuid
	`, electionID).Scan(&ballotFormat, &status, &approvalMaxChoices, &rankingTopK, &scoreMin, &scoreMax, &scoreStep, &scoreAllowSkip)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return SubmitResp{}, "not_found", nil
		}
		return SubmitResp{}, "", err
	}

	if status != "active" {
		return SubmitResp{}, "election_not_active", nil
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

	switch ballotFormat {
	case "approval":
		if len(req.ApprovalSet) == 0 {
			return SubmitResp{}, "invalid_ballot", nil
		}
		if approvalMaxChoices != nil && len(req.ApprovalSet) > *approvalMaxChoices {
			return SubmitResp{}, "too_many_choices", nil
		}
		seen := map[string]struct{}{}
		for _, cid := range req.ApprovalSet {
			if _, err := uuid.Parse(cid); err != nil {
				return SubmitResp{}, "invalid_candidate_id", nil
			}
			if _, ok := cset[cid]; !ok {
				return SubmitResp{}, "invalid_candidate_id", nil
			}
			if _, ok := seen[cid]; ok {
				return SubmitResp{}, "duplicate_candidate", nil
			}
			seen[cid] = struct{}{}
		}
		approvalJSON, _ = json.Marshal(req.ApprovalSet)

	case "ranking":
		if len(req.Ranking) == 0 {
			return SubmitResp{}, "invalid_ballot", nil
		}
		if rankingTopK != nil && len(req.Ranking) > *rankingTopK {
			return SubmitResp{}, "too_many_choices", nil
		}
		seen := map[string]struct{}{}
		for _, cid := range req.Ranking {
			if _, err := uuid.Parse(cid); err != nil {
				return SubmitResp{}, "invalid_candidate_id", nil
			}
			if _, ok := cset[cid]; !ok {
				return SubmitResp{}, "invalid_candidate_id", nil
			}
			if _, ok := seen[cid]; ok {
				return SubmitResp{}, "duplicate_candidate", nil
			}
			seen[cid] = struct{}{}
		}
		rankingJSON, _ = json.Marshal(req.Ranking)

	case "score":
		if req.Scores == nil || len(req.Scores) == 0 {
			return SubmitResp{}, "invalid_ballot", nil
		}
		if scoreMin == nil || scoreMax == nil || scoreStep == nil || *scoreStep <= 0 {
			return SubmitResp{}, "score_rules_missing", nil
		}

		// no extra candidates
		for cid := range req.Scores {
			if _, err := uuid.Parse(cid); err != nil {
				return SubmitResp{}, "invalid_candidate_id", nil
			}
			if _, ok := cset[cid]; !ok {
				return SubmitResp{}, "invalid_candidate_id", nil
			}
			v := req.Scores[cid]
			if v < *scoreMin || v > *scoreMax {
				return SubmitResp{}, "score_out_of_range", nil
			}
			if (v-*scoreMin)%*scoreStep != 0 {
				return SubmitResp{}, "score_invalid_step", nil
			}
		}

		if !scoreAllowSkip {
			for _, cid := range candidates {
				if _, ok := req.Scores[cid]; !ok {
					return SubmitResp{}, "score_missing_candidate", nil
				}
			}
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

	// insert or update only if draft
	var ballotID string
	err = tx.QueryRow(ctx, `
		INSERT INTO ballots (election_id, voter_hash, format, approval_set, ranking, scores, status)
		VALUES ($1::uuid, $2, $3, $4::jsonb, $5::jsonb, $6::jsonb, 'accepted')
		ON CONFLICT (election_id, voter_hash)
		DO UPDATE SET
			format = EXCLUDED.format,
			approval_set = EXCLUDED.approval_set,
			ranking = EXCLUDED.ranking,
			scores = EXCLUDED.scores,
			status = 'accepted',
			updated_at = now()
		WHERE ballots.status = 'draft'
		RETURNING id::text
	`, electionID, voterHash, ballotFormat,
		toJSONBOrNull(approvalJSON),
		toJSONBOrNull(rankingJSON),
		toJSONBOrNull(scoresJSON),
	).Scan(&ballotID)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			// conflict but not updated => already accepted/rejected
			return SubmitResp{}, "already_submitted", nil
		}
		return SubmitResp{}, "", err
	}

	if err := tx.Commit(ctx); err != nil {
		return SubmitResp{}, "", err
	}

	resp := SubmitResp{Ok: true, BallotID: ballotID, Status: "accepted"}

	if s.rdb != nil {
		if b, err := json.Marshal(resp); err == nil {
			_ = s.rdb.Set(ctx, rkey, string(b), s.idemTTL).Err()
		}
	}

	return resp, "", nil
}

func (s *Service) MyBallot(ctx context.Context, electionID, userID string) (MyBallotResp, string, error) {
	if _, err := uuid.Parse(electionID); err != nil {
		return MyBallotResp{}, "invalid_id", nil
	}
	voterHash := computeVoterHash(electionID, userID)

	var st string
	var sub time.Time
	var upd *time.Time

	err := s.db.QueryRow(ctx, `
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

	subS := sub.UTC().Format(time.RFC3339)
	var updS *string
	if upd != nil {
		s := upd.UTC().Format(time.RFC3339)
		updS = &s
	}

	return MyBallotResp{
		Status:      st,
		SubmittedAt: &subS,
		UpdatedAt:   updS,
	}, "", nil
}

func computeVoterHash(electionID, userID string) string {
	h := sha256.Sum256([]byte("election:" + electionID + ":user:" + userID))
	return hex.EncodeToString(h[:])
}

func toJSONBOrNull(b []byte) any {
	if len(b) == 0 {
		return nil
	}
	return string(b)
}
