package results

import (
	"context"
	"errors"
	"fmt"
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

func assignScan(dest any, v any) error {
	switch d := dest.(type) {
	case *string:
		s, ok := v.(string)
		if !ok {
			return fmt.Errorf("want string got %T", v)
		}
		*d = s
		return nil

	case *bool:
		b, ok := v.(bool)
		if !ok {
			return fmt.Errorf("want bool got %T", v)
		}
		*d = b
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

func restoreResultHooks() func() {
	old := resultQueryRowFn
	return func() {
		resultQueryRowFn = old
	}
}

func TestNewService(t *testing.T) {
	svc := NewService(nil)
	if svc == nil {
		t.Fatal("expected service")
	}
}

func TestGet_InvalidID(t *testing.T) {
	svc := NewService(nil)

	_, code, err := svc.Get(context.Background(), "bad", "admin", "u1", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if code != "invalid_id" {
		t.Fatalf("expected invalid_id, got %q", code)
	}
}

func TestGet_ElectionNotFound(t *testing.T) {
	defer restoreResultHooks()()

	resultQueryRowFn = func(_ context.Context, _ *pgxpool.Pool, _ string, _ ...any) rowScanner {
		return fakeRow{scanFn: func(dest ...any) error { return pgx.ErrNoRows }}
	}

	svc := NewService(nil)
	_, code, err := svc.Get(context.Background(), "11111111-1111-1111-1111-111111111111", "admin", "u1", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if code != "not_found" {
		t.Fatalf("expected not_found, got %q", code)
	}
}

func TestGet_AdminNotOwner(t *testing.T) {
	defer restoreResultHooks()()

	call := 0
	resultQueryRowFn = func(_ context.Context, _ *pgxpool.Pool, _ string, _ ...any) rowScanner {
		call++
		return fakeRow{
			scanFn: func(dest ...any) error {
				row := []any{"published", "open", true, "owner-2"}
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
	_, code, err := svc.Get(context.Background(), "11111111-1111-1111-1111-111111111111", "admin", "owner-1", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if code != "not_found" {
		t.Fatalf("expected not_found, got %q", code)
	}
	if call != 1 {
		t.Fatalf("expected one query, got %d", call)
	}
}

func TestGet_InviteNotAccepted(t *testing.T) {
	defer restoreResultHooks()()

	call := 0
	resultQueryRowFn = func(_ context.Context, _ *pgxpool.Pool, _ string, _ ...any) rowScanner {
		call++
		if call == 1 {
			return fakeRow{
				scanFn: func(dest ...any) error {
					row := []any{"published", "invite", true, "owner-1"}
					for i := range dest {
						if err := assignScan(dest[i], row[i]); err != nil {
							return err
						}
					}
					return nil
				},
			}
		}
		return fakeRow{scanFn: func(dest ...any) error { return pgx.ErrNoRows }}
	}

	svc := NewService(nil)
	_, code, err := svc.Get(context.Background(), "11111111-1111-1111-1111-111111111111", "voter", "u1", "user@example.com")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if code != "not_found" {
		t.Fatalf("expected not_found, got %q", code)
	}
}

func TestGet_NotPublishedForVoter(t *testing.T) {
	defer restoreResultHooks()()

	call := 0
	resultQueryRowFn = func(_ context.Context, _ *pgxpool.Pool, _ string, _ ...any) rowScanner {
		call++
		return fakeRow{
			scanFn: func(dest ...any) error {
				switch call {
				case 1:
					row := []any{"closed", "open", true, "owner-1"}
					for i := range dest {
						if err := assignScan(dest[i], row[i]); err != nil {
							return err
						}
					}
					return nil
				default:
					return errors.New("unexpected extra query")
				}
			},
		}
	}

	svc := NewService(nil)
	_, code, err := svc.Get(context.Background(), "11111111-1111-1111-1111-111111111111", "voter", "u1", "user@example.com")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if code != "not_published" {
		t.Fatalf("expected not_published, got %q", code)
	}
}

func TestGet_NoResults(t *testing.T) {
	defer restoreResultHooks()()

	call := 0
	resultQueryRowFn = func(_ context.Context, _ *pgxpool.Pool, _ string, _ ...any) rowScanner {
		call++
		if call == 1 {
			return fakeRow{
				scanFn: func(dest ...any) error {
					row := []any{"published", "open", true, "owner-1"}
					for i := range dest {
						if err := assignScan(dest[i], row[i]); err != nil {
							return err
						}
					}
					return nil
				},
			}
		}
		return fakeRow{scanFn: func(dest ...any) error { return pgx.ErrNoRows }}
	}

	svc := NewService(nil)
	_, code, err := svc.Get(context.Background(), "11111111-1111-1111-1111-111111111111", "voter", "u1", "user@example.com")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if code != "no_results" {
		t.Fatalf("expected no_results, got %q", code)
	}
}

func TestGet_VoterHideAggregates(t *testing.T) {
	defer restoreResultHooks()()

	call := 0
	now := time.Date(2026, 3, 18, 13, 0, 0, 0, time.UTC)

	resultQueryRowFn = func(_ context.Context, _ *pgxpool.Pool, _ string, _ ...any) rowScanner {
		call++
		return fakeRow{
			scanFn: func(dest ...any) error {
				switch call {
				case 1:
					row := []any{"published", "open", false, "owner-1"}
					for i := range dest {
						if err := assignScan(dest[i], row[i]); err != nil {
							return err
						}
					}
					return nil
				case 2:
					row := []any{
						2,
						"plurality",
						[]byte(`{"committee_size":1}`),
						[]byte(`["c1"]`),
						[]byte(`{"sum":1}`),
						[]byte(`[{"round":1}]`),
						now,
					}
					for i := range dest {
						if err := assignScan(dest[i], row[i]); err != nil {
							return err
						}
					}
					return nil
				default:
					return errors.New("unexpected query")
				}
			},
		}
	}

	svc := NewService(nil)
	res, code, err := svc.Get(context.Background(), "11111111-1111-1111-1111-111111111111", "voter", "u1", "user@example.com")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if code != "" {
		t.Fatalf("unexpected code: %q", code)
	}
	if string(res.Winners) != `["c1"]` {
		t.Fatalf("unexpected winners: %s", string(res.Winners))
	}
	if res.Params != nil || res.Metrics != nil || res.Protocol != nil {
		t.Fatalf("expected aggregates hidden, got %#v", res)
	}
	if res.PublishedAt == nil || *res.PublishedAt != now.Format(time.RFC3339) {
		t.Fatalf("unexpected published_at: %#v", res.PublishedAt)
	}
}

func TestGet_AdminKeepsAggregates(t *testing.T) {
	defer restoreResultHooks()()

	call := 0
	resultQueryRowFn = func(_ context.Context, _ *pgxpool.Pool, _ string, _ ...any) rowScanner {
		call++
		return fakeRow{
			scanFn: func(dest ...any) error {
				switch call {
				case 1:
					row := []any{"closed", "invite", false, "owner-1"}
					for i := range dest {
						if err := assignScan(dest[i], row[i]); err != nil {
							return err
						}
					}
					return nil
				case 2:
					row := []any{
						1,
						"plurality",
						[]byte(`{"committee_size":1}`),
						[]byte(`["c1"]`),
						[]byte(`{"sum":1}`),
						[]byte(`[{"round":1}]`),
						nil,
					}
					for i := range dest {
						if err := assignScan(dest[i], row[i]); err != nil {
							return err
						}
					}
					return nil
				default:
					return errors.New("unexpected query")
				}
			},
		}
	}

	svc := NewService(nil)
	res, code, err := svc.Get(context.Background(), "11111111-1111-1111-1111-111111111111", "admin", "owner-1", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if code != "" {
		t.Fatalf("unexpected code: %q", code)
	}
	if string(res.Params) != `{"committee_size":1}` {
		t.Fatalf("unexpected params: %s", string(res.Params))
	}
	if string(res.Metrics) != `{"sum":1}` {
		t.Fatalf("unexpected metrics: %s", string(res.Metrics))
	}
	if string(res.Protocol) != `[{"round":1}]` {
		t.Fatalf("unexpected protocol: %s", string(res.Protocol))
	}
}
