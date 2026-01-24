package worker

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/segmentio/kafka-go"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"secure-voting/apps/backend/internal/jobs"
	"secure-voting/apps/backend/internal/tally"
)

type Worker struct {
	db     *pgxpool.Pool
	mdb    *mongo.Database
	runner *jobs.Runner

	kw *kafka.Writer
	kr *kafka.Reader

	pollInterval time.Duration
}

type Config struct {
	PollInterval time.Duration

	TasksTopic   string
	ResultsTopic string
	GroupID      string
	Brokers      []string
}

func New(db *pgxpool.Pool, mdb *mongo.Database, cfg Config) *Worker {
	kw := &kafka.Writer{
		Addr:         kafka.TCP(cfg.Brokers...),
		Topic:        cfg.TasksTopic,
		RequiredAcks: kafka.RequireAll,
		Balancer:     &kafka.LeastBytes{},
		BatchTimeout: 50 * time.Millisecond,
	}

	kr := kafka.NewReader(kafka.ReaderConfig{
		Brokers:     cfg.Brokers,
		GroupID:     cfg.GroupID,
		Topic:       cfg.ResultsTopic,
		MinBytes:    1e3,
		MaxBytes:    10e6,
		MaxWait:     250 * time.Millisecond,
		StartOffset: kafka.FirstOffset,
	})

	pi := cfg.PollInterval
	if pi <= 0 {
		pi = 1 * time.Second
	}

	return &Worker{
		db:           db,
		mdb:          mdb,
		runner:       jobs.NewRunner(db),
		kw:           kw,
		kr:           kr,
		pollInterval: pi,
	}
}

func (w *Worker) Close() {
	if w.kw != nil {
		_ = w.kw.Close()
	}
	if w.kr != nil {
		_ = w.kr.Close()
	}
}

func (w *Worker) Run(ctx context.Context) error {
	errCh := make(chan error, 1)

	go func() {
		errCh <- w.consumeResults(ctx)
	}()

	ticker := time.NewTicker(w.pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case err := <-errCh:
			return err
		case <-ticker.C:
			if err := w.tick(ctx); err != nil {
				log.Printf("worker.tick error: %v", err)
			}
		}
	}
}

func (w *Worker) tick(ctx context.Context) error {
	job, ok, err := w.runner.ClaimNext(ctx, []string{"tally", "experiment_run"})
	if err != nil {
		return err
	}
	if !ok {
		return nil
	}

	_ = w.runner.UpdateProgress(ctx, job.ID, 5)

	switch job.Kind {
	case "experiment_run":
		return w.handleExperimentRun(ctx, job)
	case "tally":
		return w.handleTallyLocal(ctx, job)
	default:
		_ = w.runner.MarkError(ctx, job.ID, "unsupported job kind: "+job.Kind)
		return nil
	}
}

type expRunPayload struct {
	ExperimentID string `json:"experiment_id"`
	DatasetID    string `json:"dataset_id"`
	RunID        string `json:"run_id"`
}

func (w *Worker) handleExperimentRun(ctx context.Context, job jobs.ClaimedJob) error {
	if job.ExperimentRunID == nil || strings.TrimSpace(*job.ExperimentRunID) == "" {
		_ = w.runner.MarkError(ctx, job.ID, "missing experiment_run_id in jobs row")
		return nil
	}

	var pl expRunPayload
	if len(job.Payload) == 0 || string(job.Payload) == "null" {
		_ = w.runner.MarkError(ctx, job.ID, "missing payload")
		return nil
	}
	if err := json.Unmarshal(job.Payload, &pl); err != nil {
		_ = w.runner.MarkError(ctx, job.ID, "invalid payload json")
		return nil
	}

	pl.RunID = strings.TrimSpace(pl.RunID)
	pl.ExperimentID = strings.TrimSpace(pl.ExperimentID)
	pl.DatasetID = strings.TrimSpace(pl.DatasetID)

	if _, err := uuid.Parse(pl.RunID); err != nil {
		_ = w.runner.MarkError(ctx, job.ID, "invalid run_id in payload")
		return nil
	}
	if _, err := uuid.Parse(pl.ExperimentID); err != nil {
		_ = w.runner.MarkError(ctx, job.ID, "invalid experiment_id in payload")
		return nil
	}
	if pl.RunID != *job.ExperimentRunID {
		_ = w.runner.MarkError(ctx, job.ID, "payload run_id mismatch with jobs.experiment_run_id")
		return nil
	}

	expType, expSeed, expParams, code, err := w.loadExperiment(ctx, pl.ExperimentID)
	if err != nil {
		return err
	}
	if code != "" {
		_ = w.failRunAndJob(ctx, pl.RunID, job.ID, "experiment not found")
		return nil
	}

	_ = w.runner.UpdateProgress(ctx, job.ID, 15)

	ds, code, err := w.loadDatasetMeta(ctx, pl.DatasetID)
	if err != nil {
		return err
	}
	if code != "" {
		_ = w.failRunAndJob(ctx, pl.RunID, job.ID, "dataset not found")
		return nil
	}

	_ = w.runner.UpdateProgress(ctx, job.ID, 30)

	if err := w.markRunRunning(ctx, pl.RunID, job.ID); err != nil {
		return err
	}

	task := ExperimentRunTask{
		Kind:            "experiment_run",
		JobID:           job.ID,
		RunID:           pl.RunID,
		ExperimentID:    pl.ExperimentID,
		DatasetID:       pl.DatasetID,
		ExperimentType:  expType,
		ExperimentSeed:  expSeed,
		ExperimentParams: expParams,
		Dataset:         ds,
	}

	value, err := json.Marshal(task)
	if err != nil {
		_ = w.failRunAndJob(ctx, pl.RunID, job.ID, "failed to marshal task")
		return nil
	}

	err = w.kw.WriteMessages(ctx, kafka.Message{
		Key:   []byte(pl.RunID),
		Value: value,
		Time:  time.Now().UTC(),
	})
	if err != nil {
		_ = w.failRunAndJob(ctx, pl.RunID, job.ID, "kafka publish failed: "+err.Error())
		return nil
	}

	_ = w.runner.UpdateProgress(ctx, job.ID, 45)
	return nil
}

