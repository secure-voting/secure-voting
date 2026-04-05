package ballots

import (
	"github.com/jackc/pgx/v5/pgxpool"
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/redis/go-redis/v9"
)

type fakeBallotRow struct {
	scanFn func(dest ...any) error
}

func (r fakeBallotRow) Scan(dest ...any) error {
	return r.scanFn(dest...)
}

type fakeCandidateRows struct {
	items   []string
	idx     int
	scanErr error
	rowsErr error
	closed  bool
}

func (r *fakeCandidateRows) Next() bool {
	return r.idx < len(r.items)
}

func (r *fakeCandidateRows) Scan(dest ...any) error {
	if r.scanErr != nil {
		return r.scanErr
	}
	if r.idx >= len(r.items) {
		return errors.New("out of range")
	}
	*(dest[0].(*string)) = r.items[r.idx]
	r.idx++
	return nil
}

func (r *fakeCandidateRows) Close() {
	r.closed = true
}

func (r *fakeCandidateRows) Err() error {
	return r.rowsErr
}

type fakeBallotTx struct {
	queryRowFn func(ctx context.Context, sql string, args ...any) rowScanner
	execFn     func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	commitFn   func(ctx context.Context) error
	rollbackFn func(ctx context.Context) error
}

func (tx *fakeBallotTx) QueryRow(ctx context.Context, sql string, args ...any) rowScanner {
	if tx.queryRowFn != nil {
		return tx.queryRowFn(ctx, sql, args...)
	}
	return fakeBallotRow{scanFn: func(dest ...any) error { return nil }}
}

func (tx *fakeBallotTx) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	if tx.execFn != nil {
		return tx.execFn(ctx, sql, args...)
	}
	return pgconn.NewCommandTag("INSERT 0 1"), nil
}

func (tx *fakeBallotTx) Commit(ctx context.Context) error {
	if tx.commitFn != nil {
		return tx.commitFn(ctx)
	}
	return nil
}

func (tx *fakeBallotTx) Rollback(ctx context.Context) error {
	if tx.rollbackFn != nil {
		return tx.rollbackFn(ctx)
	}
	return nil
}

func restoreBallotHooks() func() {
	oldQueryRow := ballotsQueryRowFn
	oldQuery := ballotsQueryFn
	oldBeginTx := ballotsBeginTxFn

	return func() {
		ballotsQueryRowFn = oldQueryRow
		ballotsQueryFn = oldQuery
		ballotsBeginTxFn = oldBeginTx
	}
}

func voteCfgRow(format, status, accessMode string, approvalMax, rankingTopK, scoreMin, scoreMax, scoreStep *int, scoreAllowSkip, allowed bool) rowScanner {
	return fakeBallotRow{
		scanFn: func(dest ...any) error {
			*(dest[0].(*string)) = format
			*(dest[1].(*string)) = status
			*(dest[2].(*string)) = accessMode
			*(dest[3].(**int)) = approvalMax
			*(dest[4].(**int)) = rankingTopK
			*(dest[5].(**int)) = scoreMin
			*(dest[6].(**int)) = scoreMax
			*(dest[7].(**int)) = scoreStep
			*(dest[8].(*bool)) = scoreAllowSkip
			*(dest[9].(*bool)) = allowed
			return nil
		},
	}
}

func ballotStateRow(status string, submittedAt, updatedAt *time.Time) rowScanner {
	return fakeBallotRow{
		scanFn: func(dest ...any) error {
			*(dest[0].(*string)) = status
			*(dest[1].(**time.Time)) = submittedAt
			*(dest[2].(**time.Time)) = updatedAt
			return nil
		},
	}
}

func TestLoadElectionVoteCfg_NotFound(t *testing.T) {
	defer restoreBallotHooks()()

	ballotsQueryRowFn = func(_ context.Context, _ *pgxpool.Pool, _ string, _ ...any) rowScanner {
		return fakeBallotRow{scanFn: func(dest ...any) error { return pgx.ErrNoRows }}
	}

	svc := NewService(nil, nil, time.Minute)
	_, code, err := svc.loadElectionVoteCfg(context.Background(), "11111111-1111-1111-1111-111111111111", "u@example.com")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if code != "not_found" {
		t.Fatalf("unexpected code: %s", code)
	}
}

