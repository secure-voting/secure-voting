package auth

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"golang.org/x/crypto/bcrypt"
)

type fakeRow struct {
	scanFn func(dest ...any) error
}

func (r fakeRow) Scan(dest ...any) error {
	return r.scanFn(dest...)
}

type fakeTx struct {
	execFn     func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	queryRowFn func(ctx context.Context, sql string, args ...any) rowScanner
	commitFn   func(ctx context.Context) error
	rollbackFn func(ctx context.Context) error
}

func (tx *fakeTx) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	if tx.execFn != nil {
		return tx.execFn(ctx, sql, args...)
	}
	return pgconn.NewCommandTag("INSERT 0 1"), nil
}

func (tx *fakeTx) QueryRow(ctx context.Context, sql string, args ...any) rowScanner {
	if tx.queryRowFn != nil {
		return tx.queryRowFn(ctx, sql, args...)
	}
	return fakeRow{scanFn: func(dest ...any) error { return nil }}
}

func (tx *fakeTx) Commit(ctx context.Context) error {
	if tx.commitFn != nil {
		return tx.commitFn(ctx)
	}
	return nil
}

func (tx *fakeTx) Rollback(ctx context.Context) error {
	if tx.rollbackFn != nil {
		return tx.rollbackFn(ctx)
	}
	return nil
}

func restoreAuthHooks() func() {
	oldBegin := authBeginTxFn
	oldQueryRow := authDBQueryRowFn
	oldExec := authDBExecFn
	oldRand := randReadFn
	oldNow := nowFn
	return func() {
		authBeginTxFn = oldBegin
		authDBQueryRowFn = oldQueryRow
		authDBExecFn = oldExec
		randReadFn = oldRand
		nowFn = oldNow
	}
}

func TestNewServiceAndValidation(t *testing.T) {
	svc := NewService(nil, time.Hour)
	if svc == nil {
		t.Fatal("expected service")
	}

	if !ValidateEmail("user@example.com") {
		t.Fatal("expected valid email")
	}
	if ValidateEmail("bad") {
		t.Fatal("expected invalid email")
	}

	if !ValidatePassword("12345678") {
		t.Fatal("expected valid password")
	}
	if ValidatePassword("1234567") {
		t.Fatal("expected invalid password")
	}
}

func TestSHA256Hex(t *testing.T) {
	got := sha256Hex("abc")
	want := "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad"
	if got != want {
		t.Fatalf("unexpected sha256: %q", got)
	}
}

func TestIssueToken_Success(t *testing.T) {
	defer restoreAuthHooks()()

	nowFn = func() time.Time {
		return time.Date(2026, 3, 18, 12, 0, 0, 0, time.UTC)
	}
	randReadFn = func(b []byte) (int, error) {
		for i := range b {
			b[i] = byte(i)
		}
		return len(b), nil
	}

	var gotArgs []any
	tx := &fakeTx{
		execFn: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
			gotArgs = args
			return pgconn.NewCommandTag("INSERT 0 1"), nil
		},
	}

	svc := NewService(nil, 2*time.Hour)
	token, tokenHash, expiresAt, err := svc.issueToken(context.Background(), tx, "11111111-1111-1111-1111-111111111111")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(token) != 64 {
		t.Fatalf("unexpected token len: %d", len(token))
	}
	if tokenHash != sha256Hex(token) {
		t.Fatalf("unexpected token hash")
	}
	wantExp := time.Date(2026, 3, 18, 14, 0, 0, 0, time.UTC)
	if !expiresAt.Equal(wantExp) {
		t.Fatalf("unexpected expiresAt: %v", expiresAt)
	}
	if len(gotArgs) != 4 {
		t.Fatalf("unexpected exec args: %#v", gotArgs)
	}
}

func TestIssueToken_RandError(t *testing.T) {
	defer restoreAuthHooks()()

	randReadFn = func(b []byte) (int, error) {
		return 0, errors.New("rand boom")
	}

	svc := NewService(nil, time.Hour)
	_, _, _, err := svc.issueToken(context.Background(), &fakeTx{}, "u")
	if err == nil || !strings.Contains(err.Error(), "rand boom") {
		t.Fatalf("expected rand boom, got %v", err)
	}
}

