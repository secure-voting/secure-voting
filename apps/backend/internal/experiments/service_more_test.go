package experiments

import (
	"context"
	"encoding/json"
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

func (r *fakeRows) Err() error {
	return r.err
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

	case *[]byte:
		if v == nil {
			*d = nil
			return nil
		}
		b, ok := v.([]byte)
		if !ok {
			return fmt.Errorf("want []byte got %T", v)
		}
		*d = append([]byte(nil), b...)
		return nil

	case **int64:
		if v == nil {
			*d = nil
			return nil
		}
		n, ok := v.(int64)
		if !ok {
			return fmt.Errorf("want int64 got %T", v)
		}
		x := n
		*d = &x
		return nil

	case *time.Time:
		tv, ok := v.(time.Time)
		if !ok {
			return fmt.Errorf("want time.Time got %T", v)
		}
		*d = tv
		return nil

	default:
		return fmt.Errorf("unsupported dest %T", dest)
	}
}

func restoreExperimentHooks() func() {
	oldCreate := createExperimentQueryRowFn
	oldGet := getExperimentQueryRowFn
	oldList := listExperimentsQueryFn
	oldAudit := insertAuditFn

	return func() {
		createExperimentQueryRowFn = oldCreate
		getExperimentQueryRowFn = oldGet
		listExperimentsQueryFn = oldList
		insertAuditFn = oldAudit
	}
}

func TestNewService(t *testing.T) {
	svc := NewService(nil)
	if svc == nil {
		t.Fatal("expected service")
	}
}

