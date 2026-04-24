package datasets

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func (s *Service) List(ctx context.Context) ([]ListItem, error) {
	cur, err := datasetFindFn(
		ctx,
		s.db,
		"datasets",
		bson.M{},
		options.Find().SetSort(bson.D{{Key: "created_at", Value: -1}}),
	)
	if err != nil {
		return nil, err
	}
	defer func() { _ = cur.Close(ctx) }()

	out := make([]ListItem, 0)
	for cur.Next(ctx) {
		var d DatasetDoc
		if err := cur.Decode(&d); err != nil {
			return nil, err
		}
		out = append(out, ListItem{
			ID:        d.ID.Hex(),
			Name:      d.Name,
			Source:    d.Source,
			Format:    d.Format,
			CreatedAt: d.CreatedAt.UTC().Format(time.RFC3339),
		})
	}

	if err := cur.Err(); err != nil {
		return nil, err
	}

	return out, nil
}
