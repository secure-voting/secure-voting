package experimentruns

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

type fakeRow struct {
	scanFn func(dest ...any) error
}

func (r fakeRow) Scan(dest ...any) error {
	return r.scanFn(dest...)
}

type fakeRows struct {
	rows [][]any
	idx  int
	err  error
}

func (r *fakeRows) Next() bool {
	return r.idx < len(r.rows)
}

func (r *fakeRows) Scan(dest ...any) error {
	if r.idx >= len(r.rows) {
		return errors.New("scan past end")
	}
	row := r.rows[r.idx]
	r.idx++

	if len(dest) != len(row) {
		return fmt.Errorf("dest len=%d row len=%d", len(dest), len(row))
	}

	for i := range dest {
		if err := assignScan(dest[i], row[i]); err != nil {
			return fmt.Errorf("assign col %d: %w", i, err)
		}
	}
	return nil
}

func (r *fakeRows) Close() {}

func (r *fakeRows) Err() error { return r.err }

type fakeSingleResult struct {
	decodeFn func(v any) error
}

func (r fakeSingleResult) Decode(v any) error {
	return r.decodeFn(v)
}

type fakeBatchTx struct {
	commitErr   error
	rollbackErr error
	committed   bool
	rolledBack  bool
}

func (tx *fakeBatchTx) Commit(ctx context.Context) error {
	tx.committed = true
	return tx.commitErr
}

func (tx *fakeBatchTx) Rollback(ctx context.Context) error {
	tx.rolledBack = true
	return tx.rollbackErr
}

func assignScan(dest any, v any) error {
	switch d := dest.(type) {
	case *string:
		if v == nil {
			*d = ""
			return nil
		}
		s, ok := v.(string)
		if !ok {
			return fmt.Errorf("want string got %T", v)
		}
		*d = s
		return nil

	case **string:
		if v == nil {
			*d = nil
			return nil
		}
		s, ok := v.(string)
		if !ok {
			return fmt.Errorf("want string got %T", v)
		}
		x := s
		*d = &x
		return nil

	case **time.Time:
		if v == nil {
			*d = nil
			return nil
		}
		tv, ok := v.(time.Time)
		if !ok {
			return fmt.Errorf("want time.Time got %T", v)
		}
		x := tv
		*d = &x
		return nil

	default:
		return fmt.Errorf("unsupported dest %T", dest)
	}
}

func restoreExperimentRunHooks() func() {
	oldCheckOwner := batchCheckExperimentOwnerFn
	oldValidateDatasets := batchValidateDatasetsExistFn
	oldBeginTx := batchBeginTxFn
	oldInsertRun := batchInsertRunFn
	oldInsertJob := batchInsertJobFn
	oldAudit := batchAuditInsertFn
	oldGetRow := getRunQueryRowFn
	oldListRows := listRunsQueryFn
	oldGetAccess := getRunAccessFn
	oldFindResult := findExperimentResultFn
	oldCountDatasets := countDatasetsFn
	oldDownloadGet := downloadResultGetFn

	return func() {
		batchCheckExperimentOwnerFn = oldCheckOwner
		batchValidateDatasetsExistFn = oldValidateDatasets
		batchBeginTxFn = oldBeginTx
		batchInsertRunFn = oldInsertRun
		batchInsertJobFn = oldInsertJob
		batchAuditInsertFn = oldAudit
		getRunQueryRowFn = oldGetRow
		listRunsQueryFn = oldListRows
		getRunAccessFn = oldGetAccess
		findExperimentResultFn = oldFindResult
		countDatasetsFn = oldCountDatasets
		downloadResultGetFn = oldDownloadGet
	}
}

func TestNewService(t *testing.T) {
	svc := NewService(nil, nil)
	if svc == nil {
		t.Fatal("expected service")
	}
}

