package auth

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"database/sql"

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
	if len(gotArgs) != 5 {
		t.Fatalf("unexpected exec args: %#v", gotArgs)
	}
	if gotArgs[1] != nil {
		t.Fatalf("legacy issueToken should not attach session_id, got %#v", gotArgs[1])
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

func TestIssueTokenPair_Success(t *testing.T) {
	defer restoreAuthHooks()()

	nowFn = func() time.Time {
		return time.Date(2026, 4, 28, 10, 0, 0, 0, time.UTC)
	}
	randReadFn = func(b []byte) (int, error) {
		for i := range b {
			b[i] = byte((i % 16) + 1)
		}
		return len(b), nil
	}

	sessionInserted := false
	accessInserted := false

	tx := &fakeTx{
		queryRowFn: func(ctx context.Context, sqlText string, args ...any) rowScanner {
			if strings.Contains(sqlText, "INSERT INTO auth_sessions") {
				sessionInserted = true
				return fakeRow{scanFn: func(dest ...any) error {
					*(dest[0].(*string)) = "11111111-1111-1111-1111-111111111111"
					return nil
				}}
			}
			return fakeRow{scanFn: func(dest ...any) error {
				return errors.New("unexpected query row")
			}}
		},
		execFn: func(ctx context.Context, sqlText string, args ...any) (pgconn.CommandTag, error) {
			if strings.Contains(sqlText, "INSERT INTO api_tokens") {
				accessInserted = true
				if len(args) != 5 {
					t.Fatalf("unexpected api token args: %#v", args)
				}
				if args[1] != "11111111-1111-1111-1111-111111111111" {
					t.Fatalf("expected session id in access token insert, got %#v", args[1])
				}
				return pgconn.NewCommandTag("INSERT 0 1"), nil
			}
			return pgconn.CommandTag{}, errors.New("unexpected exec")
		},
	}

	svc := NewServiceWithRefreshTTL(nil, 15*time.Minute, 30*24*time.Hour)
	pair, err := svc.issueTokenPair(
		context.Background(),
		tx,
		"22222222-2222-2222-2222-222222222222",
		"test-agent",
		"127.0.0.1",
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !sessionInserted {
		t.Fatal("expected auth session insert")
	}
	if !accessInserted {
		t.Fatal("expected access token insert")
	}
	if pair.SessionID != "11111111-1111-1111-1111-111111111111" {
		t.Fatalf("unexpected session id: %q", pair.SessionID)
	}
	if pair.AccessToken == "" || pair.RefreshToken == "" {
		t.Fatalf("expected both access and refresh tokens")
	}
	if !pair.AccessExpiresAt.Equal(time.Date(2026, 4, 28, 10, 15, 0, 0, time.UTC)) {
		t.Fatalf("unexpected access expiration: %v", pair.AccessExpiresAt)
	}
	if !pair.RefreshExpiresAt.Equal(time.Date(2026, 5, 28, 10, 0, 0, 0, time.UTC)) {
		t.Fatalf("unexpected refresh expiration: %v", pair.RefreshExpiresAt)
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

func TestVerifyAccessToken_ChecksSessionState(t *testing.T) {
	defer restoreAuthHooks()()

	var gotSQL string

	authDBQueryRowFn = func(ctx context.Context, _ any, q string, args ...any) rowScanner {
		gotSQL = q
		return fakeRow{scanFn: func(dest ...any) error {
			return pgx.ErrNoRows
		}}
	}

	svc := NewService(nil, time.Hour)
	_, _, _, _, err := svc.VerifyAccessToken(context.Background(), "raw-token")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(gotSQL, "LEFT JOIN auth_sessions") {
		t.Fatalf("VerifyAccessToken must check auth_sessions state, got sql: %s", gotSQL)
	}
	if !strings.Contains(gotSQL, "s.revoked_at IS NULL") {
		t.Fatalf("VerifyAccessToken must reject revoked sessions, got sql: %s", gotSQL)
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

	_, code, err := svc.Login(context.Background(), "bad", "12345678", "", LoginOptions{})
	if err != nil || code != "invalid_email" {
		t.Fatalf("unexpected invalid email result: %q %v", code, err)
	}

	_, code, err = svc.Login(context.Background(), "user@example.com", "", "", LoginOptions{})
	if err != nil || code != "invalid_password" {
		t.Fatalf("unexpected invalid password result: %q %v", code, err)
	}

	authDBQueryRowFn = func(ctx context.Context, _ any, q string, args ...any) rowScanner {
		return fakeRow{scanFn: func(dest ...any) error { return pgx.ErrNoRows }}
	}
	_, code, err = svc.Login(context.Background(), "user@example.com", "12345678", "", LoginOptions{})
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
	_, code, err = svc.Login(context.Background(), "user@example.com", "wrong-pass", "", LoginOptions{})
	if err != nil || code != "invalid_credentials" {
		t.Fatalf("unexpected wrong password result: %q %v", code, err)
	}
}

func TestLogin_ActiveSessionExists(t *testing.T) {
	defer restoreAuthHooks()()

	hash, errHash := bcrypt.GenerateFromPassword([]byte("correct-pass"), 12)
	if errHash != nil {
		t.Fatalf("bcrypt error: %v", errHash)
	}

	authDBQueryRowFn = func(ctx context.Context, _ any, q string, args ...any) rowScanner {
		return fakeRow{scanFn: func(dest ...any) error {
			*(dest[0].(*string)) = "11111111-1111-1111-1111-111111111111"
			*(dest[1].(*string)) = "user@example.com"
			*(dest[2].(*string)) = "voter"
			*(dest[3].(*string)) = string(hash)
			return nil
		}}
	}

	authBeginTxFn = func(ctx context.Context, _ any) (txLike, error) {
		return &fakeTx{
			queryRowFn: func(ctx context.Context, sqlText string, args ...any) rowScanner {
				if strings.Contains(sqlText, "FROM users") && strings.Contains(sqlText, "FOR UPDATE") {
					return fakeRow{scanFn: func(dest ...any) error {
						*(dest[0].(*string)) = "11111111-1111-1111-1111-111111111111"
						return nil
					}}
				}

				if strings.Contains(sqlText, "FROM auth_sessions") {
					return fakeRow{scanFn: func(dest ...any) error {
						*(dest[0].(*string)) = "22222222-2222-2222-2222-222222222222"
						return nil
					}}
				}

				return fakeRow{scanFn: func(dest ...any) error {
					return errors.New("unexpected query")
				}}
			},
		}, nil
	}

	svc := NewServiceWithRefreshTTL(nil, 15*time.Minute, 30*24*time.Hour)
	res, code, err := svc.Login(
		context.Background(),
		"user@example.com",
		"correct-pass",
		"",
		LoginOptions{},
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if code != "active_session_exists" {
		t.Fatalf("unexpected code: %q", code)
	}
	if res != (AuthResult{}) {
		t.Fatalf("unexpected result: %+v", res)
	}
}

func TestLogin_ReplaceExistingSession(t *testing.T) {
	defer restoreAuthHooks()()

	nowFn = func() time.Time {
		return time.Date(2026, 4, 28, 10, 0, 0, 0, time.UTC)
	}
	randReadFn = func(b []byte) (int, error) {
		for i := range b {
			b[i] = byte((i % 16) + 1)
		}
		return len(b), nil
	}

	hash, errHash := bcrypt.GenerateFromPassword([]byte("correct-pass"), 12)
	if errHash != nil {
		t.Fatalf("bcrypt error: %v", errHash)
	}

	authDBQueryRowFn = func(ctx context.Context, _ any, q string, args ...any) rowScanner {
		return fakeRow{scanFn: func(dest ...any) error {
			*(dest[0].(*string)) = "11111111-1111-1111-1111-111111111111"
			*(dest[1].(*string)) = "user@example.com"
			*(dest[2].(*string)) = "voter"
			*(dest[3].(*string)) = string(hash)
			return nil
		}}
	}

	sessionRevoked := false
	oldTokensDeleted := false
	newAccessTokenInserted := false
	committed := false

	authBeginTxFn = func(ctx context.Context, _ any) (txLike, error) {
		return &fakeTx{
			queryRowFn: func(ctx context.Context, sqlText string, args ...any) rowScanner {
				if strings.Contains(sqlText, "FROM users") && strings.Contains(sqlText, "FOR UPDATE") {
					return fakeRow{scanFn: func(dest ...any) error {
						*(dest[0].(*string)) = "11111111-1111-1111-1111-111111111111"
						return nil
					}}
				}

				if strings.Contains(sqlText, "FROM auth_sessions") && strings.Contains(sqlText, "ORDER BY last_used_at") {
					return fakeRow{scanFn: func(dest ...any) error {
						*(dest[0].(*string)) = "22222222-2222-2222-2222-222222222222"
						return nil
					}}
				}

				if strings.Contains(sqlText, "INSERT INTO auth_sessions") {
					return fakeRow{scanFn: func(dest ...any) error {
						*(dest[0].(*string)) = "33333333-3333-3333-3333-333333333333"
						return nil
					}}
				}

				return fakeRow{scanFn: func(dest ...any) error {
					return errors.New("unexpected query")
				}}
			},
			execFn: func(ctx context.Context, sqlText string, args ...any) (pgconn.CommandTag, error) {
				switch {
				case strings.Contains(sqlText, "UPDATE auth_sessions") && strings.Contains(sqlText, "replaced_by_new_login"):
					sessionRevoked = true
					return pgconn.NewCommandTag("UPDATE 1"), nil

				case strings.Contains(sqlText, "DELETE FROM api_tokens"):
					oldTokensDeleted = true
					return pgconn.NewCommandTag("DELETE 1"), nil

				case strings.Contains(sqlText, "INSERT INTO api_tokens"):
					newAccessTokenInserted = true
					return pgconn.NewCommandTag("INSERT 0 1"), nil

				case strings.Contains(sqlText, "INSERT INTO audit_log"):
					return pgconn.NewCommandTag("INSERT 0 1"), nil

				default:
					return pgconn.CommandTag{}, errors.New("unexpected exec")
				}
			},
			commitFn: func(ctx context.Context) error {
				committed = true
				return nil
			},
		}, nil
	}

	svc := NewServiceWithRefreshTTL(nil, 15*time.Minute, 30*24*time.Hour)
	res, code, err := svc.Login(
		context.Background(),
		"user@example.com",
		"correct-pass",
		"",
		LoginOptions{
			ReplaceExistingSession: true,
			UserAgent:              "test-browser",
			IPAddress:              "192.0.2.10",
		},
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if code != "" {
		t.Fatalf("unexpected code: %q", code)
	}
	if res.AccessToken == "" || res.RefreshToken == "" {
		t.Fatalf("expected token pair, got %+v", res)
	}
	if !sessionRevoked {
		t.Fatal("expected old session revocation")
	}
	if !oldTokensDeleted {
		t.Fatal("expected old access tokens delete")
	}
	if !newAccessTokenInserted {
		t.Fatal("expected new access token insert")
	}
	if !committed {
		t.Fatal("expected commit")
	}
}

func TestLogout(t *testing.T) {
	defer restoreAuthHooks()()

	svc := NewService(nil, time.Hour)

	ok, err := svc.Logout(context.Background(), "", nil)
	if err != nil || ok {
		t.Fatalf("unexpected empty logout result: %v %v", ok, err)
	}

	t.Run("token not found", func(t *testing.T) {
		authBeginTxFn = func(ctx context.Context, _ any) (txLike, error) {
			return &fakeTx{
				queryRowFn: func(ctx context.Context, sqlText string, args ...any) rowScanner {
					return fakeRow{scanFn: func(dest ...any) error {
						return pgx.ErrNoRows
					}}
				},
			}, nil
		}

		ok, err := svc.Logout(context.Background(), "missing-token", nil)
		if err != nil || ok {
			t.Fatalf("unexpected missing logout result: %v %v", ok, err)
		}
	})

	t.Run("success revokes session", func(t *testing.T) {
		sessionRevoked := false
		committed := false

		authBeginTxFn = func(ctx context.Context, _ any) (txLike, error) {
			return &fakeTx{
				queryRowFn: func(ctx context.Context, sqlText string, args ...any) rowScanner {
					return fakeRow{scanFn: func(dest ...any) error {
						*(dest[0].(*sql.NullString)) = sql.NullString{
							String: "11111111-1111-1111-1111-111111111111",
							Valid:  true,
						}
						*(dest[1].(*string)) = "22222222-2222-2222-2222-222222222222"
						return nil
					}}
				},
				execFn: func(ctx context.Context, sqlText string, args ...any) (pgconn.CommandTag, error) {
					if strings.Contains(sqlText, "UPDATE auth_sessions") {
						sessionRevoked = true
						return pgconn.NewCommandTag("UPDATE 1"), nil
					}
					if strings.Contains(sqlText, "INSERT INTO audit_log") {
						return pgconn.NewCommandTag("INSERT 0 1"), nil
					}
					return pgconn.CommandTag{}, errors.New("unexpected exec")
				},
				commitFn: func(ctx context.Context) error {
					committed = true
					return nil
				},
			}, nil
		}

		ok, err := svc.Logout(context.Background(), "raw-token", nil)
		if err != nil || !ok {
			t.Fatalf("unexpected success logout result: %v %v", ok, err)
		}
		if !sessionRevoked {
			t.Fatal("expected session revocation")
		}
		if !committed {
			t.Fatal("expected commit")
		}
	})
}

func TestRefresh_EmptyToken(t *testing.T) {
	svc := NewService(nil, time.Hour)

	res, code, err := svc.Refresh(context.Background(), "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if code != "invalid_refresh_token" {
		t.Fatalf("unexpected code: %q", code)
	}
	if res != (AuthResult{}) {
		t.Fatalf("unexpected result: %+v", res)
	}
}

func TestChangePasswordValidation(t *testing.T) {
	svc := NewService(nil, time.Hour)

	code, err := svc.ChangePassword(context.Background(), "", "old-pass", "new-pass-123")
	if err != nil || code != "unauthorized" {
		t.Fatalf("unexpected unauthorized result: %q %v", code, err)
	}

	code, err = svc.ChangePassword(context.Background(), "u1", "", "new-pass-123")
	if err != nil || code != "invalid_current_password" {
		t.Fatalf("unexpected current password result: %q %v", code, err)
	}

	code, err = svc.ChangePassword(context.Background(), "u1", "old-pass", "123")
	if err != nil || code != "invalid_password" {
		t.Fatalf("unexpected invalid password result: %q %v", code, err)
	}
}

func TestChangePasswordSuccessAndErrors(t *testing.T) {
	defer restoreAuthHooks()()

	oldHash, err := bcrypt.GenerateFromPassword([]byte("old-pass-123"), 12)
	if err != nil {
		t.Fatalf("bcrypt old hash error: %v", err)
	}

	t.Run("user not found", func(t *testing.T) {
		authBeginTxFn = func(ctx context.Context, _ any) (txLike, error) {
			return &fakeTx{
				queryRowFn: func(ctx context.Context, sql string, args ...any) rowScanner {
					return fakeRow{scanFn: func(dest ...any) error { return pgx.ErrNoRows }}
				},
			}, nil
		}

		svc := NewService(nil, time.Hour)
		code, err := svc.ChangePassword(context.Background(), "u1", "old-pass-123", "new-pass-456")
		if err != nil || code != "unauthorized" {
			t.Fatalf("unexpected not found result: %q %v", code, err)
		}
	})

	t.Run("invalid current password", func(t *testing.T) {
		authBeginTxFn = func(ctx context.Context, _ any) (txLike, error) {
			return &fakeTx{
				queryRowFn: func(ctx context.Context, sql string, args ...any) rowScanner {
					return fakeRow{scanFn: func(dest ...any) error {
						*(dest[0].(*string)) = string(oldHash)
						return nil
					}}
				},
			}, nil
		}

		svc := NewService(nil, time.Hour)
		code, err := svc.ChangePassword(context.Background(), "u1", "wrong-pass", "new-pass-456")
		if err != nil || code != "invalid_current_password" {
			t.Fatalf("unexpected invalid current password result: %q %v", code, err)
		}
	})

	t.Run("password unchanged", func(t *testing.T) {
		authBeginTxFn = func(ctx context.Context, _ any) (txLike, error) {
			return &fakeTx{
				queryRowFn: func(ctx context.Context, sql string, args ...any) rowScanner {
					return fakeRow{scanFn: func(dest ...any) error {
						*(dest[0].(*string)) = string(oldHash)
						return nil
					}}
				},
			}, nil
		}

		svc := NewService(nil, time.Hour)
		code, err := svc.ChangePassword(context.Background(), "u1", "old-pass-123", "old-pass-123")
		if err != nil || code != "password_unchanged" {
			t.Fatalf("unexpected unchanged result: %q %v", code, err)
		}
	})

	t.Run("update error", func(t *testing.T) {
		authBeginTxFn = func(ctx context.Context, _ any) (txLike, error) {
			return &fakeTx{
				queryRowFn: func(ctx context.Context, sql string, args ...any) rowScanner {
					return fakeRow{scanFn: func(dest ...any) error {
						*(dest[0].(*string)) = string(oldHash)
						return nil
					}}
				},
				execFn: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
					if strings.Contains(sql, "UPDATE users") {
						return pgconn.CommandTag{}, errors.New("update boom")
					}
					return pgconn.NewCommandTag("INSERT 0 1"), nil
				},
			}, nil
		}

		svc := NewService(nil, time.Hour)
		code, err := svc.ChangePassword(context.Background(), "u1", "old-pass-123", "new-pass-456")
		if err == nil || code != "" || !strings.Contains(err.Error(), "update boom") {
			t.Fatalf("unexpected update error result: %q %v", code, err)
		}
	})

	t.Run("success", func(t *testing.T) {
		committed := false

		authBeginTxFn = func(ctx context.Context, _ any) (txLike, error) {
			return &fakeTx{
				queryRowFn: func(ctx context.Context, sql string, args ...any) rowScanner {
					return fakeRow{scanFn: func(dest ...any) error {
						*(dest[0].(*string)) = string(oldHash)
						return nil
					}}
				},
				execFn: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
					return pgconn.NewCommandTag("UPDATE 1"), nil
				},
				commitFn: func(ctx context.Context) error {
					committed = true
					return nil
				},
			}, nil
		}

		svc := NewService(nil, time.Hour)
		code, err := svc.ChangePassword(context.Background(), "u1", "old-pass-123", "new-pass-456")
		if err != nil || code != "" {
			t.Fatalf("unexpected success result: %q %v", code, err)
		}
		if !committed {
			t.Fatal("expected commit")
		}
	})
}

func TestValidateFullNameAndPhone(t *testing.T) {
	if !ValidateFullName("") {
		t.Fatal("empty full name should be allowed")
	}
	if !ValidateFullName("Иван Иванов") {
		t.Fatal("expected valid full name")
	}
	if ValidateFullName(strings.Repeat("a", 121)) {
		t.Fatal("expected invalid full name length")
	}

	if !ValidatePhone("") {
		t.Fatal("empty phone should be allowed")
	}
	if !ValidatePhone("+7 (999) 000-00-00") {
		t.Fatal("expected valid phone")
	}
	if ValidatePhone("abc") {
		t.Fatal("expected invalid phone")
	}
}

func TestGetProfile(t *testing.T) {
	defer restoreAuthHooks()()

	svc := NewService(nil, time.Hour)

	user, code, err := svc.GetProfile(context.Background(), "")
	if err != nil || code != "unauthorized" || user != (User{}) {
		t.Fatalf("unexpected empty user result: %+v %q %v", user, code, err)
	}

	authDBQueryRowFn = func(ctx context.Context, _ any, q string, args ...any) rowScanner {
		return fakeRow{scanFn: func(dest ...any) error { return pgx.ErrNoRows }}
	}
	_, code, err = svc.GetProfile(context.Background(), "u1")
	if err != nil || code != "unauthorized" {
		t.Fatalf("unexpected not found result: %q %v", code, err)
	}

	authDBQueryRowFn = func(ctx context.Context, _ any, q string, args ...any) rowScanner {
		return fakeRow{scanFn: func(dest ...any) error {
			*(dest[0].(*string)) = "u1"
			*(dest[1].(*string)) = "user@example.com"
			*(dest[2].(*string)) = "voter"
			*(dest[3].(**string)) = func() *string { s := "Иван Иванов"; return &s }()
			*(dest[4].(**string)) = func() *string { s := "+79990000000"; return &s }()
			return nil
		}}
	}
	user, code, err = svc.GetProfile(context.Background(), "u1")
	if err != nil || code != "" {
		t.Fatalf("unexpected get profile error: %q %v", code, err)
	}
	if user.FullName == nil || *user.FullName != "Иван Иванов" {
		t.Fatalf("unexpected full_name: %+v", user.FullName)
	}
	if user.Phone == nil || *user.Phone != "+79990000000" {
		t.Fatalf("unexpected phone: %+v", user.Phone)
	}
}

func TestUpdateProfile(t *testing.T) {
	defer restoreAuthHooks()()

	svc := NewService(nil, time.Hour)

	user, code, err := svc.UpdateProfile(context.Background(), "", "Иван", "+7999")
	if err != nil || code != "unauthorized" || user != (User{}) {
		t.Fatalf("unexpected unauthorized result: %+v %q %v", user, code, err)
	}

	_, code, err = svc.UpdateProfile(context.Background(), "u1", strings.Repeat("a", 121), "+7999")
	if err != nil || code != "invalid_full_name" {
		t.Fatalf("unexpected invalid_full_name result: %q %v", code, err)
	}

	_, code, err = svc.UpdateProfile(context.Background(), "u1", "Иван", "abc")
	if err != nil || code != "invalid_phone" {
		t.Fatalf("unexpected invalid_phone result: %q %v", code, err)
	}

	t.Run("not found", func(t *testing.T) {
		authBeginTxFn = func(ctx context.Context, _ any) (txLike, error) {
			return &fakeTx{
				queryRowFn: func(ctx context.Context, sql string, args ...any) rowScanner {
					return fakeRow{scanFn: func(dest ...any) error { return pgx.ErrNoRows }}
				},
			}, nil
		}

		_, code, err := svc.UpdateProfile(context.Background(), "u1", "Иван Иванов", "+79990000000")
		if err != nil || code != "unauthorized" {
			t.Fatalf("unexpected not found result: %q %v", code, err)
		}
	})

	t.Run("success", func(t *testing.T) {
		phase := 0
		committed := false

		authBeginTxFn = func(ctx context.Context, _ any) (txLike, error) {
			return &fakeTx{
				queryRowFn: func(ctx context.Context, sql string, args ...any) rowScanner {
					if phase == 0 {
						phase++
						return fakeRow{scanFn: func(dest ...any) error {
							*(dest[0].(*string)) = "user@example.com"
							*(dest[1].(*string)) = "voter"
							*(dest[2].(**string)) = func() *string { s := "Старое имя"; return &s }()
							*(dest[3].(**string)) = func() *string { s := "+70000000000"; return &s }()
							return nil
						}}
					}

					return fakeRow{scanFn: func(dest ...any) error {
						*(dest[0].(*string)) = "u1"
						*(dest[1].(*string)) = "user@example.com"
						*(dest[2].(*string)) = "voter"
						*(dest[3].(**string)) = func() *string { s := "Иван Иванов"; return &s }()
						*(dest[4].(**string)) = func() *string { s := "+79990000000"; return &s }()
						return nil
					}}
				},
				execFn: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
					return pgconn.NewCommandTag("UPDATE 1"), nil
				},
				commitFn: func(ctx context.Context) error {
					committed = true
					return nil
				},
			}, nil
		}

		user, code, err := svc.UpdateProfile(context.Background(), "u1", "Иван Иванов", "+79990000000")
		if err != nil || code != "" {
			t.Fatalf("unexpected success result: %+v %q %v", user, code, err)
		}
		if !committed {
			t.Fatal("expected commit")
		}
		if user.FullName == nil || *user.FullName != "Иван Иванов" {
			t.Fatalf("unexpected full_name: %+v", user.FullName)
		}
		if user.Phone == nil || *user.Phone != "+79990000000" {
			t.Fatalf("unexpected phone: %+v", user.Phone)
		}
	})
}