func (w *Worker) consumeResults(ctx context.Context) error {
	for {
		msg, err := w.kr.ReadMessage(ctx)
		if err != nil {
			return err
		}

		var res ExperimentRunResult
		if err := json.Unmarshal(msg.Value, &res); err != nil {
			log.Printf("worker.consumeResults: bad json: %v", err)
			continue
		}

		res.RunID = strings.TrimSpace(res.RunID)
		if res.RunID == "" {
			log.Printf("worker.consumeResults: missing run_id")
			continue
		}
		if _, err := uuid.Parse(res.RunID); err != nil {
			log.Printf("worker.consumeResults: invalid run_id: %q", res.RunID)
			continue
		}

		res.Status = strings.TrimSpace(res.Status)
		if res.Status != "done" && res.Status != "error" {
			log.Printf("worker.consumeResults: invalid status: %q", res.Status)
			continue
		}

		if err := w.applyExperimentRunResult(ctx, res); err != nil {
			log.Printf("worker.applyExperimentRunResult error: %v", err)
			continue
		}
	}
}

func (w *Worker) applyExperimentRunResult(ctx context.Context, res ExperimentRunResult) error {
	// 1) upsert в Mongo по run_id (идемпотентно)
	oidHex, err := w.upsertExperimentResult(ctx, res)
	if err != nil {
		return err
	}

	// 2) транзакционно обновляем Postgres
	tx, err := w.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	now := time.Now().UTC()

	if res.Status == "done" {
		_, err = tx.Exec(ctx, `
UPDATE experiment_runs
SET status='done', finished_at=$2
WHERE id=$1::uuid
`, res.RunID, now)
		if err != nil {
			return err
		}

		ref := map[string]any{
			"mongo_experiment_result_id": oidHex,
			"run_id": res.RunID,
		}
		refJSON, _ := json.Marshal(ref)

		_, err = tx.Exec(ctx, `
UPDATE jobs
SET status='done', progress=100, finished_at=$2, error_text=NULL, result_ref=$3::jsonb
WHERE kind='experiment_run'
  AND experiment_run_id=$1::uuid
  AND status IN ('queued','running')
`, res.RunID, now, string(refJSON))
		if err != nil {
			return err
		}
	} else {
		errText := strings.TrimSpace(res.ErrorText)
		if errText == "" {
			errText = "experiment_run failed"
		}

		_, err = tx.Exec(ctx, `
UPDATE experiment_runs
SET status='error', finished_at=$2
WHERE id=$1::uuid
`, res.RunID, now)
		if err != nil {
			return err
		}

		_, err = tx.Exec(ctx, `
UPDATE jobs
SET status='error', finished_at=$2, error_text=$3
WHERE kind='experiment_run'
  AND experiment_run_id=$1::uuid
  AND status IN ('queued','running')
`, res.RunID, now, errText)
		if err != nil {
			return err
		}
	}

	return tx.Commit(ctx)
}

