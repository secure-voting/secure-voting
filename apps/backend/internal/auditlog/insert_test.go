package auditlog

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
)

type fakeExecutor struct {
	sql  string
	args []any
	err  error
}

func (f *fakeExecutor) Exec(_ context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	f.sql = sql
	f.args = args
	if f.err != nil {
		return pgconn.CommandTag{}, f.err
	}
	return pgconn.NewCommandTag("INSERT 0 1"), nil
}

func TestInsert_WithActor(t *testing.T) {
	ex := &fakeExecutor{}
	actor := "11111111-1111-1111-1111-111111111111"

	err := Insert(context.Background(), ex, &actor, "evt", map[string]any{"k": "v"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(ex.sql, "actor_user_id") {
		t.Fatalf("unexpected sql: %q", ex.sql)
	}
	if len(ex.args) != 3 {
		t.Fatalf("unexpected args: %#v", ex.args)
	}
}

func TestInsert_WithoutActor(t *testing.T) {
	ex := &fakeExecutor{}

	err := Insert(context.Background(), ex, nil, "evt", map[string]any{"k": "v"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(ex.args) != 2 {
		t.Fatalf("unexpected args: %#v", ex.args)
	}
}

func TestInsert_NilDetails(t *testing.T) {
	ex := &fakeExecutor{}
	actor := "11111111-1111-1111-1111-111111111111"

	err := Insert(context.Background(), ex, &actor, "evt", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(ex.args) != 3 {
		t.Fatalf("unexpected args len: %#v", ex.args)
	}
}

func TestInsert_MarshalError(t *testing.T) {
	ex := &fakeExecutor{}
	actor := "11111111-1111-1111-1111-111111111111"

	err := Insert(context.Background(), ex, &actor, "evt", map[string]any{
		"bad": func() {},
	})
	if err == nil {
		t.Fatal("expected marshal error")
	}
}

func TestInsert_ExecError(t *testing.T) {
	ex := &fakeExecutor{err: errors.New("boom")}
	actor := "11111111-1111-1111-1111-111111111111"

	err := Insert(context.Background(), ex, &actor, "evt", map[string]any{"k": "v"})
	if err == nil || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("expected boom, got %v", err)
	}
}
