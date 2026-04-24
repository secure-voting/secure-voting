package jobs

import (
	"context"

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

var getJobQueryRowFn = func(ctx context.Context, db *pgxpool.Pool, q string, args ...any) rowScanner {
	return db.QueryRow(ctx, q, args...)
}

var listJobsQueryFn = func(ctx context.Context, db *pgxpool.Pool, q string, args ...any) (rowsScanner, error) {
	return db.Query(ctx, q, args...)
}

var claimNextJobQueryRowFn = func(ctx context.Context, db *pgxpool.Pool, q string, args ...any) rowScanner {
	return db.QueryRow(ctx, q, args...)
}

var runnerExecFn = func(ctx context.Context, db *pgxpool.Pool, q string, args ...any) error {
	_, err := db.Exec(ctx, q, args...)
	return err
}
