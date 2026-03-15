package jobs

import "github.com/jackc/pgx/v5/pgxpool"

func NewRunner(db *pgxpool.Pool) *Runner {
	return &Runner{db: db}
}
