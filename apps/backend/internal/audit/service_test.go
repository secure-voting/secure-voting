package audit

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

type fakeRows struct {
	items    []Record
	idx      int
	scanErr  error
	rowsErr  error
	closed   bool
	details  []any
	occurs   []time.Time
	actorIDs []*string
}

func (r *fakeRows) Next() bool {
	return r.idx < len(r.items)
}

func (r *fakeRows) Scan(dest ...any) error {
	if r.scanErr != nil {
		return r.scanErr
	}
	i := r.idx
	if i >= len(r.items) {
		return errors.New("out of range")
	}
	*(dest[0].(*int64)) = r.items[i].ID
	*(dest[1].(*time.Time)) = r.occurs[i]
	*(dest[2].(**string)) = r.actorIDs[i]
	*(dest[3].(*string)) = r.items[i].EventType
	*(dest[4].(*any)) = r.details[i]
	r.idx++
	return nil
}

func (r *fakeRows) Close() { r.closed = true }

func (r *fakeRows) Err() error { return r.rowsErr }

func restoreAuditHooks() func() {
	old := auditQueryFn
	return func() { auditQueryFn = old }
}

func TestNewService(t *testing.T) {
	svc := NewService(nil)
	if svc == nil {
		t.Fatal("expected service")
	}
}

func TestParseUUIDOrEmpty(t *testing.T) {
	if got, ok := ParseUUIDOrEmpty(""); got != "" || !ok {
		t.Fatalf("unexpected empty parse: %q %v", got, ok)
	}

	id := "11111111-1111-1111-1111-111111111111"
	if got, ok := ParseUUIDOrEmpty(id); got != id || !ok {
		t.Fatalf("unexpected valid parse: %q %v", got, ok)
	}

	if _, ok := ParseUUIDOrEmpty("bad"); ok {
		t.Fatal("expected invalid uuid")
	}
}

func TestParseTimeRFC3339(t *testing.T) {
	svc := NewService(nil)

	got, err := svc.ParseTimeRFC3339("")
	if err != nil || got != nil {
		t.Fatalf("unexpected empty parse: %v %#v", err, got)
	}

	got, err = svc.ParseTimeRFC3339("2026-03-18T12:00:00Z")
	if err != nil || got == nil {
		t.Fatalf("unexpected valid parse: %v %#v", err, got)
	}

	if _, err := svc.ParseTimeRFC3339("bad"); err == nil {
		t.Fatal("expected parse error")
	}
}

func TestParseIntAndItoa(t *testing.T) {
	if n, ok := ParseInt(""); n != 0 || ok {
		t.Fatalf("unexpected empty ParseInt: %d %v", n, ok)
	}
	if n, ok := ParseInt("42"); n != 42 || !ok {
		t.Fatalf("unexpected ParseInt: %d %v", n, ok)
	}
	if _, ok := ParseInt("xx"); ok {
		t.Fatal("expected invalid int")
	}
	if itoa(15) != "15" {
		t.Fatalf("unexpected itoa")
	}
}

func TestList_QueryError(t *testing.T) {
	defer restoreAuditHooks()()

	auditQueryFn = func(_ context.Context, _ any, _ string, _ ...any) (rowsScanner, error) {
		return nil, errors.New("boom")
	}

	svc := NewService(nil)
	_, err := svc.List(context.Background(), "admin", "", ListFilter{})
	if err == nil || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("expected boom, got %v", err)
	}
}

func TestList_ScanError(t *testing.T) {
	defer restoreAuditHooks()()

	now := time.Date(2026, 3, 18, 12, 0, 0, 0, time.UTC)
	rows := &fakeRows{
		items:    []Record{{ID: 1, EventType: "evt"}},
		occurs:   []time.Time{now},
		actorIDs: []*string{nil},
		details:  []any{map[string]any{}},
		scanErr:  errors.New("scan boom"),
	}
	auditQueryFn = func(_ context.Context, _ any, _ string, _ ...any) (rowsScanner, error) {
		return rows, nil
	}

	svc := NewService(nil)
	_, err := svc.List(context.Background(), "admin", "", ListFilter{})
	if err == nil || !strings.Contains(err.Error(), "scan boom") {
		t.Fatalf("expected scan boom, got %v", err)
	}
}

func TestList_RowsErr(t *testing.T) {
	defer restoreAuditHooks()()

	rows := &fakeRows{rowsErr: errors.New("rows boom")}
	auditQueryFn = func(_ context.Context, _ any, _ string, _ ...any) (rowsScanner, error) {
		return rows, nil
	}

	svc := NewService(nil)
	_, err := svc.List(context.Background(), "admin", "", ListFilter{})
	if err == nil || !strings.Contains(err.Error(), "rows boom") {
		t.Fatalf("expected rows boom, got %v", err)
	}
}

func TestList_SuccessAdmin(t *testing.T) {
	defer restoreAuditHooks()()

	now := time.Date(2026, 3, 18, 12, 0, 0, 0, time.UTC)
	actor := "11111111-1111-1111-1111-111111111111"
	rows := &fakeRows{
		items: []Record{
			{ID: 1, EventType: "evt1"},
			{ID: 2, EventType: "evt2"},
		},
		occurs:   []time.Time{now, now.Add(-time.Hour)},
		actorIDs: []*string{&actor, nil},
		details:  []any{map[string]any{"a": 1}, map[string]any{"b": 2}},
	}

	var gotQuery string
	var gotArgs []any
	auditQueryFn = func(_ context.Context, _ any, q string, args ...any) (rowsScanner, error) {
		gotQuery = q
		gotArgs = args
		return rows, nil
	}

	svc := NewService(nil)
	items, err := svc.List(context.Background(), "admin", "", ListFilter{Limit: 10, Offset: 5})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(items))
	}
	if !rows.closed {
		t.Fatal("expected rows to be closed")
	}
	if !strings.Contains(gotQuery, "ORDER BY occurred_at DESC") {
		t.Fatalf("unexpected query: %q", gotQuery)
	}
	if len(gotArgs) != 2 {
		t.Fatalf("unexpected args: %#v", gotArgs)
	}
}

func TestList_SuccessNonAdminWithFilters(t *testing.T) {
	defer restoreAuditHooks()()

	now := time.Date(2026, 3, 18, 12, 0, 0, 0, time.UTC)
	eventType := "evt"
	actorID := "22222222-2222-2222-2222-222222222222"

	rows := &fakeRows{
		items:    []Record{},
		occurs:   []time.Time{},
		actorIDs: []*string{},
		details:  []any{},
	}

	var gotQuery string
	var gotArgs []any
	auditQueryFn = func(_ context.Context, _ any, q string, args ...any) (rowsScanner, error) {
		gotQuery = q
		gotArgs = args
		return rows, nil
	}

	svc := NewService(nil)
	_, err := svc.List(context.Background(), "voter", "user-1", ListFilter{
		EventType:   &eventType,
		ActorUserID: &actorID,
		Since:       &now,
		Until:       &now,
		Limit:       -1,
		Offset:      3,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(gotQuery, "actor_user_id = $1") {
		t.Fatalf("expected role filter in query: %q", gotQuery)
	}
	if len(gotArgs) != 7 {
		t.Fatalf("unexpected args: %#v", gotArgs)
	}
}
