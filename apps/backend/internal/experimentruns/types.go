package experimentruns

import (
	"github.com/jackc/pgx/v5/pgxpool"
	"go.mongodb.org/mongo-driver/mongo"
)

type Service struct {
	db      *pgxpool.Pool
	mongodb *mongo.Database
}
