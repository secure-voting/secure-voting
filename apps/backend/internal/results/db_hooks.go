package results

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

type rowScanner interface {
	Scan(dest ...any) error
}

var resultQueryRowFn = func(ctx context.Context, db *pgxpool.Pool, q string, args ...any) rowScanner {
	return db.QueryRow(ctx, q, args...)
}
