package jobs

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
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

	case *int:
		n, ok := v.(int)
		if !ok {
			return fmt.Errorf("want int got %T", v)
		}
		*d = n
		return nil

	case *[]byte:
		b, ok := v.([]byte)
		if !ok {
			return fmt.Errorf("want []byte got %T", v)
		}
		*d = append([]byte(nil), b...)
		return nil

	case *time.Time:
		tv, ok := v.(time.Time)
		if !ok {
			return fmt.Errorf("want time.Time got %T", v)
		}
		*d = tv
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

func restoreJobHooks() func() {
	oldGet := getJobQueryRowFn
	oldList := listJobsQueryFn
	oldClaim := claimNextJobQueryRowFn
	oldExec := runnerExecFn

	return func() {
		getJobQueryRowFn = oldGet
		listJobsQueryFn = oldList
		claimNextJobQueryRowFn = oldClaim
		runnerExecFn = oldExec
	}
}

func TestNewService(t *testing.T) {
	svc := NewService(nil)
	if svc == nil {
		t.Fatal("expected service")
	}
}

func TestNewRunner(t *testing.T) {
	r := NewRunner(nil)
	if r == nil {
		t.Fatal("expected runner")
	}
}

func TestNormalizeKinds(t *testing.T) {
	got := normalizeKinds([]string{" tally ", "", "tally", "experiment_run"})
	if len(got) != 2 || got[0] != "tally" || got[1] != "experiment_run" {
		t.Fatalf("unexpected kinds: %#v", got)
	}
}

func TestGet_InvalidID(t *testing.T) {
	svc := NewService(nil)

	_, code, err := svc.Get(context.Background(), "admin", "u1", "bad")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if code != "invalid_id" {
		t.Fatalf("expected invalid_id, got %q", code)
	}
}

func TestGet_NotFound(t *testing.T) {
	defer restoreJobHooks()()

	getJobQueryRowFn = func(_ context.Context, _ *pgxpool.Pool, _ string, _ ...any) rowScanner {
		return fakeRow{scanFn: func(dest ...any) error { return pgx.ErrNoRows }}
	}

	svc := NewService(nil)
	_, code, err := svc.Get(context.Background(), "admin", "u1", "11111111-1111-1111-1111-111111111111")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if code != "not_found" {
		t.Fatalf("expected not_found, got %q", code)
	}
}