func TestBatchCreate_InvalidExperimentID(t *testing.T) {
	svc := NewService(nil, nil)

	_, code, err := svc.BatchCreate(context.Background(), "user", "researcher", BatchReq{
		ExperimentID: "bad",
		DatasetIDs:   []string{"507f1f77bcf86cd799439011"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if code != "invalid_experiment_id" {
		t.Fatalf("expected invalid_experiment_id, got %q", code)
	}
}

func TestBatchCreate_DatasetIDsRequired(t *testing.T) {
	svc := NewService(nil, nil)

	_, code, err := svc.BatchCreate(context.Background(), "user", "researcher", BatchReq{
		ExperimentID: "11111111-1111-1111-1111-111111111111",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if code != "dataset_ids_required" {
		t.Fatalf("expected dataset_ids_required, got %q", code)
	}
}

func TestBatchCreate_NotFoundForOwnerACL(t *testing.T) {
	defer restoreExperimentRunHooks()()

	batchCheckExperimentOwnerFn = func(_ context.Context, _ *pgxpool.Pool, _, _ string) (bool, error) {
		return false, nil
	}

	svc := NewService(nil, nil)
	_, code, err := svc.BatchCreate(context.Background(), "11111111-1111-1111-1111-111111111111", "researcher", BatchReq{
		ExperimentID: "22222222-2222-2222-2222-222222222222",
		DatasetIDs:   []string{"507f1f77bcf86cd799439011"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if code != "not_found" {
		t.Fatalf("expected not_found, got %q", code)
	}
}

func TestBatchCreate_InvalidDatasetID(t *testing.T) {
	defer restoreExperimentRunHooks()()

	batchCheckExperimentOwnerFn = func(_ context.Context, _ *pgxpool.Pool, _, _ string) (bool, error) {
		return true, nil
	}

	svc := NewService(nil, nil)
	_, code, err := svc.BatchCreate(context.Background(), "11111111-1111-1111-1111-111111111111", "researcher", BatchReq{
		ExperimentID: "22222222-2222-2222-2222-222222222222",
		DatasetIDs:   []string{"bad"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if code != "invalid_dataset_id" {
		t.Fatalf("expected invalid_dataset_id, got %q", code)
	}
}

func TestBatchCreate_DatasetNotFound(t *testing.T) {
	defer restoreExperimentRunHooks()()

	batchCheckExperimentOwnerFn = func(_ context.Context, _ *pgxpool.Pool, _, _ string) (bool, error) {
		return true, nil
	}
	batchValidateDatasetsExistFn = func(_ *Service, _ context.Context, _ []primitive.ObjectID) (bool, error) {
		return false, nil
	}

	svc := NewService(nil, nil)
	_, code, err := svc.BatchCreate(context.Background(), "11111111-1111-1111-1111-111111111111", "researcher", BatchReq{
		ExperimentID: "22222222-2222-2222-2222-222222222222",
		DatasetIDs:   []string{"507f1f77bcf86cd799439011"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if code != "dataset_not_found" {
		t.Fatalf("expected dataset_not_found, got %q", code)
	}
}

func TestBatchCreate_SuccessDeduplicatesAndCommits(t *testing.T) {
	defer restoreExperimentRunHooks()()

	fakeTx := &fakeBatchTx{}
	runByDataset := map[string]string{
		"507f1f77bcf86cd799439011": "run-1",
		"507f1f77bcf86cd799439012": "run-2",
	}
	jobByRun := map[string]string{
		"run-1": "job-1",
		"run-2": "job-2",
	}

	batchCheckExperimentOwnerFn = func(_ context.Context, _ *pgxpool.Pool, _, _ string) (bool, error) {
		return true, nil
	}
	batchValidateDatasetsExistFn = func(_ *Service, _ context.Context, oids []primitive.ObjectID) (bool, error) {
		if len(oids) != 2 {
			t.Fatalf("expected 2 unique dataset ids, got %d", len(oids))
		}
		return true, nil
	}
	batchBeginTxFn = func(_ context.Context, _ *pgxpool.Pool) (batchTx, error) {
		return fakeTx, nil
	}
	batchInsertRunFn = func(_ context.Context, _ batchTx, _ string, dsid string) (string, error) {
		return runByDataset[dsid], nil
	}
	batchInsertJobFn = func(_ context.Context, _ batchTx, _, _, runID, _ string) (string, error) {
		return jobByRun[runID], nil
	}
	batchAuditInsertFn = func(_ context.Context, _ batchTx, createdBy, expID string, count int) error {
		if createdBy != "11111111-1111-1111-1111-111111111111" {
			t.Fatalf("unexpected createdBy: %q", createdBy)
		}
		if expID != "22222222-2222-2222-2222-222222222222" {
			t.Fatalf("unexpected expID: %q", expID)
		}
		if count != 2 {
			t.Fatalf("expected count=2, got %d", count)
		}
		return nil
	}

	svc := NewService(nil, nil)
	items, code, err := svc.BatchCreate(context.Background(), "11111111-1111-1111-1111-111111111111", "researcher", BatchReq{
		ExperimentID: "22222222-2222-2222-2222-222222222222",
		DatasetIDs: []string{
			"507f1f77bcf86cd799439011",
			"507f1f77bcf86cd799439011",
			"507f1f77bcf86cd799439012",
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if code != "" {
		t.Fatalf("unexpected code: %q", code)
	}
	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d %#v", len(items), items)
	}
	if !fakeTx.committed {
		t.Fatal("expected tx commit")
	}
}

func TestGet_InvalidID(t *testing.T) {
	svc := NewService(nil, nil)

	_, code, err := svc.Get(context.Background(), "researcher", "user", "bad")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if code != "invalid_id" {
		t.Fatalf("expected invalid_id, got %q", code)
	}
}

func TestGet_NotFound(t *testing.T) {
	defer restoreExperimentRunHooks()()

	getRunQueryRowFn = func(_ context.Context, _ *pgxpool.Pool, _ string, _ ...any) rowScanner {
		return fakeRow{scanFn: func(dest ...any) error { return pgx.ErrNoRows }}
	}

	svc := NewService(nil, nil)
	_, code, err := svc.Get(context.Background(), "researcher", "user", "11111111-1111-1111-1111-111111111111")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if code != "not_found" {
		t.Fatalf("expected not_found, got %q", code)
	}
}

func TestGet_ACLHidden(t *testing.T) {
	defer restoreExperimentRunHooks()()

	now := time.Date(2026, 3, 18, 10, 0, 0, 0, time.UTC)
	getRunQueryRowFn = func(_ context.Context, _ *pgxpool.Pool, _ string, _ ...any) rowScanner {
		return fakeRow{
			scanFn: func(dest ...any) error {
				row := []any{
					"run-1",
					"exp-1",
					"ds-1",
					"done",
					"kernel-1",
					now,
					now,
					"owner-2",
				}
				for i := range dest {
					if err := assignScan(dest[i], row[i]); err != nil {
						return err
					}
				}
				return nil
			},
		}
	}

	svc := NewService(nil, nil)
	_, code, err := svc.Get(context.Background(), "researcher", "owner-1", "11111111-1111-1111-1111-111111111111")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if code != "not_found" {
		t.Fatalf("expected not_found, got %q", code)
	}
}

func TestGet_Success(t *testing.T) {
	defer restoreExperimentRunHooks()()

	now := time.Date(2026, 3, 18, 10, 0, 0, 0, time.UTC)
	getRunQueryRowFn = func(_ context.Context, _ *pgxpool.Pool, _ string, _ ...any) rowScanner {
		return fakeRow{
			scanFn: func(dest ...any) error {
				row := []any{
					"run-1",
					"exp-1",
					"ds-1",
					"done",
					"kernel-1",
					now,
					now,
					"owner-1",
				}
				for i := range dest {
					if err := assignScan(dest[i], row[i]); err != nil {
						return err
					}
				}
				return nil
			},
		}
	}

	svc := NewService(nil, nil)
	r, code, err := svc.Get(context.Background(), "researcher", "owner-1", "11111111-1111-1111-1111-111111111111")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if code != "" {
		t.Fatalf("unexpected code: %q", code)
	}
	if r.ID != "run-1" || r.KernelTaskID == nil || *r.KernelTaskID != "kernel-1" {
		t.Fatalf("unexpected run: %#v", r)
	}
	if r.StartedAt == nil || r.FinishedAt == nil {
		t.Fatalf("expected started/finished timestamps: %#v", r)
	}
}

func TestList_InvalidExperimentID(t *testing.T) {
	svc := NewService(nil, nil)

	_, code, err := svc.List(context.Background(), "admin", "", "bad")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if code != "invalid_experiment_id" {
		t.Fatalf("expected invalid_experiment_id, got %q", code)
	}
}

func TestList_SuccessWithOwnerFilter(t *testing.T) {
	defer restoreExperimentRunHooks()()

	var capturedQuery string
	var capturedArgs []any
	now := time.Date(2026, 3, 18, 11, 0, 0, 0, time.UTC)

	listRunsQueryFn = func(_ context.Context, _ *pgxpool.Pool, q string, args ...any) (rowsScanner, error) {
		capturedQuery = q
		capturedArgs = args
		return &fakeRows{
			rows: [][]any{
				{"run-1", "exp-1", "ds-1", "done", "kernel-1", now, now, "owner-1"},
			},
		}, nil
	}

	svc := NewService(nil, nil)
	items, code, err := svc.List(context.Background(), "researcher", "owner-1", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if code != "" {
		t.Fatalf("unexpected code: %q", code)
	}
	if len(items) != 1 || items[0].ID != "run-1" {
		t.Fatalf("unexpected items: %#v", items)
	}
	if !strings.Contains(capturedQuery, "e.created_by = $1::uuid") {
		t.Fatalf("expected owner filter in query: %s", capturedQuery)
	}
	if len(capturedArgs) != 1 || capturedArgs[0] != "owner-1" {
		t.Fatalf("unexpected args: %#v", capturedArgs)
	}
}

func TestList_AdminWithoutOwnerFilter(t *testing.T) {
	defer restoreExperimentRunHooks()()

	var capturedQuery string
	listRunsQueryFn = func(_ context.Context, _ *pgxpool.Pool, q string, args ...any) (rowsScanner, error) {
		capturedQuery = q
		return &fakeRows{}, nil
	}

	svc := NewService(nil, nil)
	_, code, err := svc.List(context.Background(), "admin", "", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if code != "" {
		t.Fatalf("unexpected code: %q", code)
	}
	if strings.Contains(capturedQuery, "e.created_by =") {
		t.Fatalf("did not expect owner filter for admin: %s", capturedQuery)
	}
}

func TestList_QueryError(t *testing.T) {
	defer restoreExperimentRunHooks()()

	listRunsQueryFn = func(_ context.Context, _ *pgxpool.Pool, _ string, _ ...any) (rowsScanner, error) {
		return nil, errors.New("boom")
	}

	svc := NewService(nil, nil)
	_, _, err := svc.List(context.Background(), "admin", "", "")
	if err == nil || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("expected boom, got %v", err)
	}
}

func TestList_RowsErr(t *testing.T) {
	defer restoreExperimentRunHooks()()

	listRunsQueryFn = func(_ context.Context, _ *pgxpool.Pool, _ string, _ ...any) (rowsScanner, error) {
		return &fakeRows{err: errors.New("rows boom")}, nil
	}

	svc := NewService(nil, nil)
	_, _, err := svc.List(context.Background(), "admin", "", "")
	if err == nil || !strings.Contains(err.Error(), "rows boom") {
		t.Fatalf("expected rows boom, got %v", err)
	}
}

func TestGetResult_PropagatesAccessCode(t *testing.T) {
	defer restoreExperimentRunHooks()()

	getRunAccessFn = func(_ *Service, _ context.Context, _, _, _ string) (Run, string, error) {
		return Run{}, "not_found", nil
	}

	svc := NewService(nil, nil)
	_, code, err := svc.GetResult(context.Background(), "researcher", "user", "run-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if code != "not_found" {
		t.Fatalf("expected not_found, got %q", code)
	}
}

func TestGetResult_NotFoundInMongo(t *testing.T) {
	defer restoreExperimentRunHooks()()

	getRunAccessFn = func(_ *Service, _ context.Context, _, _, _ string) (Run, string, error) {
		return Run{ID: "run-1"}, "", nil
	}
	findExperimentResultFn = func(_ context.Context, _ *mongo.Database, _ string) singleResultDecoder {
		return fakeSingleResult{decodeFn: func(v any) error { return mongo.ErrNoDocuments }}
	}

	svc := NewService(nil, nil)
	_, code, err := svc.GetResult(context.Background(), "researcher", "user", "run-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if code != "not_found" {
		t.Fatalf("expected not_found, got %q", code)
	}
}

func TestGetResult_Success(t *testing.T) {
	defer restoreExperimentRunHooks()()

	getRunAccessFn = func(_ *Service, _ context.Context, _, _, _ string) (Run, string, error) {
		return Run{ID: "run-1"}, "", nil
	}
	findExperimentResultFn = func(_ context.Context, _ *mongo.Database, _ string) singleResultDecoder {
		return fakeSingleResult{
			decodeFn: func(v any) error {
				out := v.(*Result)
				out.RunID = "run-1"
				out.Winners = []any{"c1"}
				return nil
			},
		}
	}

	svc := NewService(nil, nil)
	res, code, err := svc.GetResult(context.Background(), "researcher", "user", "run-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if code != "" {
		t.Fatalf("unexpected code: %q", code)
	}
	if res.RunID != "run-1" || len(res.Winners) != 1 || res.Winners[0] != "c1" {
		t.Fatalf("unexpected result: %#v", res)
	}
}

func TestDownloadResult_Success(t *testing.T) {
	defer restoreExperimentRunHooks()()

	downloadResultGetFn = func(_ *Service, _ context.Context, _, _, runID string) (Result, string, error) {
		return Result{RunID: runID, Winners: []any{"c1"}}, "", nil
	}

	svc := NewService(nil, nil)
	data, filename, mime, code, err := svc.DownloadResult(context.Background(), "researcher", "user", "run-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if code != "" {
		t.Fatalf("unexpected code: %q", code)
	}
	if filename != "experiment_result_run-1.json" {
		t.Fatalf("unexpected filename: %q", filename)
	}
	if mime != "application/json" {
		t.Fatalf("unexpected mime: %q", mime)
	}
	if !strings.Contains(string(data), `"run_id":"run-1"`) {
		t.Fatalf("unexpected data: %s", string(data))
	}
}

func TestDownloadResult_Code(t *testing.T) {
	defer restoreExperimentRunHooks()()

	downloadResultGetFn = func(_ *Service, _ context.Context, _, _, _ string) (Result, string, error) {
		return Result{}, "not_found", nil
	}

	svc := NewService(nil, nil)
	_, _, _, code, err := svc.DownloadResult(context.Background(), "researcher", "user", "run-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if code != "not_found" {
		t.Fatalf("expected not_found, got %q", code)
	}
}

func TestValidateDatasetsExist(t *testing.T) {
	defer restoreExperimentRunHooks()()

	countDatasetsFn = func(_ context.Context, _ *mongo.Database, oids []primitive.ObjectID) (int64, error) {
		if len(oids) != 2 {
			t.Fatalf("unexpected oids len: %d", len(oids))
		}
		return 2, nil
	}

	svc := NewService(nil, nil)
	ok, err := svc.validateDatasetsExist(context.Background(), []primitive.ObjectID{
		primitive.NewObjectID(),
		primitive.NewObjectID(),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Fatal("expected ok=true")
	}
}

func TestValidateDatasetsExist_Empty(t *testing.T) {
	svc := NewService(nil, nil)
	ok, err := svc.validateDatasetsExist(context.Background(), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ok {
		t.Fatal("expected ok=false")
	}
}

func TestItoa(t *testing.T) {
	cases := map[int]string{
		0:   "0",
		7:   "7",
		42:  "42",
		-12: "-12",
	}
	for in, want := range cases {
		if got := itoa(in); got != want {
			t.Fatalf("itoa(%d)=%q want %q", in, got, want)
		}
	}
}
