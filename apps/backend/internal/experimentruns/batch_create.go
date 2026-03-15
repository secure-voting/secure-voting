package experimentruns

import (
	"context"
	"encoding/json"
	"errors"
	"secure-voting/apps/backend/internal/auditlog"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

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
