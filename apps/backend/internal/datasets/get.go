package datasets

import (
	"context"
	"errors"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

func (s *Service) Get(ctx context.Context, id string) (Dataset, string, error) {
	oid, err := primitive.ObjectIDFromHex(strings.TrimSpace(id))
	if err != nil {
		return Dataset{}, "invalid_id", nil
	}

	var d DatasetDoc
	err = s.db.Collection("datasets").FindOne(ctx, bson.M{"_id": oid}).Decode(&d)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return Dataset{}, "not_found", nil
		}
		return Dataset{}, "", err
	}

	return Dataset{
		ID:          d.ID.Hex(),
		Name:        d.Name,
		Description: d.Description,
		Source:      d.Source,
		Format:      d.Format,
		Candidates:  d.Candidates,
		CreatedAt:   d.CreatedAt.UTC().Format(time.RFC3339),
		Seed:        d.Seed,
		Parameters:  d.Parameters,
	}, "", nil
}
