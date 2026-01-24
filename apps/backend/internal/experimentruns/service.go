package experimentruns

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/bson/primitive"

	"secure-voting/apps/backend/internal/auditlog"
)

type Service struct {
	db      *pgxpool.Pool
	mongodb *mongo.Database
}

func NewService(db *pgxpool.Pool, mongodb *mongo.Database) *Service {
	return &Service{db: db, mongodb: mongodb}
}

type BatchReq struct {
	ExperimentID string   `json:"experiment_id"`
	DatasetIDs   []string `json:"dataset_ids"`
}

type BatchItem struct {
	RunID string `json:"run_id"`
	JobID string `json:"job_id"`
}

type Run struct {
	ID          string  `json:"id"`
	ExperimentID string `json:"experiment_id"`
	DatasetID   string  `json:"dataset_id"`
	Status      string  `json:"status"`
	KernelTaskID *string `json:"kernel_task_id,omitempty"`
	StartedAt   *string `json:"started_at,omitempty"`
	FinishedAt  *string `json:"finished_at,omitempty"`
}

type Result struct {
	RunID    string         `json:"run_id" bson:"run_id"`
	Winners  []any          `json:"winners,omitempty" bson:"winners,omitempty"`
	Metrics  map[string]any `json:"metrics,omitempty" bson:"metrics,omitempty"`
	Timings  map[string]any `json:"timings,omitempty" bson:"timings,omitempty"`
	Artifacts map[string]any `json:"artifacts,omitempty" bson:"artifacts,omitempty"`
}

func (s *Service) BatchCreate(ctx context.Context, createdBy, role string, req BatchReq) ([]BatchItem, string, error) {
	expID := strings.TrimSpace(req.ExperimentID)
	if _, err := uuid.Parse(expID); err != nil {
		return nil, "invalid_experiment_id", nil
	}
	if len(req.DatasetIDs) == 0 {
		return nil, "dataset_ids_required", nil
	}

	if role != "admin" {
		var x int
		err := s.db.QueryRow(ctx, `
			SELECT 1 FROM experiments
			WHERE id=$1::uuid AND created_by=$2::uuid
		`, expID, createdBy).Scan(&x)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return nil, "not_found", nil
			}
			return nil, "", err
		}
	}

	unique := make([]string, 0, len(req.DatasetIDs))
	seen := make(map[string]struct{}, len(req.DatasetIDs))
	oids := make([]primitive.ObjectID, 0, len(req.DatasetIDs))

	for _, dsidRaw := range req.DatasetIDs {
		dsid := strings.TrimSpace(dsidRaw)
		if dsid == "" {
			return nil, "invalid_dataset_id", nil
		}
		if _, ok := seen[dsid]; ok {
			continue
		}
		oid, err := primitive.ObjectIDFromHex(dsid)
		if err != nil {
			return nil, "invalid_dataset_id", nil
		}
		seen[dsid] = struct{}{}
		unique = append(unique, dsid)
		oids = append(oids, oid)
	}

	ok, err := s.validateDatasetsExist(ctx, oids)
	if err != nil {
		return nil, "", err
	}
	if !ok {
		return nil, "dataset_not_found", nil
	}

	tx, err := s.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, "", err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	out := make([]BatchItem, 0, len(unique))

	for _, dsid := range unique {
		var runID string
		err := tx.QueryRow(ctx, `
			INSERT INTO experiment_runs (experiment_id, dataset_id, status)
			VALUES ($1::uuid, $2, 'queued')
			RETURNING id::text
		`, expID, dsid).Scan(&runID)
		if err != nil {
			return nil, "", err
		}

		payload := map[string]any{
			"experiment_id": expID,
			"dataset_id":    dsid,
			"run_id":        runID,
		}
		pb, _ := json.Marshal(payload)

		var jobID string
		err = tx.QueryRow(ctx, `
			INSERT INTO jobs (kind, status, progress, created_by, experiment_id, experiment_run_id, payload)
			VALUES ('experiment_run', 'queued', 0, $1::uuid, $2::uuid, $3::uuid, $4::jsonb)
			RETURNING id::text
		`, createdBy, expID, runID, string(pb)).Scan(&jobID)
		if err != nil {
			return nil, "", err
		}

		out = append(out, BatchItem{RunID: runID, JobID: jobID})
	}

	_ = auditlog.Insert(ctx, tx, &createdBy, "experiment_runs_batch_created", map[string]any{
		"target_type": "experiment",
		"target_id":   expID,
		"after": map[string]any{
			"count": len(out),
		},
	})

	if err := tx.Commit(ctx); err != nil {
		return nil, "", err
	}
	return out, "", nil
}