func TestCreate_Unauthorized(t *testing.T) {
	svc := NewService(nil)

	_, code, err := svc.Create(context.Background(), "", CreateReq{Type: "algo"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if code != "unauthorized" {
		t.Fatalf("expected unauthorized, got %q", code)
	}
}

func TestCreate_InvalidType(t *testing.T) {
	svc := NewService(nil)

	_, code, err := svc.Create(context.Background(), "11111111-1111-1111-1111-111111111111", CreateReq{Type: "bad"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if code != "invalid_type" {
		t.Fatalf("expected invalid_type, got %q", code)
	}
}

func TestCreate_InvalidParams(t *testing.T) {
	svc := NewService(nil)

	_, code, err := svc.Create(context.Background(), "11111111-1111-1111-1111-111111111111", CreateReq{
		Type: "algo",
		Params: map[string]any{
			"ballot_format": "ranking",
			"score_min":     5,
			"score_max":     1,
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if code != "invalid_score_range" {
		t.Fatalf("expected invalid_score_range, got %q", code)
	}
}

func TestCreate_Success(t *testing.T) {
	defer restoreExperimentHooks()()

	var gotArgs []any
	createExperimentQueryRowFn = func(_ context.Context, _ *pgxpool.Pool, _ string, args ...any) rowScanner {
		gotArgs = args
		return fakeRow{
			scanFn: func(dest ...any) error {
				*(dest[0].(*string)) = "exp-1"
				return nil
			},
		}
	}
	auditCalled := false
	insertAuditFn = func(_ context.Context, _ *pgxpool.Pool, actorUserID, eventType string, details map[string]any) error {
		auditCalled = true
		if actorUserID != "11111111-1111-1111-1111-111111111111" {
			t.Fatalf("unexpected actor: %q", actorUserID)
		}
		if eventType != "experiment_created" {
			t.Fatalf("unexpected eventType: %q", eventType)
		}
		if details["target_type"] != "experiment" {
			t.Fatalf("unexpected details: %#v", details)
		}
		return nil
	}

	seed := int64(42)
	svc := NewService(nil)
	id, code, err := svc.Create(context.Background(), "11111111-1111-1111-1111-111111111111", CreateReq{
		Type: "algo",
		Params: map[string]any{
			"ballot_format":  "ranking",
			"tally_rule":     "plurality",
			"committee_size": 1,
		},
		Seed: &seed,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if code != "" {
		t.Fatalf("unexpected code: %q", code)
	}
	if id != "exp-1" {
		t.Fatalf("unexpected id: %q", id)
	}
	if !auditCalled {
		t.Fatal("expected audit to be called")
	}
	if len(gotArgs) != 4 {
		t.Fatalf("expected 4 args, got %d", len(gotArgs))
	}
}

func TestGet_InvalidID(t *testing.T) {
	svc := NewService(nil)

	_, code, err := svc.Get(context.Background(), "researcher", "u1", "bad")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if code != "invalid_id" {
		t.Fatalf("expected invalid_id, got %q", code)
	}
}

func TestGet_NotFound(t *testing.T) {
	defer restoreExperimentHooks()()

	getExperimentQueryRowFn = func(_ context.Context, _ *pgxpool.Pool, _ string, _ ...any) rowScanner {
		return fakeRow{
			scanFn: func(dest ...any) error {
				return pgx.ErrNoRows
			},
		}
	}

	svc := NewService(nil)
	_, code, err := svc.Get(context.Background(), "researcher", "11111111-1111-1111-1111-111111111111", "22222222-2222-2222-2222-222222222222")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if code != "not_found" {
		t.Fatalf("expected not_found, got %q", code)
	}
}

func TestGet_ACLHiddenForNonAdmin(t *testing.T) {
	defer restoreExperimentHooks()()

	now := time.Date(2026, 3, 18, 0, 0, 0, 0, time.UTC)
	getExperimentQueryRowFn = func(_ context.Context, _ *pgxpool.Pool, _ string, _ ...any) rowScanner {
		return fakeRow{
			scanFn: func(dest ...any) error {
				row := []any{
					"22222222-2222-2222-2222-222222222222",
					"algo",
					[]byte(`{"ballot_format":"ranking"}`),
					"draft",
					int64(42),
					"33333333-3333-3333-3333-333333333333",
					now,
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
	_, code, err := svc.Get(context.Background(), "researcher", "11111111-1111-1111-1111-111111111111", "22222222-2222-2222-2222-222222222222")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if code != "not_found" {
		t.Fatalf("expected not_found, got %q", code)
	}
}

func TestGet_Success(t *testing.T) {
	defer restoreExperimentHooks()()

	now := time.Date(2026, 3, 18, 1, 2, 3, 0, time.UTC)
	getExperimentQueryRowFn = func(_ context.Context, _ *pgxpool.Pool, _ string, _ ...any) rowScanner {
		return fakeRow{
			scanFn: func(dest ...any) error {
				row := []any{
					"22222222-2222-2222-2222-222222222222",
					"algo",
					[]byte(`{"ballot_format":"ranking"}`),
					"draft",
					int64(42),
					"11111111-1111-1111-1111-111111111111",
					now,
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
	e, code, err := svc.Get(context.Background(), "researcher", "11111111-1111-1111-1111-111111111111", "22222222-2222-2222-2222-222222222222")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if code != "" {
		t.Fatalf("unexpected code: %q", code)
	}
	if e.ID != "22222222-2222-2222-2222-222222222222" {
		t.Fatalf("unexpected id: %q", e.ID)
	}
	if string(e.Params) != `{"ballot_format":"ranking"}` {
		t.Fatalf("unexpected params: %s", string(e.Params))
	}
	if e.CreatedAt != now.Format(time.RFC3339) {
		t.Fatalf("unexpected created_at: %q", e.CreatedAt)
	}
	if e.Seed == nil || *e.Seed != 42 {
		t.Fatalf("unexpected seed: %#v", e.Seed)
	}
}

func TestList_DefaultsAndFiltersForNonAdmin(t *testing.T) {
	defer restoreExperimentHooks()()

	var capturedQuery string
	var capturedArgs []any
	now := time.Date(2026, 3, 18, 4, 5, 6, 0, time.UTC)

	listExperimentsQueryFn = func(_ context.Context, _ *pgxpool.Pool, q string, args ...any) (rowsScanner, error) {
		capturedQuery = q
		capturedArgs = args
		return &fakeRows{
			rows: [][]any{
				{
					"exp-1",
					"algo",
					[]byte(`{"ballot_format":"ranking"}`),
					"draft",
					nil,
					"11111111-1111-1111-1111-111111111111",
					now,
				},
			},
		}, nil
	}

	svc := NewService(nil)
	items, err := svc.List(context.Background(), "researcher", "11111111-1111-1111-1111-111111111111", ListParams{
		Type:   " ALGO ",
		Status: "draft",
		Limit:  0,
		Offset: -10,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 1 || items[0].ID != "exp-1" {
		t.Fatalf("unexpected items: %#v", items)
	}
	if !strings.Contains(capturedQuery, "created_by = $1::uuid") {
		t.Fatalf("expected created_by filter in query: %s", capturedQuery)
	}
	if !strings.Contains(capturedQuery, "type = $2") {
		t.Fatalf("expected type filter in query: %s", capturedQuery)
	}
	if !strings.Contains(capturedQuery, "status = $3") {
		t.Fatalf("expected status filter in query: %s", capturedQuery)
	}
	if len(capturedArgs) != 5 {
		t.Fatalf("unexpected args len: %d args=%#v", len(capturedArgs), capturedArgs)
	}
	if capturedArgs[0] != "11111111-1111-1111-1111-111111111111" {
		t.Fatalf("unexpected user arg: %#v", capturedArgs[0])
	}
	if capturedArgs[1] != "algo" {
		t.Fatalf("expected normalized type algo, got %#v", capturedArgs[1])
	}
	if capturedArgs[2] != "draft" {
		t.Fatalf("unexpected status arg: %#v", capturedArgs[2])
	}
	if capturedArgs[3] != 50 || capturedArgs[4] != 0 {
		t.Fatalf("unexpected limit/offset args: %#v", capturedArgs[3:])
	}
}

func TestList_AdminNoOwnerFilter(t *testing.T) {
	defer restoreExperimentHooks()()

	var capturedQuery string
	listExperimentsQueryFn = func(_ context.Context, _ *pgxpool.Pool, q string, args ...any) (rowsScanner, error) {
		capturedQuery = q
		return &fakeRows{}, nil
	}

	svc := NewService(nil)
	_, err := svc.List(context.Background(), "admin", "", ListParams{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.Contains(capturedQuery, "created_by =") {
		t.Fatalf("did not expect created_by filter for admin: %s", capturedQuery)
	}
}

func TestList_QueryError(t *testing.T) {
	defer restoreExperimentHooks()()

	listExperimentsQueryFn = func(_ context.Context, _ *pgxpool.Pool, _ string, _ ...any) (rowsScanner, error) {
		return nil, errors.New("boom")
	}

	svc := NewService(nil)
	_, err := svc.List(context.Background(), "admin", "", ListParams{})
	if err == nil || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("expected boom, got %v", err)
	}
}

func TestList_RowsErr(t *testing.T) {
	defer restoreExperimentHooks()()

	listExperimentsQueryFn = func(_ context.Context, _ *pgxpool.Pool, _ string, _ ...any) (rowsScanner, error) {
		return &fakeRows{err: errors.New("rows boom")}, nil
	}

	svc := NewService(nil)
	_, err := svc.List(context.Background(), "admin", "", ListParams{})
	if err == nil || !strings.Contains(err.Error(), "rows boom") {
		t.Fatalf("expected rows boom, got %v", err)
	}
}

func TestCreate_StoresNilParamsAsEmptyObject(t *testing.T) {
	defer restoreExperimentHooks()()

	var paramsArg string
	createExperimentQueryRowFn = func(_ context.Context, _ *pgxpool.Pool, _ string, args ...any) rowScanner {
		paramsArg = args[1].(string)
		return fakeRow{
			scanFn: func(dest ...any) error {
				*(dest[0].(*string)) = "exp-2"
				return nil
			},
		}
	}
	insertAuditFn = func(_ context.Context, _ *pgxpool.Pool, _ string, _ string, _ map[string]any) error { return nil }

	svc := NewService(nil)
	id, code, err := svc.Create(context.Background(), "11111111-1111-1111-1111-111111111111", CreateReq{
		Type: "algo",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if code != "" || id != "exp-2" {
		t.Fatalf("unexpected result: id=%q code=%q err=%v", id, code, err)
	}

	var parsed map[string]any
	if err := json.Unmarshal([]byte(paramsArg), &parsed); err != nil {
		t.Fatalf("unmarshal stored params: %v", err)
	}
	if len(parsed) != 0 {
		t.Fatalf("expected empty params object, got %#v", parsed)
	}
}
