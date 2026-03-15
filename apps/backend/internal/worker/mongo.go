package worker

import (
	"context"
	"errors"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type datasetDoc struct {
	ID          primitive.ObjectID `bson:"_id,omitempty"`
	Name        string             `bson:"name"`
	Description string             `bson:"description,omitempty"`
	Source      string             `bson:"source"`
	Format      string             `bson:"format"`
	Candidates  []struct {
		ID   string `bson:"id"`
		Name string `bson:"name"`
	} `bson:"candidates"`
	CreatedAt  time.Time      `bson:"created_at"`
	Seed       *int64         `bson:"seed,omitempty"`
	Parameters map[string]any `bson:"parameters,omitempty"`
}

func (w *Worker) loadDatasetMeta(ctx context.Context, datasetHex string) (DatasetInfo, string, error) {
	oid, err := primitive.ObjectIDFromHex(strings.TrimSpace(datasetHex))
	if err != nil {
		return DatasetInfo{}, "invalid_dataset_id", nil
	}

	var ds datasetDoc
	err = w.mdb.Collection("datasets").FindOne(ctx, bson.M{"_id": oid}).Decode(&ds)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return DatasetInfo{}, "not_found", nil
		}
		return DatasetInfo{}, "", err
	}

	cands := make([]DatasetCandidate, 0, len(ds.Candidates))
	for _, c := range ds.Candidates {
		cands = append(cands, DatasetCandidate{ID: c.ID, Name: c.Name})
	}

	out := DatasetInfo{
		ID:          ds.ID.Hex(),
		Name:        ds.Name,
		Description: ds.Description,
		Source:      ds.Source,
		Format:      ds.Format,
		Candidates:  cands,
		Seed:        ds.Seed,
		Parameters:  ds.Parameters,
		CreatedAt:   ds.CreatedAt.UTC().Format(time.RFC3339),
	}

	return out, "", nil
}

func (w *Worker) upsertExperimentResult(ctx context.Context, res ExperimentRunResult) (string, error) {
	coll := w.mdb.Collection("experiment_results")

	update := bson.M{
		"$set": bson.M{
			"run_id":     res.RunID,
			"winners":    res.Winners,
			"metrics":    res.Metrics,
			"timings":    res.Timings,
			"artifacts":  res.Artifacts,
			"updated_at": time.Now().UTC(),
		},
		"$setOnInsert": bson.M{
			"created_at": time.Now().UTC(),
		},
	}

	_, err := coll.UpdateOne(ctx, bson.M{"run_id": res.RunID}, update, options.Update().SetUpsert(true))
	if err != nil {
		return "", err
	}

	var doc struct {
		ID primitive.ObjectID `bson:"_id"`
	}
	err = coll.FindOne(ctx, bson.M{"run_id": res.RunID}).Decode(&doc)
	if err != nil {
		return "", err
	}

	return doc.ID.Hex(), nil
}
