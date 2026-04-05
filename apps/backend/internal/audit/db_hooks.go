package audit

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

type rowsScanner interface {
	Next() bool
	Scan(dest ...any) error
	Close()
	Err() error
}

var auditQueryFn = func(ctx context.Context, db any, q string, args ...any) (rowsScanner, error) {
	return db.(*pgxpool.Pool).Query(ctx, q, args...)
}
