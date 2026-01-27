package ballots

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
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
	ApprovalSet []string       `json:"approval_set,omitempty"`
	Ranking     []string       `json:"ranking,omitempty"`
	Scores      map[string]int `json:"scores,omitempty"`
}

type SubmitResp struct {
	Ok       bool   `json:"ok"`
	BallotID string `json:"ballot_id"`
	Status   string `json:"status"`
}

type MyBallotResp struct {
	Status      string  `json:"status"`
	SubmittedAt *string `json:"submitted_at,omitempty"`
	UpdatedAt   *string `json:"updated_at,omitempty"`
}

type electionVoteCfg struct {
	BallotFormat    string
	Status          string
	AccessMode      string
	Allowed         bool
	ApprovalMax     *int
	RankingTopK     *int
	ScoreMin        *int
	ScoreMax        *int
	ScoreStep       *int
	ScoreAllowSkip  bool
}

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
		             AND i.status IN ('created','sent','accepted')
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

func (s *Service) tryGetCached(ctx context.Context, rkey string) (SubmitResp, bool) {
	if s.rdb == nil {
		return SubmitResp{}, false
	}
	val, err := s.rdb.Get(ctx, rkey).Result()
	if err != nil {
		return SubmitResp{}, false
	}
	if strings.TrimSpace(val) == "" {
		return SubmitResp{}, false
	}
	var cached SubmitResp
	if json.Unmarshal([]byte(val), &cached) != nil {
		return SubmitResp{}, false
	}
	if cached.BallotID == "" || cached.Status == "" {
		return SubmitResp{}, false
	}
	return cached, true
}

func (s *Service) cacheResp(ctx context.Context, rkey string, resp SubmitResp) {
	if s.rdb == nil {
		return
	}
	b, err := json.Marshal(resp)
	if err != nil {
		return
	}
	_ = s.rdb.Set(ctx, rkey, string(b), s.idemTTL).Err()
}

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

func insertAuditTx(ctx context.Context, tx pgx.Tx, actorUserID, eventType string, details map[string]any) error {
	if details == nil {
		details = map[string]any{}
	}
	b, err := json.Marshal(details)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx,
		`INSERT INTO audit_log (actor_user_id, event_type, details)
		 VALUES ($1::uuid, $2, $3::jsonb)`,
		actorUserID, eventType, string(b),
	)
	return err
}