func TestLoadElectionVoteCfg_QueryError(t *testing.T) {
	defer restoreBallotHooks()()

	ballotsQueryRowFn = func(_ context.Context, _ *pgxpool.Pool, _ string, _ ...any) rowScanner {
		return fakeBallotRow{scanFn: func(dest ...any) error { return errors.New("boom") }}
	}

	svc := NewService(nil, nil, time.Minute)
	_, code, err := svc.loadElectionVoteCfg(context.Background(), "11111111-1111-1111-1111-111111111111", "u@example.com")
	if err == nil || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("expected boom, got code=%s err=%v", code, err)
	}
}

func TestLoadElectionVoteCfg_NotAllowed(t *testing.T) {
	defer restoreBallotHooks()()

	topK := 3
	ballotsQueryRowFn = func(_ context.Context, _ *pgxpool.Pool, _ string, _ ...any) rowScanner {
		return voteCfgRow("ranking", "active", "invite", nil, &topK, nil, nil, nil, false, false)
	}

	svc := NewService(nil, nil, time.Minute)
	_, code, err := svc.loadElectionVoteCfg(context.Background(), "11111111-1111-1111-1111-111111111111", "u@example.com")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if code != "not_found" {
		t.Fatalf("unexpected code: %s", code)
	}
}

func TestLoadElectionVoteCfg_Success(t *testing.T) {
	defer restoreBallotHooks()()

	topK := 2
	ballotsQueryRowFn = func(_ context.Context, _ *pgxpool.Pool, _ string, _ ...any) rowScanner {
		return voteCfgRow("ranking", "active", "open", nil, &topK, nil, nil, nil, false, true)
	}

	svc := NewService(nil, nil, time.Minute)
	cfg, code, err := svc.loadElectionVoteCfg(context.Background(), "11111111-1111-1111-1111-111111111111", "u@example.com")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if code != "" {
		t.Fatalf("unexpected code: %s", code)
	}
	if cfg.BallotFormat != "ranking" || cfg.Status != "active" || !cfg.Allowed {
		t.Fatalf("unexpected cfg: %+v", cfg)
	}
	if cfg.RankingTopK == nil || *cfg.RankingTopK != 2 {
		t.Fatalf("unexpected ranking_top_k: %+v", cfg.RankingTopK)
	}
}

func TestMyBallot_InvalidID(t *testing.T) {
	svc := NewService(nil, nil, time.Minute)

	_, code, err := svc.MyBallot(context.Background(), "bad-id", "user-1", "u@example.com")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if code != "invalid_id" {
		t.Fatalf("unexpected code: %s", code)
	}
}

func TestMyBallot_NoBallot(t *testing.T) {
	defer restoreBallotHooks()()

	topK := 2
	ballotsQueryRowFn = func(_ context.Context, _ *pgxpool.Pool, q string, _ ...any) rowScanner {
		if strings.Contains(q, "FROM elections e") {
			return voteCfgRow("ranking", "active", "open", nil, &topK, nil, nil, nil, false, true)
		}
		return fakeBallotRow{scanFn: func(dest ...any) error { return pgx.ErrNoRows }}
	}

	svc := NewService(nil, nil, time.Minute)
	got, code, err := svc.MyBallot(context.Background(), "11111111-1111-1111-1111-111111111111", "user-1", "u@example.com")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if code != "" {
		t.Fatalf("unexpected code: %s", code)
	}
	if got.Status != "none" {
		t.Fatalf("unexpected resp: %+v", got)
	}
}

func TestMyBallot_Success(t *testing.T) {
	defer restoreBallotHooks()()

	topK := 2
	sub := time.Date(2026, 3, 18, 12, 0, 0, 0, time.UTC)
	upd := sub.Add(5 * time.Minute)

	ballotsQueryRowFn = func(_ context.Context, _ *pgxpool.Pool, q string, _ ...any) rowScanner {
		if strings.Contains(q, "FROM elections e") {
			return voteCfgRow("ranking", "active", "open", nil, &topK, nil, nil, nil, false, true)
		}
		return ballotStateRow("accepted", &sub, &upd)
	}

	svc := NewService(nil, nil, time.Minute)
	got, code, err := svc.MyBallot(context.Background(), "11111111-1111-1111-1111-111111111111", "user-1", "u@example.com")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if code != "" {
		t.Fatalf("unexpected code: %s", code)
	}
	if got.Status != "accepted" {
		t.Fatalf("unexpected status: %+v", got)
	}
	if got.SubmittedAt == nil || *got.SubmittedAt != sub.Format(time.RFC3339) {
		t.Fatalf("unexpected submitted_at: %+v", got.SubmittedAt)
	}
	if got.UpdatedAt == nil || *got.UpdatedAt != upd.Format(time.RFC3339) {
		t.Fatalf("unexpected updated_at: %+v", got.UpdatedAt)
	}
}

