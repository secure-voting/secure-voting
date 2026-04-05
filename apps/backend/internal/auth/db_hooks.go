package auth

import (
	"context"
	"crypto/rand"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type rowScanner = pgx.Row

type txLike interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	QueryRow(ctx context.Context, sql string, args ...any) rowScanner
	Commit(ctx context.Context) error
	Rollback(ctx context.Context) error
}

var authBeginTxFn = func(ctx context.Context, db any) (txLike, error) {
	return db.(*pgxpool.Pool).Begin(ctx)
}

var authDBQueryRowFn = func(ctx context.Context, db any, q string, args ...any) rowScanner {
	return db.(*pgxpool.Pool).QueryRow(ctx, q, args...)
}

var authDBExecFn = func(ctx context.Context, db any, q string, args ...any) (pgconn.CommandTag, error) {
	return db.(*pgxpool.Pool).Exec(ctx, q, args...)
}

var randReadFn = rand.Read
var nowFn = time.Now