func TestIssueToken_ExecError(t *testing.T) {
	defer restoreAuthHooks()()

	randReadFn = func(b []byte) (int, error) {
		for i := range b {
			b[i] = 1
		}
		return len(b), nil
	}

	tx := &fakeTx{
		execFn: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
			return pgconn.CommandTag{}, errors.New("exec boom")
		},
	}

	svc := NewService(nil, time.Hour)
	_, _, _, err := svc.issueToken(context.Background(), tx, "u")
	if err == nil || !strings.Contains(err.Error(), "exec boom") {
		t.Fatalf("expected exec boom, got %v", err)
	}
}

func TestVerifyAccessToken(t *testing.T) {
	defer restoreAuthHooks()()

	svc := NewService(nil, time.Hour)

	userID, email, role, ok, err := svc.VerifyAccessToken(context.Background(), "")
	if err != nil || ok || userID != "" || email != "" || role != "" {
		t.Fatalf("unexpected empty token result")
	}

	authDBQueryRowFn = func(ctx context.Context, _ any, q string, args ...any) rowScanner {
		return fakeRow{scanFn: func(dest ...any) error { return pgx.ErrNoRows }}
	}
	_, _, _, ok, err = svc.VerifyAccessToken(context.Background(), "abc")
	if err != nil || ok {
		t.Fatalf("expected not found token")
	}

	authDBQueryRowFn = func(ctx context.Context, _ any, q string, args ...any) rowScanner {
		return fakeRow{scanFn: func(dest ...any) error { return errors.New("db boom") }}
	}
	_, _, _, ok, err = svc.VerifyAccessToken(context.Background(), "abc")
	if err == nil || ok || !strings.Contains(err.Error(), "db boom") {
		t.Fatalf("expected db boom, got ok=%v err=%v", ok, err)
	}

	authDBQueryRowFn = func(ctx context.Context, _ any, q string, args ...any) rowScanner {
		return fakeRow{scanFn: func(dest ...any) error {
			*(dest[0].(*string)) = "u1"
			*(dest[1].(*string)) = "user@example.com"
			*(dest[2].(*string)) = "voter"
			*(dest[3].(*time.Time)) = time.Now().Add(time.Hour)
			return nil
		}}
	}
	userID, email, role, ok, err = svc.VerifyAccessToken(context.Background(), "abc")
	if err != nil || !ok || userID != "u1" || email != "user@example.com" || role != "voter" {
		t.Fatalf("unexpected success result: %q %q %q %v %v", userID, email, role, ok, err)
	}
}

func TestInsertAudit(t *testing.T) {
	svc := NewService(nil, time.Hour)

	tx := &fakeTx{}
	if err := svc.insertAudit(context.Background(), tx, nil, "evt", nil); err != nil {
		t.Fatalf("unexpected nil-details audit error: %v", err)
	}

	actor := "11111111-1111-1111-1111-111111111111"
	if err := svc.insertAudit(context.Background(), tx, &actor, "evt", map[string]any{"k": "v"}); err != nil {
		t.Fatalf("unexpected actor audit error: %v", err)
	}

	err := svc.insertAudit(context.Background(), tx, &actor, "evt", map[string]any{"bad": func() {}})
	if err == nil {
		t.Fatal("expected marshal error")
	}
}

