package experimentruns

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/google/uuid"
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
		ok, err := batchCheckExperimentOwnerFn(ctx, s.db, expID, createdBy)
		if err != nil {
			return nil, "", err
		}
		if !ok {
			return nil, "not_found", nil
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

	ok, err := batchValidateDatasetsExistFn(s, ctx, oids)
	if err != nil {
		return nil, "", err
	}
	if !ok {
		return nil, "dataset_not_found", nil
	}

	tx, err := batchBeginTxFn(ctx, s.db)
	if err != nil {
		return nil, "", err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	out := make([]BatchItem, 0, len(unique))

	for _, dsid := range unique {
		runID, err := batchInsertRunFn(ctx, tx, expID, dsid)
		if err != nil {
			return nil, "", err
		}

		payload := map[string]any{
			"experiment_id": expID,
			"dataset_id":    dsid,
			"run_id":        runID,
		}
		pb, _ := json.Marshal(payload)

		jobID, err := batchInsertJobFn(ctx, tx, createdBy, expID, runID, string(pb))
		if err != nil {
			return nil, "", err
		}

		out = append(out, BatchItem{RunID: runID, JobID: jobID})
	}

	_ = batchAuditInsertFn(ctx, tx, createdBy, expID, len(out))

	if err := tx.Commit(ctx); err != nil {
		return nil, "", err
	}
	return out, "", nil
}