func TestSubmit_EarlyCodes(t *testing.T) {
	svc := NewService(nil, nil, time.Minute)

	if _, code, _ := svc.Submit(context.Background(), "bad-id", "user-1", "u@example.com", "11111111-1111-1111-1111-111111111111", SubmitReq{}); code != "invalid_id" {
		t.Fatalf("unexpected code: %s", code)
	}

	if _, code, _ := svc.Submit(context.Background(), "11111111-1111-1111-1111-111111111111", "user-1", "u@example.com", "", SubmitReq{}); code != "missing_idempotency_key" {
		t.Fatalf("unexpected code: %s", code)
	}
}

func TestSubmit_ElectionNotActive(t *testing.T) {
	defer restoreBallotHooks()()

	topK := 2
	ballotsQueryRowFn = func(_ context.Context, _ *pgxpool.Pool, _ string, _ ...any) rowScanner {
		return voteCfgRow("ranking", "scheduled", "open", nil, &topK, nil, nil, nil, false, true)
	}

	svc := NewService(nil, nil, time.Minute)
	_, code, err := svc.Submit(
		context.Background(),
		"11111111-1111-1111-1111-111111111111",
		"user-1",
		"u@example.com",
		"11111111-1111-1111-1111-111111111111",
		SubmitReq{Ranking: []string{"c1", "c2"}},
	)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if code != "election_not_active" {
		t.Fatalf("unexpected code: %s", code)
	}
}

func TestSubmit_CachedResponse(t *testing.T) {
	defer restoreBallotHooks()()

	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis: %v", err)
	}
	defer mr.Close()

	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer func() { _ = rdb.Close() }()

	topK := 2
	ballotsQueryRowFn = func(_ context.Context, _ *pgxpool.Pool, _ string, _ ...any) rowScanner {
		return voteCfgRow("ranking", "active", "open", nil, &topK, nil, nil, nil, false, true)
	}

	svc := NewService(nil, rdb, time.Minute)

	electionID := "11111111-1111-1111-1111-111111111111"
	userID := "user-1"
	idemKey := "11111111-1111-1111-1111-111111111111"
	voterHash := computeVoterHash(electionID, userID)
	rkey := "idem:submit:" + electionID + ":" + voterHash + ":" + idemKey

	want := SubmitResp{Ok: true, BallotID: "ballot-cached", Status: "accepted"}
	svc.cacheResp(context.Background(), rkey, want)

	got, code, err := svc.Submit(
		context.Background(),
		electionID,
		userID,
		"u@example.com",
		idemKey,
		SubmitReq{Ranking: []string{"c1", "c2"}},
	)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if code != "" {
		t.Fatalf("unexpected code: %s", code)
	}
	if got != want {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}

func TestSubmit_NoCandidates(t *testing.T) {
	defer restoreBallotHooks()()

	topK := 2
	ballotsQueryRowFn = func(_ context.Context, _ *pgxpool.Pool, _ string, _ ...any) rowScanner {
		return voteCfgRow("ranking", "active", "open", nil, &topK, nil, nil, nil, false, true)
	}
	ballotsQueryFn = func(_ context.Context, _ *pgxpool.Pool, _ string, _ ...any) (rowsScanner, error) {
		return &fakeCandidateRows{}, nil
	}

	svc := NewService(nil, nil, time.Minute)
	_, code, err := svc.Submit(
		context.Background(),
		"11111111-1111-1111-1111-111111111111",
		"user-1",
		"u@example.com",
		"11111111-1111-1111-1111-111111111111",
		SubmitReq{Ranking: []string{"c1", "c2"}},
	)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if code != "no_candidates" {
		t.Fatalf("unexpected code: %s", code)
	}
}

