package ballots

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type rowScanner interface {
	Scan(dest ...any) error
}

type rowsScanner interface {
	Next() bool
	Scan(dest ...any) error
	Close()
	Err() error
}

type txLike interface {
	QueryRow(ctx context.Context, sql string, args ...any) rowScanner
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Commit(ctx context.Context) error
	Rollback(ctx context.Context) error
}

type pgxTxWrapper struct {
	tx pgx.Tx
}

func (w pgxTxWrapper) QueryRow(ctx context.Context, sql string, args ...any) rowScanner {
	return w.tx.QueryRow(ctx, sql, args...)
}

func (w pgxTxWrapper) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	return w.tx.Exec(ctx, sql, args...)
}

func (w pgxTxWrapper) Commit(ctx context.Context) error {
	return w.tx.Commit(ctx)
}

func (w pgxTxWrapper) Rollback(ctx context.Context) error {
	return w.tx.Rollback(ctx)
}

var ballotsQueryRowFn = func(ctx context.Context, db *pgxpool.Pool, q string, args ...any) rowScanner {
	return db.QueryRow(ctx, q, args...)
}

var ballotsQueryFn = func(ctx context.Context, db *pgxpool.Pool, q string, args ...any) (rowsScanner, error) {
	return db.Query(ctx, q, args...)
}

var ballotsBeginTxFn = func(ctx context.Context, db *pgxpool.Pool) (txLike, error) {
	tx, err := db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, err
	}
	return pgxTxWrapper{tx: tx}, nil
}
