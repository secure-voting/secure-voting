package ballots

import (
	"time"

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
	BallotFormat   string
	Status         string
	AccessMode     string
	Allowed        bool
	ApprovalMax    *int
	RankingTopK    *int
	ScoreMin       *int
	ScoreMax       *int
	ScoreStep      *int
	ScoreAllowSkip bool
}