func TestEnsureBootstrapUser(t *testing.T) {
	defer restoreAuthHooks()()

	called := 0
	authDBExecFn = func(ctx context.Context, _ any, q string, args ...any) (pgconn.CommandTag, error) {
		called++
		return pgconn.NewCommandTag("INSERT 0 1"), nil
	}

	if err := EnsureBootstrapUser(context.Background(), nil, "", "", "admin"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if called != 0 {
		t.Fatal("exec should not be called")
	}

	if err := EnsureBootstrapUser(context.Background(), nil, "bad", "12345678", "admin"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if called != 0 {
		t.Fatal("exec should not be called for invalid email")
	}

	if err := EnsureBootstrapUser(context.Background(), nil, "user@example.com", "123", "admin"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if called != 0 {
		t.Fatal("exec should not be called for invalid password")
	}

	if err := EnsureBootstrapUser(context.Background(), nil, "user@example.com", "12345678", "voter"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if called != 0 {
		t.Fatal("exec should not be called for invalid role")
	}

	if err := EnsureBootstrapUser(context.Background(), nil, "user@example.com", "12345678", "admin"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if called != 1 {
		t.Fatalf("expected one exec call, got %d", called)
	}

	authDBExecFn = func(ctx context.Context, _ any, q string, args ...any) (pgconn.CommandTag, error) {
		return pgconn.CommandTag{}, errors.New("exec boom")
	}
	if err := EnsureBootstrapUser(context.Background(), nil, "user@example.com", "12345678", "admin"); err == nil || !strings.Contains(err.Error(), "exec boom") {
		t.Fatalf("expected exec boom, got %v", err)
	}
}

func TestAcceptInviteTx(t *testing.T) {
	svc := NewService(nil, time.Hour)

	tx := &fakeTx{}
	got, code, err := svc.acceptInviteTx(context.Background(), tx, "user@example.com", "")
	if err != nil || code != "" || got != (acceptedInvite{}) {
		t.Fatalf("unexpected empty invite result: %#v %q %v", got, code, err)
	}

	tx = &fakeTx{
		queryRowFn: func(ctx context.Context, sql string, args ...any) rowScanner {
			return fakeRow{scanFn: func(dest ...any) error { return pgx.ErrNoRows }}
		},
	}
	_, code, err = svc.acceptInviteTx(context.Background(), tx, "user@example.com", "code")
	if err != nil || code != "invalid_invite_code" {
		t.Fatalf("unexpected invalid invite result: %q %v", code, err)
	}

	tx = &fakeTx{
		queryRowFn: func(ctx context.Context, sql string, args ...any) rowScanner {
			return fakeRow{scanFn: func(dest ...any) error {
				*(dest[0].(*string)) = "inv1"
				*(dest[1].(*string)) = "user@example.com"
				*(dest[2].(*string)) = "accepted"
				*(dest[3].(*string)) = "e1"
				return nil
			}}
		},
	}
	_, code, err = svc.acceptInviteTx(context.Background(), tx, "user@example.com", "code")
	if err != nil || code != "invite_code_inactive" {
		t.Fatalf("unexpected inactive invite result: %q %v", code, err)
	}

	tx = &fakeTx{
		queryRowFn: func(ctx context.Context, sql string, args ...any) rowScanner {
			return fakeRow{scanFn: func(dest ...any) error {
				*(dest[0].(*string)) = "inv1"
				*(dest[1].(*string)) = "other@example.com"
				*(dest[2].(*string)) = "created"
				*(dest[3].(*string)) = "e1"
				return nil
			}}
		},
	}
	_, code, err = svc.acceptInviteTx(context.Background(), tx, "user@example.com", "code")
	if err != nil || code != "invite_email_mismatch" {
		t.Fatalf("unexpected email mismatch result: %q %v", code, err)
	}

	tx = &fakeTx{
		queryRowFn: func(ctx context.Context, sql string, args ...any) rowScanner {
			return fakeRow{scanFn: func(dest ...any) error {
				*(dest[0].(*string)) = "11111111-1111-1111-1111-111111111111"
				*(dest[1].(*string)) = "user@example.com"
				*(dest[2].(*string)) = "created"
				*(dest[3].(*string)) = "e1"
				return nil
			}}
		},
		execFn: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
			return pgconn.NewCommandTag("UPDATE 0"), nil
		},
	}
	_, code, err = svc.acceptInviteTx(context.Background(), tx, "user@example.com", "code")
	if err != nil || code != "invite_code_inactive" {
		t.Fatalf("unexpected zero-update result: %q %v", code, err)
	}

	tx = &fakeTx{
		queryRowFn: func(ctx context.Context, sql string, args ...any) rowScanner {
			return fakeRow{scanFn: func(dest ...any) error {
				*(dest[0].(*string)) = "11111111-1111-1111-1111-111111111111"
				*(dest[1].(*string)) = "user@example.com"
				*(dest[2].(*string)) = "sent"
				*(dest[3].(*string)) = "e1"
				return nil
			}}
		},
		execFn: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
			return pgconn.CommandTag{}, errors.New("update boom")
		},
	}
	_, code, err = svc.acceptInviteTx(context.Background(), tx, "user@example.com", "code")
	if err == nil || code != "" || !strings.Contains(err.Error(), "update boom") {
		t.Fatalf("unexpected update error result: %q %v", code, err)
	}

	tx = &fakeTx{
		queryRowFn: func(ctx context.Context, sql string, args ...any) rowScanner {
			return fakeRow{scanFn: func(dest ...any) error {
				*(dest[0].(*string)) = "11111111-1111-1111-1111-111111111111"
				*(dest[1].(*string)) = "user@example.com"
				*(dest[2].(*string)) = "created"
				*(dest[3].(*string)) = "e1"
				return nil
			}}
		},
		execFn: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
			return pgconn.NewCommandTag("UPDATE 1"), nil
		},
	}
	got, code, err = svc.acceptInviteTx(context.Background(), tx, "user@example.com", "code")
	if err != nil || code != "" || got.ID != "11111111-1111-1111-1111-111111111111" || got.ElectionID != "e1" {
		t.Fatalf("unexpected success invite result: %#v %q %v", got, code, err)
	}
}

func TestRegisterBasicValidation(t *testing.T) {
	svc := NewService(nil, time.Hour)

	_, code, err := svc.Register(context.Background(), "bad", "12345678", "", "")
	if err != nil || code != "invalid_email" {
		t.Fatalf("unexpected invalid email result: %q %v", code, err)
	}

	_, code, err = svc.Register(context.Background(), "user@example.com", "123", "", "")
	if err != nil || code != "invalid_password" {
		t.Fatalf("unexpected invalid password result: %q %v", code, err)
	}
}

func TestLoginBasicValidationAndCredentials(t *testing.T) {
	defer restoreAuthHooks()()

	svc := NewService(nil, time.Hour)

	_, code, err := svc.Login(context.Background(), "bad", "12345678", "")
	if err != nil || code != "invalid_email" {
		t.Fatalf("unexpected invalid email result: %q %v", code, err)
	}

	_, code, err = svc.Login(context.Background(), "user@example.com", "", "")
	if err != nil || code != "invalid_password" {
		t.Fatalf("unexpected invalid password result: %q %v", code, err)
	}

	authDBQueryRowFn = func(ctx context.Context, _ any, q string, args ...any) rowScanner {
		return fakeRow{scanFn: func(dest ...any) error { return pgx.ErrNoRows }}
	}
	_, code, err = svc.Login(context.Background(), "user@example.com", "12345678", "")
	if err != nil || code != "invalid_credentials" {
		t.Fatalf("unexpected invalid credentials result: %q %v", code, err)
	}

	hash, errHash := bcrypt.GenerateFromPassword([]byte("correct-pass"), 12)
	if errHash != nil {
		t.Fatalf("bcrypt error: %v", errHash)
	}
	authDBQueryRowFn = func(ctx context.Context, _ any, q string, args ...any) rowScanner {
		return fakeRow{scanFn: func(dest ...any) error {
			*(dest[0].(*string)) = "u1"
			*(dest[1].(*string)) = "user@example.com"
			*(dest[2].(*string)) = "voter"
			*(dest[3].(*string)) = string(hash)
			return nil
		}}
	}
	_, code, err = svc.Login(context.Background(), "user@example.com", "wrong-pass", "")
	if err != nil || code != "invalid_credentials" {
		t.Fatalf("unexpected wrong password result: %q %v", code, err)
	}
}

func TestLogout(t *testing.T) {
	defer restoreAuthHooks()()

	svc := NewService(nil, time.Hour)

	ok, err := svc.Logout(context.Background(), "", nil)
	if err != nil || ok {
		t.Fatalf("unexpected empty logout result: %v %v", ok, err)
	}

	authBeginTxFn = func(ctx context.Context, _ any) (txLike, error) {
		return &fakeTx{
			execFn: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
				return pgconn.NewCommandTag("DELETE 1"), nil
			},
		}, nil
	}
	ok, err = svc.Logout(context.Background(), "raw-token", nil)
	if err != nil || !ok {
		t.Fatalf("unexpected success logout result: %v %v", ok, err)
	}
}
