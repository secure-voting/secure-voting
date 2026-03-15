package datasets

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

func (s *Service) Download(ctx context.Context, id string) ([]byte, string, string, string, error) {
	oid, err := primitive.ObjectIDFromHex(strings.TrimSpace(id))
	if err != nil {
		return nil, "", "", "invalid_id", nil
	}

	var d DatasetDoc
	err = s.db.Collection("datasets").FindOne(ctx, bson.M{"_id": oid}).Decode(&d)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, "", "", "not_found", nil
		}
		return nil, "", "", "", err
	}

	if len(d.Raw.Data) == 0 {
		meta := map[string]any{
			"id":          d.ID.Hex(),
			"name":        d.Name,
			"description": d.Description,
			"source":      d.Source,
			"format":      d.Format,
			"candidates":  d.Candidates,
			"created_at":  d.CreatedAt.UTC().Format(time.RFC3339),
			"seed":        d.Seed,
		}
		b, _ := json.Marshal(meta)
		return b, "dataset.json", "application/json", "", nil
	}

	fn := d.RawFilename
	if fn == "" {
		fn = "dataset.bin"
	}
	ct := d.RawMime
	if ct == "" {
		ct = "application/octet-stream"
	}
	return d.Raw.Data, fn, ct, "", nil
}