func TestGet_ACLHidden(t *testing.T) {
	defer restoreJobHooks()()

	now := time.Date(2026, 3, 18, 12, 0, 0, 0, time.UTC)
	getJobQueryRowFn = func(_ context.Context, _ *pgxpool.Pool, _ string, _ ...any) rowScanner {
		return fakeRow{
			scanFn: func(dest ...any) error {
				row := []any{
					"job-1", "experiment_run", "done", 100, "owner-2",
					nil, nil, nil,
					nil, now, nil, nil,
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

	svc := NewService(nil)
	_, code, err := svc.Get(context.Background(), "researcher", "owner-1", "11111111-1111-1111-1111-111111111111")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if code != "not_found" {
		t.Fatalf("expected not_found, got %q", code)
	}
}

func TestGet_Success(t *testing.T) {
	defer restoreJobHooks()()

	now := time.Date(2026, 3, 18, 12, 1, 0, 0, time.UTC)
	getJobQueryRowFn = func(_ context.Context, _ *pgxpool.Pool, _ string, _ ...any) rowScanner {
		return fakeRow{
			scanFn: func(dest ...any) error {
				row := []any{
					"job-1", "experiment_run", "done", 100, "owner-1",
					"el-1", "exp-1", "run-1",
					"boom", now, now, now,
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

	svc := NewService(nil)
	j, code, err := svc.Get(context.Background(), "researcher", "owner-1", "11111111-1111-1111-1111-111111111111")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if code != "" {
		t.Fatalf("unexpected code: %q", code)
	}
	if j.ID != "job-1" || j.Kind != "experiment_run" || j.CreatedBy != "owner-1" {
		t.Fatalf("unexpected job: %#v", j)
	}
	if j.StartedAt == nil || j.FinishedAt == nil || j.ErrorText == nil {
		t.Fatalf("expected optional fields populated: %#v", j)
	}
}

func TestList_DefaultsAndFilters(t *testing.T) {
	defer restoreJobHooks()()

	var capturedQuery string
	var capturedArgs []any
	now := time.Date(2026, 3, 18, 12, 2, 0, 0, time.UTC)
	status := "done"
	kind := "experiment_run"

	listJobsQueryFn = func(_ context.Context, _ *pgxpool.Pool, q string, args ...any) (rowsScanner, error) {
		capturedQuery = q
		capturedArgs = args
		return &fakeRows{
			rows: [][]any{
				{"job-1", "experiment_run", "done", 100, "owner-1", nil, nil, nil, nil, now, nil, nil},
			},
		}, nil
	}

	svc := NewService(nil)
	items, err := svc.List(context.Background(), "researcher", "owner-1", ListFilter{
		Status: &status,
		Kind:   &kind,
		Limit:  0,
		Offset: -10,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 1 || items[0].ID != "job-1" {
		t.Fatalf("unexpected items: %#v", items)
	}
	if !strings.Contains(capturedQuery, "created_by = $1") {
		t.Fatalf("expected created_by filter: %s", capturedQuery)
	}
	if !strings.Contains(capturedQuery, "status = $2") {
		t.Fatalf("expected status filter: %s", capturedQuery)
	}
	if !strings.Contains(capturedQuery, "kind = $3") {
		t.Fatalf("expected kind filter: %s", capturedQuery)
	}
	if len(capturedArgs) != 5 {
		t.Fatalf("unexpected args: %#v", capturedArgs)
	}
	if capturedArgs[3] != 50 || capturedArgs[4] != 0 {
		t.Fatalf("unexpected limit/offset args: %#v", capturedArgs[3:])
	}
}

func TestList_AdminNoOwnerFilter(t *testing.T) {
	defer restoreJobHooks()()

	var capturedQuery string
	listJobsQueryFn = func(_ context.Context, _ *pgxpool.Pool, q string, args ...any) (rowsScanner, error) {
		capturedQuery = q
		return &fakeRows{}, nil
	}

	svc := NewService(nil)
	_, err := svc.List(context.Background(), "admin", "", ListFilter{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.Contains(capturedQuery, "created_by =") {
		t.Fatalf("did not expect owner filter: %s", capturedQuery)
	}
}

func TestList_QueryError(t *testing.T) {
	defer restoreJobHooks()()

	listJobsQueryFn = func(_ context.Context, _ *pgxpool.Pool, _ string, _ ...any) (rowsScanner, error) {
		return nil, errors.New("boom")
	}

	svc := NewService(nil)
	_, err := svc.List(context.Background(), "admin", "", ListFilter{})
	if err == nil || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("expected boom, got %v", err)
	}
}

func TestList_RowsErr(t *testing.T) {
	defer restoreJobHooks()()

	listJobsQueryFn = func(_ context.Context, _ *pgxpool.Pool, _ string, _ ...any) (rowsScanner, error) {
		return &fakeRows{err: errors.New("rows boom")}, nil
	}

	svc := NewService(nil)
	_, err := svc.List(context.Background(), "admin", "", ListFilter{})
	if err == nil || !strings.Contains(err.Error(), "rows boom") {
		t.Fatalf("expected rows boom, got %v", err)
	}
}

func TestClaimNext_KindsRequired(t *testing.T) {
	r := NewRunner(nil)

	_, ok, err := r.ClaimNext(context.Background(), nil)
	if err == nil || !strings.Contains(err.Error(), "kinds required") {
		t.Fatalf("expected kinds required error, got ok=%v err=%v", ok, err)
	}
}

func TestClaimNext_NoRows(t *testing.T) {
	defer restoreJobHooks()()

	claimNextJobQueryRowFn = func(_ context.Context, _ *pgxpool.Pool, _ string, _ ...any) rowScanner {
		return fakeRow{scanFn: func(dest ...any) error { return pgx.ErrNoRows }}
	}

	r := NewRunner(nil)
	_, ok, err := r.ClaimNext(context.Background(), []string{"experiment_run"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ok {
		t.Fatal("expected ok=false")
	}
}

func TestClaimNext_Success(t *testing.T) {
	defer restoreJobHooks()()

	now := time.Date(2026, 3, 18, 12, 3, 0, 0, time.UTC)
	claimNextJobQueryRowFn = func(_ context.Context, _ *pgxpool.Pool, q string, args ...any) rowScanner {
		if len(args) != 2 {
			t.Fatalf("expected 2 normalized kinds args, got %#v", args)
		}
		return fakeRow{
			scanFn: func(dest ...any) error {
				row := []any{
					"job-1", "experiment_run", "running", 0, "owner-1",
					nil, "exp-1", "run-1", []byte(`{"x":1}`), now,
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

	r := NewRunner(nil)
	j, ok, err := r.ClaimNext(context.Background(), []string{" experiment_run ", "experiment_run", "tally"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Fatal("expected ok=true")
	}
	if j.ID != "job-1" || string(j.Payload) != `{"x":1}` {
		t.Fatalf("unexpected job: %#v", j)
	}
}

func TestMarkDone_WithResultRef(t *testing.T) {
	defer restoreJobHooks()()

	var gotArgs []any
	runnerExecFn = func(_ context.Context, _ *pgxpool.Pool, _ string, args ...any) error {
		gotArgs = args
		return nil
	}

	r := NewRunner(nil)
	err := r.MarkDone(context.Background(), "11111111-1111-1111-1111-111111111111", map[string]any{"x": 1})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(gotArgs) != 2 {
		t.Fatalf("unexpected args: %#v", gotArgs)
	}
	if gotArgs[1] == nil {
		t.Fatal("expected serialized result_ref")
	}
}

func TestMarkError_DefaultTextAndTrim(t *testing.T) {
	defer restoreJobHooks()()

	var gotArgs []any
	runnerExecFn = func(_ context.Context, _ *pgxpool.Pool, _ string, args ...any) error {
		gotArgs = args
		return nil
	}

	r := NewRunner(nil)
	if err := r.MarkError(context.Background(), "11111111-1111-1111-1111-111111111111", "   "); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotArgs[1] != "job failed" {
		t.Fatalf("expected default job failed, got %#v", gotArgs[1])
	}

	if err := r.MarkError(context.Background(), "11111111-1111-1111-1111-111111111111", "  boom  "); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotArgs[1] != "boom" {
		t.Fatalf("expected trimmed text, got %#v", gotArgs[1])
	}
}

func TestUpdateProgress_Clamp(t *testing.T) {
	defer restoreJobHooks()()

	var gotArgs []any
	runnerExecFn = func(_ context.Context, _ *pgxpool.Pool, _ string, args ...any) error {
		gotArgs = args
		return nil
	}

	r := NewRunner(nil)

	if err := r.UpdateProgress(context.Background(), "11111111-1111-1111-1111-111111111111", -5); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotArgs[1] != 0 {
		t.Fatalf("expected progress 0, got %#v", gotArgs[1])
	}

	if err := r.UpdateProgress(context.Background(), "11111111-1111-1111-1111-111111111111", 150); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotArgs[1] != 100 {
		t.Fatalf("expected progress 100, got %#v", gotArgs[1])
	}
}
