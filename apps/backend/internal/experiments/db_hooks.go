package experiments

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

var createExperimentQueryRowFn = func(ctx context.Context, db *pgxpool.Pool, q string, args ...any) rowScanner {
	return db.QueryRow(ctx, q, args...)
}

var getExperimentQueryRowFn = func(ctx context.Context, db *pgxpool.Pool, q string, args ...any) rowScanner {
	return db.QueryRow(ctx, q, args...)
}

var listExperimentsQueryFn = func(ctx context.Context, db *pgxpool.Pool, q string, args ...any) (rowsScanner, error) {
	return db.Query(ctx, q, args...)
}

var insertAuditFn = insertAudit
