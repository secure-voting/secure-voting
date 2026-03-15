package experimentruns

import (
	"context"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func (s *Service) validateDatasetsExist(ctx context.Context, oids []primitive.ObjectID) (bool, error) {
	if len(oids) == 0 {
		return false, nil
	}
	coll := s.mongodb.Collection("datasets")

	filter := bson.M{"_id": bson.M{"$in": oids}}
	cnt, err := coll.CountDocuments(ctx, filter)
	if err != nil {
		return false, err
	}
	return int(cnt) == len(oids), nil
}
