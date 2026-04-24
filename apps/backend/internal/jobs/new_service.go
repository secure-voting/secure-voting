package jobs

import "github.com/jackc/pgx/v5/pgxpool"

func NewService(db *pgxpool.Pool) *Service {
	return &Service{db: db}
}
