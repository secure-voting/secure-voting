package datasets

import (
	"github.com/jackc/pgx/v5/pgxpool"
	"go.mongodb.org/mongo-driver/mongo"
)

type Service struct {
	db *mongo.Database
	pg *pgxpool.Pool
}

func NewService(db *mongo.Database, pg ...*pgxpool.Pool) *Service {
	var pool *pgxpool.Pool
	if len(pg) > 0 {
		pool = pg[0]
	}

	return &Service{
		db: db,
		pg: pool,
	}
}
