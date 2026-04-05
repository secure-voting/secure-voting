package main

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
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

func streamElectionRankingBallots(ctx context.Context, db *pgxpool.Pool, cfg Config, electionID string, stream pb.Compute_RunClient) (int, string, error) {
	rows, err := db.Query(ctx, `
		SELECT id::text, ranking
		FROM ballots
		WHERE election_id = $1::uuid AND status = 'accepted'
	`, strings.TrimSpace(electionID))
	if err != nil {
		return 0, "", err
	}
	defer rows.Close()

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

	for rows.Next() {
		var ballotID string
		var raw []byte

		if err := rows.Scan(&ballotID, &raw); err != nil {
			return total, "", err
		}

		if len(raw) == 0 || string(raw) == "null" {
			continue
		}

		var ranking []string
		if err := json.Unmarshal(raw, &ranking); err != nil {
			return total, "bad_ballot_data", nil
		}

		clean := make([]string, 0, len(ranking))
		for _, v := range ranking {
			v = strings.TrimSpace(v)
			if v != "" {
				clean = append(clean, v)
			}
		}
		if len(clean) == 0 {
			continue
		}

		batch = append(batch, &pb.Ballot{
			VoterRef: ballotID,
			Payload: &pb.Ballot_Ranking{
				Ranking: &pb.RankingBallot{Ranking: clean},
			},
		})
		total++

		if len(batch) >= cfg.KafkaBallotBatchSize {
			if err := flush(); err != nil {
				return total, "", err
			}
		}
	}

	if err := rows.Err(); err != nil {
		return total, "", err
	}

	if err := flush(); err != nil {
		return total, "", err
	}

	if total == 0 {
		return 0, "empty_election_ballots", nil
	}

	return total, "", nil
}
