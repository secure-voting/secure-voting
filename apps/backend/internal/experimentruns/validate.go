package experimentruns

import (
	"context"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

func (s *Service) validateDatasetsExist(ctx context.Context, oids []primitive.ObjectID) (bool, error) {
	if len(oids) == 0 {
		return false, nil
	}
	cnt, err := countDatasetsFn(ctx, s.mongodb, oids)
	if err != nil {
		return false, err
	}
	return int(cnt) == len(oids), nil
}
