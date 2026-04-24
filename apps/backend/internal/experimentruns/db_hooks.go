package experimentruns

import (
	"context"
	"errors"

	"secure-voting/apps/backend/internal/auditlog"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

type rowScanner interface {
	Scan(dest ...any) error
}

type rowsScanner interface {
	Next() bool
	Scan(dest ...any) error
	Close()
	Err() error
}

type singleResultDecoder interface {
	Decode(v any) error
}

type batchTx interface {
	Commit(ctx context.Context) error
	Rollback(ctx context.Context) error
}

var batchCheckExperimentOwnerFn = func(ctx context.Context, db *pgxpool.Pool, expID, createdBy string) (bool, error) {
	var x int
	err := db.QueryRow(ctx, `
		SELECT 1 FROM experiments
		WHERE id=$1::uuid AND created_by=$2::uuid
	`, expID, createdBy).Scan(&x)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

var batchValidateDatasetsExistFn = func(s *Service, ctx context.Context, oids []primitive.ObjectID) (bool, error) {
	return s.validateDatasetsExist(ctx, oids)
}

var batchBeginTxFn = func(ctx context.Context, db *pgxpool.Pool) (batchTx, error) {
	return db.BeginTx(ctx, pgx.TxOptions{})
}

var batchInsertRunFn = func(ctx context.Context, tx batchTx, expID, dsid string) (string, error) {
	pgxTx, ok := tx.(pgx.Tx)
	if !ok {
		return "", errors.New("unexpected tx type")
	}

	var runID string
	err := pgxTx.QueryRow(ctx, `
		INSERT INTO experiment_runs (experiment_id, dataset_id, status)
		VALUES ($1::uuid, $2, 'queued')
		RETURNING id::text
	`, expID, dsid).Scan(&runID)
	return runID, err
}

var batchInsertJobFn = func(ctx context.Context, tx batchTx, createdBy, expID, runID, payload string) (string, error) {
	pgxTx, ok := tx.(pgx.Tx)
	if !ok {
		return "", errors.New("unexpected tx type")
	}

	var jobID string
	err := pgxTx.QueryRow(ctx, `
		INSERT INTO jobs (kind, status, progress, created_by, experiment_id, experiment_run_id, payload)
		VALUES ('experiment_run', 'queued', 0, $1::uuid, $2::uuid, $3::uuid, $4::jsonb)
		RETURNING id::text
	`, createdBy, expID, runID, payload).Scan(&jobID)
	return jobID, err
}

var batchAuditInsertFn = func(ctx context.Context, tx batchTx, createdBy, expID string, count int) error {
	pgxTx, ok := tx.(pgx.Tx)
	if !ok {
		return errors.New("unexpected tx type")
	}
	return auditlog.Insert(ctx, pgxTx, &createdBy, "experiment_runs_batch_created", map[string]any{
		"target_type": "experiment",
		"target_id":   expID,
		"after": map[string]any{
			"count": count,
		},
	})
}

var getRunQueryRowFn = func(ctx context.Context, db *pgxpool.Pool, q string, args ...any) rowScanner {
	return db.QueryRow(ctx, q, args...)
}

var listRunsQueryFn = func(ctx context.Context, db *pgxpool.Pool, q string, args ...any) (rowsScanner, error) {
	return db.Query(ctx, q, args...)
}

var getRunAccessFn = func(s *Service, ctx context.Context, role, userID, runID string) (Run, string, error) {
	return s.Get(ctx, role, userID, runID)
}

var findExperimentResultFn = func(ctx context.Context, mdb *mongo.Database, runID string) singleResultDecoder {
	return mdb.Collection("experiment_results").FindOne(ctx, bson.M{"run_id": runID})
}

var countDatasetsFn = func(ctx context.Context, mdb *mongo.Database, oids []primitive.ObjectID) (int64, error) {
	coll := mdb.Collection("datasets")
	filter := bson.M{"_id": bson.M{"$in": oids}}
	return coll.CountDocuments(ctx, filter)
}

var downloadResultGetFn = func(s *Service, ctx context.Context, role, userID, runID string) (Result, string, error) {
	return s.GetResult(ctx, role, userID, runID)
}