func (w *Worker) upsertExperimentResult(ctx context.Context, res ExperimentRunResult) (string, error) {
	coll := w.mdb.Collection("experiment_results")

	update := bson.M{
		"$set": bson.M{
			"run_id":    res.RunID,
			"winners":   res.Winners,
			"metrics":   res.Metrics,
			"timings":   res.Timings,
			"artifacts": res.Artifacts,
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

	// узнаем _id
	var doc struct {
		ID primitive.ObjectID `bson:"_id"`
	}
	err = coll.FindOne(ctx, bson.M{"run_id": res.RunID}).Decode(&doc)
	if err != nil {
		return "", err
	}
	return doc.ID.Hex(), nil
}

func (w *Worker) loadExperiment(ctx context.Context, experimentID string) (expType string, expSeed *int64, params json.RawMessage, code string, err error) {
	var t string
	var p []byte
	var seed *int64

	err = w.db.QueryRow(ctx, `
SELECT type, COALESCE(params,'{}'::jsonb), seed
FROM experiments
WHERE id=$1::uuid
`, experimentID).Scan(&t, &p, &seed)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", nil, nil, "not_found", nil
		}
		return "", nil, nil, "", err
	}

	return t, seed, p, "", nil
}

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

func (w *Worker) markRunRunning(ctx context.Context, runID string, kernelTaskID string) error {
	_, err := w.db.Exec(ctx, `
UPDATE experiment_runs
SET status='running',
    started_at=COALESCE(started_at, now()),
    kernel_task_id=$2
WHERE id=$1::uuid
`, runID, kernelTaskID)
	return err
}

func (w *Worker) failRunAndJob(ctx context.Context, runID, jobID, errText string) error {
	errText = strings.TrimSpace(errText)
	if errText == "" {
		errText = "job failed"
	}

	tx, err := w.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	_, err = tx.Exec(ctx, `
UPDATE experiment_runs
SET status='error', finished_at=now()
WHERE id=$1::uuid
`, runID)
	if err != nil {
		return err
	}

	_, err = tx.Exec(ctx, `
UPDATE jobs
SET status='error', finished_at=now(), error_text=$2
WHERE id=$1::uuid
`, jobID, errText)
	if err != nil {
		return err
	}

	return tx.Commit(ctx)
}

func (w *Worker) handleTallyLocal(ctx context.Context, job jobs.ClaimedJob) error {
	if job.ElectionID == nil || strings.TrimSpace(*job.ElectionID) == "" {
		_ = w.runner.MarkError(ctx, job.ID, "missing election_id in jobs row")
		return nil
	}
	eid := strings.TrimSpace(*job.ElectionID)

	_ = w.runner.UpdateProgress(ctx, job.ID, 20)

	out, code, err := tally.ComputeFromDB(ctx, w.db, eid)
	if err != nil {
		_ = w.runner.MarkError(ctx, job.ID, "tally compute error: "+err.Error())
		return nil
	}
	if code != "" {
		_ = w.runner.MarkError(ctx, job.ID, "tally failed: "+code)
		return nil
	}

	_ = w.runner.UpdateProgress(ctx, job.ID, 70)

	winnersJSON, _ := json.Marshal(out.Winners)
	paramsJSON, _ := json.Marshal(out.Params)
	metricsJSON, _ := json.Marshal(out.Metrics)
	protocolJSON, _ := json.Marshal(out.Protocol)

	tx, err := w.db.Begin(ctx)
	if err != nil {
		_ = w.runner.MarkError(ctx, job.ID, "db begin failed")
		return nil
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var resultID string
	var version int
	err = tx.QueryRow(ctx, `
		WITH nextv AS (
			SELECT COALESCE(MAX(version),0)+1 AS v
			FROM results
			WHERE election_id=$1::uuid
		)
		INSERT INTO results (election_id, version, method, params, winners, metrics, protocol)
		SELECT $1::uuid, nextv.v, $2, $3::jsonb, $4::jsonb, $5::jsonb, $6::jsonb
		FROM nextv
		RETURNING id::text, version
	`, eid, out.Method,
		string(paramsJSON),
		string(winnersJSON),
		string(metricsJSON),
		string(protocolJSON),
	).Scan(&resultID, &version)
	if err != nil {
		_ = w.runner.MarkError(ctx, job.ID, "insert results failed: "+err.Error())
		return nil
	}

	_, err = tx.Exec(ctx, `
		UPDATE elections
		SET status='results_ready'
		WHERE id=$1::uuid AND status='closed'
	`, eid)
	if err != nil {
		_ = w.runner.MarkError(ctx, job.ID, "update election status failed: "+err.Error())
		return nil
	}

	ref := map[string]any{
		"result_id":   resultID,
		"version":     version,
		"election_id": eid,
	}
	refJSON, _ := json.Marshal(ref)

	_, err = tx.Exec(ctx, `
		UPDATE jobs
		SET status='done', progress=100, finished_at=now(), error_text=NULL, result_ref=$2::jsonb
		WHERE id=$1::uuid
	`, job.ID, string(refJSON))
	if err != nil {
		_ = w.runner.MarkError(ctx, job.ID, "update job failed: "+err.Error())
		return nil
	}

	if err := tx.Commit(ctx); err != nil {
		_ = w.runner.MarkError(ctx, job.ID, "commit failed: "+err.Error())
		return nil
	}

	return nil
}