func (s *Service) validateDatasetsExist(ctx context.Context, oids []primitive.ObjectID) (bool, error) {
	if len(oids) == 0 {
		return false, nil
	}
	coll := s.mongodb.Collection("datasets")

	filter := bson.M{"_id": bson.M{"$in": oids}}
	cnt, err := coll.CountDocuments(ctx, filter)
	if err != nil {
		return false, err
	}
	return int(cnt) == len(oids), nil
}

func (s *Service) List(ctx context.Context, role, userID, experimentID string) ([]Run, string, error) {
	args := []any{}
	q := `
		SELECT r.id::text, r.experiment_id::text, r.dataset_id, r.status, r.kernel_task_id, r.started_at, r.finished_at,
		       e.created_by::text
		FROM experiment_runs r
		JOIN experiments e ON e.id = r.experiment_id
		WHERE 1=1
	`
	argn := 1

	if experimentID != "" {
		if _, err := uuid.Parse(strings.TrimSpace(experimentID)); err != nil {
			return nil, "invalid_experiment_id", nil
		}
		q += ` AND r.experiment_id = $` + itoa(argn) + `::uuid`
		args = append(args, experimentID)
		argn++
	}

	if role != "admin" {
		q += ` AND e.created_by = $` + itoa(argn) + `::uuid`
		args = append(args, userID)
		argn++
	}

	q += ` ORDER BY r.started_at NULLS LAST, r.id DESC`

	rows, err := s.db.Query(ctx, q, args...)
	if err != nil {
		return nil, "", err
	}
	defer rows.Close()

	var out []Run
	for rows.Next() {
		var r Run
		var kernel *string
		var started, finished *time.Time
		var createdBy string
		if err := rows.Scan(&r.ID, &r.ExperimentID, &r.DatasetID, &r.Status, &kernel, &started, &finished, &createdBy); err != nil {
			return nil, "", err
		}
		r.KernelTaskID = kernel
		if started != nil {
			s := started.UTC().Format(time.RFC3339)
			r.StartedAt = &s
		}
		if finished != nil {
			s := finished.UTC().Format(time.RFC3339)
			r.FinishedAt = &s
		}
		out = append(out, r)
	}

	return out, "", nil
}

func (s *Service) Get(ctx context.Context, role, userID, runID string) (Run, string, error) {
	if _, err := uuid.Parse(strings.TrimSpace(runID)); err != nil {
		return Run{}, "invalid_id", nil
	}

	var r Run
	var kernel *string
	var started, finished *time.Time
	var createdBy string

	err := s.db.QueryRow(ctx, `
		SELECT r.id::text, r.experiment_id::text, r.dataset_id, r.status, r.kernel_task_id, r.started_at, r.finished_at,
		       e.created_by::text
		FROM experiment_runs r
		JOIN experiments e ON e.id = r.experiment_id
		WHERE r.id = $1::uuid
	`, runID).Scan(&r.ID, &r.ExperimentID, &r.DatasetID, &r.Status, &kernel, &started, &finished, &createdBy)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Run{}, "not_found", nil
		}
		return Run{}, "", err
	}

	if role != "admin" && createdBy != userID {
		return Run{}, "not_found", nil
	}

	r.KernelTaskID = kernel
	if started != nil {
		s := started.UTC().Format(time.RFC3339)
		r.StartedAt = &s
	}
	if finished != nil {
		s := finished.UTC().Format(time.RFC3339)
		r.FinishedAt = &s
	}

	return r, "", nil
}

func (s *Service) GetResult(ctx context.Context, role, userID, runID string) (Result, string, error) {
	_, code, err := s.Get(ctx, role, userID, runID)
	if err != nil {
		return Result{}, "", err
	}
	if code != "" {
		return Result{}, code, nil
	}

	var res Result
	err = s.mongodb.Collection("experiment_results").FindOne(ctx, bson.M{"run_id": runID}).Decode(&res)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return Result{}, "not_found", nil
		}
		return Result{}, "", err
	}
	return res, "", nil
}

func (s *Service) DownloadResult(ctx context.Context, role, userID, runID string) ([]byte, string, string, string, error) {
	res, code, err := s.GetResult(ctx, role, userID, runID)
	if err != nil {
		return nil, "", "", "", err
	}
	if code != "" {
		return nil, "", "", code, nil
	}

	b, _ := json.Marshal(res)
	return b, "experiment_result_" + runID + ".json", "application/json", "", nil
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	neg := false
	if i < 0 {
		neg = true
		i = -i
	}
	var b [32]byte
	pos := len(b)
	for i > 0 {
		pos--
		b[pos] = byte('0' + (i % 10))
		i /= 10
	}
	if neg {
		pos--
		b[pos] = '-'
	}
	return string(b[pos:])
}
