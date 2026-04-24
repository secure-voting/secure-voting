package main

import (
	"context"
	"encoding/json"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	pb "secure-voting/apps/backend/internal/compute/pb"
)

func cleanStrings(in []string) []string {
	out := make([]string, 0, len(in))
	for _, v := range in {
		v = strings.TrimSpace(v)
		if v != "" {
			out = append(out, v)
		}
	}
	return out
}

func toScoreEntries(raw map[string]any) []*pb.ScoreEntry {
	if len(raw) == 0 {
		return nil
	}

	ids := make([]string, 0, len(raw))
	for id := range raw {
		id = strings.TrimSpace(id)
		if id != "" {
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)

	out := make([]*pb.ScoreEntry, 0, len(ids))
	for _, id := range ids {
		v := raw[id]
		var score int32

		switch t := v.(type) {
		case int:
			score = int32(t)
		case int32:
			score = t
		case int64:
			score = int32(t)
		case float64:
			score = int32(t)
		case float32:
			score = int32(t)
		default:
			continue
		}

		out = append(out, &pb.ScoreEntry{
			CandidateId: id,
			Value:       score,
		})
	}

	if len(out) == 0 {
		return nil
	}
	return out
}

func streamDatasetBallots(ctx context.Context, mdb *mongo.Database, cfg Config, datasetHex, ballotFormat string, stream pb.Compute_RunClient) (int, string, error) {
	switch normalizeBallotFormat(ballotFormat) {
	case "approval":
		return streamDatasetApprovalBallots(ctx, mdb, cfg, datasetHex, stream)
	case "ranking":
		return streamDatasetRankingBallots(ctx, mdb, cfg, datasetHex, stream)
	case "score":
		return streamDatasetScoreBallots(ctx, mdb, cfg, datasetHex, stream)
	default:
		return 0, "unsupported_ballot_format_for_compute", nil
	}
}

func streamElectionBallots(ctx context.Context, db *pgxpool.Pool, cfg Config, electionID, ballotFormat string, stream pb.Compute_RunClient) (int, string, error) {
	switch normalizeBallotFormat(ballotFormat) {
	case "approval":
		return streamElectionApprovalBallots(ctx, db, cfg, electionID, stream)
	case "ranking":
		return streamElectionRankingBallots(ctx, db, cfg, electionID, stream)
	case "score":
		return streamElectionScoreBallots(ctx, db, cfg, electionID, stream)
	default:
		return 0, "unsupported_ballot_format_for_compute", nil
	}
}

func streamDatasetApprovalBallots(ctx context.Context, mdb *mongo.Database, cfg Config, datasetHex string, stream pb.Compute_RunClient) (int, string, error) {
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
			SetProjection(bson.M{"voter_ref": 1, "approval": 1}),
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

		approval := cleanStrings(d.Approval)
		if len(approval) == 0 {
			continue
		}

		batch = append(batch, &pb.Ballot{
			VoterRef: d.VoterRef,
			Payload: &pb.Ballot_Approval{
				Approval: &pb.ApprovalBallot{Approvals: approval},
			},
		})
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

func streamDatasetRankingBallots(ctx context.Context, mdb *mongo.Database, cfg Config, datasetHex string, stream pb.Compute_RunClient) (int, string, error) {
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

		ranking := cleanStrings(d.Ranking)
		if len(ranking) == 0 {
			continue
		}

		batch = append(batch, &pb.Ballot{
			VoterRef: d.VoterRef,
			Payload: &pb.Ballot_Ranking{
				Ranking: &pb.RankingBallot{Ranking: ranking},
			},
		})
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

func streamDatasetScoreBallots(ctx context.Context, mdb *mongo.Database, cfg Config, datasetHex string, stream pb.Compute_RunClient) (int, string, error) {
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
			SetProjection(bson.M{"voter_ref": 1, "scores": 1}),
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

		entries := toScoreEntries(d.Scores)
		if len(entries) == 0 {
			continue
		}

		batch = append(batch, &pb.Ballot{
			VoterRef: d.VoterRef,
			Payload: &pb.Ballot_Score{
				Score: &pb.ScoreBallot{Scores: entries},
			},
		})
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

func streamElectionApprovalBallots(ctx context.Context, db *pgxpool.Pool, cfg Config, electionID string, stream pb.Compute_RunClient) (int, string, error) {
	rows, err := db.Query(ctx, `
		SELECT id::text, approval_set
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

		var approval []string
		if err := json.Unmarshal(raw, &approval); err != nil {
			return total, "bad_ballot_data", nil
		}

		clean := cleanStrings(approval)
		if len(clean) == 0 {
			continue
		}

		batch = append(batch, &pb.Ballot{
			VoterRef: ballotID,
			Payload: &pb.Ballot_Approval{
				Approval: &pb.ApprovalBallot{Approvals: clean},
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

		clean := cleanStrings(ranking)
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

func streamElectionScoreBallots(ctx context.Context, db *pgxpool.Pool, cfg Config, electionID string, stream pb.Compute_RunClient) (int, string, error) {
	rows, err := db.Query(ctx, `
		SELECT id::text, scores
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

		var scores map[string]any
		if err := json.Unmarshal(raw, &scores); err != nil {
			return total, "bad_ballot_data", nil
		}

		entries := toScoreEntries(scores)
		if len(entries) == 0 {
			continue
		}

		batch = append(batch, &pb.Ballot{
			VoterRef: ballotID,
			Payload: &pb.Ballot_Score{
				Score: &pb.ScoreBallot{Scores: entries},
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
