package main

import (
	"context"
	"strings"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	pb "secure-voting/apps/backend/internal/compute/pb"
)

func streamRankingBallots(ctx context.Context, mdb *mongo.Database, cfg Config, datasetHex string, stream pb.Compute_RunClient) (int, string, error) {
	oid, err := primitive.ObjectIDFromHex(strings.TrimSpace(datasetHex))
	if err != nil {
		return 0, "invalid_dataset_id", nil
	}

	coll := mdb.Collection("dataset_ballots")
	cur, err := coll.Find(
		ctx,
		bson.M{"dataset_id": oid},
		options.Find().
			SetBatchSize(int32(cfg.KafkaBallotBatchSize)).
			SetProjection(bson.M{"voter_ref": 1, "ranking": 1}),
	)
	if err != nil {
		return 0, "", err
	}
	defer func() { _ = cur.Close(ctx) }()

	batch := make([]*pb.Ballot, 0, cfg.KafkaBallotBatchSize)
	total := 0

	flush := func() error {
		if len(batch) == 0 {
			return nil
		}
		err := stream.Send(&pb.RunChunk{
			Part: &pb.RunChunk_Batch{
				Batch: &pb.BallotBatch{Ballots: batch},
			},
		})
		batch = batch[:0]
		return err
	}

	for cur.Next(ctx) {
		var d ballotDoc
		if err := cur.Decode(&d); err != nil {
			return total, "", err
		}
		if len(d.Ranking) == 0 {
			continue
		}

		b := &pb.Ballot{
			VoterRef: d.VoterRef,
			Payload: &pb.Ballot_Ranking{
				Ranking: &pb.RankingBallot{Ranking: d.Ranking},
			},
		}
		batch = append(batch, b)
		total++

		if len(batch) >= cfg.KafkaBallotBatchSize {
			if err := flush(); err != nil {
				return total, "", err
			}
		}
	}

	if err := cur.Err(); err != nil {
		return total, "", err
	}

	if err := flush(); err != nil {
		return total, "", err
	}

	if total == 0 {
		return 0, "empty_dataset_ballots", nil
	}

	return total, "", nil
}
