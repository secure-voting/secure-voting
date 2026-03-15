package datasets

import "go.mongodb.org/mongo-driver/mongo"

type Service struct {
	db *mongo.Database
}

func NewService(db *mongo.Database) *Service {
	return &Service{db: db}
}
