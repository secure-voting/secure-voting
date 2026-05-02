package main

import (
	"context"
	"errors"
	"strings"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"

	"secure-voting/apps/backend/internal/worker"
)

type datasetMetaDoc struct {
	Candidates []struct {
		ID   string `bson:"id"`
		Name string `bson:"name"`
	} `bson:"candidates"`
}

func hasUsableDatasetCandidates(candidates []worker.DatasetCandidate) bool {
	for _, candidate := range candidates {
		if strings.TrimSpace(candidate.ID) != "" || strings.TrimSpace(candidate.Name) != "" {
			return true
		}
	}
	return false
}

func loadDatasetCandidates(ctx context.Context, mdb *mongo.Database, datasetHex string) ([]worker.DatasetCandidate, string, error) {
	oid, err := primitive.ObjectIDFromHex(strings.TrimSpace(datasetHex))
	if err != nil {
		return nil, "invalid_dataset_id", nil
	}

	var doc datasetMetaDoc
	err = mdb.Collection("datasets").FindOne(
		ctx,
		bson.M{"_id": oid},
	).Decode(&doc)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, "dataset_not_found", nil
		}
		return nil, "", err
	}

	out := make([]worker.DatasetCandidate, 0, len(doc.Candidates))
	for _, candidate := range doc.Candidates {
		id := strings.TrimSpace(candidate.ID)
		name := strings.TrimSpace(candidate.Name)

		if id == "" && name == "" {
			continue
		}

		out = append(out, worker.DatasetCandidate{
			ID:   id,
			Name: name,
		})
	}

	if len(out) == 0 {
		return nil, "dataset_has_no_candidates", nil
	}

	return out, "", nil
}
