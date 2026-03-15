package experimentruns

import (
	"context"
	"errors"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

func (s *Service) GetResult(ctx context.Context, role, userID, runID string) (Result, string, error) {
	_, code, err := s.Get(ctx, role, userID, runID)
	if err != nil {
		return Result{}, "", err
	}
	if code != "" {
		return Result{}, code, nil
	}

	var res Result
	err = s.mongodb.Collection("experiment_results").FindOne(ctx, bson.M{"run_id": runID}).Decode(&res)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return Result{}, "not_found", nil
		}
		return Result{}, "", err
	}
	return res, "", nil
}