func TestSubmit_AlreadySubmitted(t *testing.T) {
	defer restoreBallotHooks()()

	topK := 2
	ballotsQueryRowFn = func(_ context.Context, _ *pgxpool.Pool, _ string, _ ...any) rowScanner {
		return voteCfgRow("ranking", "active", "open", nil, &topK, nil, nil, nil, false, true)
	}
	ballotsQueryFn = func(_ context.Context, _ *pgxpool.Pool, _ string, _ ...any) (rowsScanner, error) {
		return &fakeCandidateRows{items: []string{"11111111-1111-1111-1111-111111111111", "22222222-2222-2222-2222-222222222222", "33333333-3333-3333-3333-333333333333"}}, nil
	}
	ballotsBeginTxFn = func(_ context.Context, _ *pgxpool.Pool) (txLike, error) {
		return &fakeBallotTx{
			queryRowFn: func(_ context.Context, _ string, _ ...any) rowScanner {
				return fakeBallotRow{scanFn: func(dest ...any) error { return pgx.ErrNoRows }}
			},
		}, nil
	}

	svc := NewService(nil, nil, time.Minute)
	_, code, err := svc.Submit(
		context.Background(),
		"11111111-1111-1111-1111-111111111111",
		"user-1",
		"u@example.com",
		"11111111-1111-1111-1111-111111111111",
		SubmitReq{Ranking: []string{"11111111-1111-1111-1111-111111111111", "22222222-2222-2222-2222-222222222222"}},
	)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if code != "already_submitted" {
		t.Fatalf("unexpected code: %s", code)
	}
}

func TestSubmit_SuccessAndCache(t *testing.T) {
	defer restoreBallotHooks()()

	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis: %v", err)
	}
	defer mr.Close()

	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer func() { _ = rdb.Close() }()

	topK := 2
	ballotsQueryRowFn = func(_ context.Context, _ *pgxpool.Pool, _ string, _ ...any) rowScanner {
		return voteCfgRow("ranking", "active", "open", nil, &topK, nil, nil, nil, false, true)
	}
	ballotsQueryFn = func(_ context.Context, _ *pgxpool.Pool, _ string, _ ...any) (rowsScanner, error) {
		return &fakeCandidateRows{items: []string{"11111111-1111-1111-1111-111111111111", "22222222-2222-2222-2222-222222222222", "33333333-3333-3333-3333-333333333333"}}, nil
	}
	ballotsBeginTxFn = func(_ context.Context, _ *pgxpool.Pool) (txLike, error) {
		return &fakeBallotTx{
			queryRowFn: func(_ context.Context, _ string, _ ...any) rowScanner {
				return fakeBallotRow{
					scanFn: func(dest ...any) error {
						*(dest[0].(*string)) = "ballot-1"
						return nil
					},
				}
			},
			execFn: func(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
				return pgconn.NewCommandTag("INSERT 0 1"), nil
			},
			commitFn: func(_ context.Context) error { return nil },
		}, nil
	}

	svc := NewService(nil, rdb, time.Minute)

	electionID := "11111111-1111-1111-1111-111111111111"
	userID := "user-1"
	idemKey := "11111111-1111-1111-1111-111111111111"

	got, code, err := svc.Submit(
		context.Background(),
		electionID,
		userID,
		"u@example.com",
		idemKey,
		SubmitReq{Ranking: []string{"11111111-1111-1111-1111-111111111111", "22222222-2222-2222-2222-222222222222"}},
	)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if code != "" {
		t.Fatalf("unexpected code: %s", code)
	}
	if !got.Ok || got.BallotID != "ballot-1" || got.Status != "accepted" {
		t.Fatalf("unexpected resp: %+v", got)
	}

	rkey := "idem:submit:" + electionID + ":" + computeVoterHash(electionID, userID) + ":" + idemKey
	cached, ok := svc.tryGetCached(context.Background(), rkey)
	if !ok {
		t.Fatal("expected cached response")
	}
	if cached != got {
		t.Fatalf("cached %+v, got %+v", cached, got)
	}
}

func TestInsertAuditTx(t *testing.T) {
	var gotSQL string
	var gotArgs []any

	tx := &fakeBallotTx{
		execFn: func(_ context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
			gotSQL = sql
			gotArgs = args
			return pgconn.NewCommandTag("INSERT 0 1"), nil
		},
	}

	err := insertAuditTx(context.Background(), tx, "11111111-1111-1111-1111-111111111111", "ballot_submitted", nil)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if !strings.Contains(gotSQL, "INSERT INTO audit_log") {
		t.Fatalf("unexpected sql: %q", gotSQL)
	}
	if len(gotArgs) != 3 {
		t.Fatalf("unexpected args: %#v", gotArgs)
	}
}
