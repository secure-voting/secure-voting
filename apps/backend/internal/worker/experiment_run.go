package worker

import (
	"context"
	"encoding/json"
	"log"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/segmentio/kafka-go"

	"secure-voting/apps/backend/internal/jobs"
)

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
		Kind:             jobKindExperimentRun,
		JobID:            job.ID,
		RunID:            pl.RunID,
		ExperimentID:     pl.ExperimentID,
		DatasetID:        pl.DatasetID,
		ExperimentType:   expType,
		ExperimentSeed:   expSeed,
		ExperimentParams: expParams,
		Dataset:          ds,
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
		msg, err := w.kr.FetchMessage(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			return err
		}

		var res ExperimentRunResult
		if err := json.Unmarshal(msg.Value, &res); err != nil {
			log.Printf(
				"worker.consumeResults: bad json topic=%s partition=%d offset=%d: %v",
				msg.Topic, msg.Partition, msg.Offset, err,
			)
			if err := w.commitResultMessage(ctx, msg); err != nil {
				return err
			}
			continue
		}

		res.RunID = strings.TrimSpace(res.RunID)
		if res.RunID == "" {
			log.Printf(
				"worker.consumeResults: missing run_id topic=%s partition=%d offset=%d",
				msg.Topic, msg.Partition, msg.Offset,
			)
			if err := w.commitResultMessage(ctx, msg); err != nil {
				return err
			}
			continue
		}
		if _, err := uuid.Parse(res.RunID); err != nil {
			log.Printf(
				"worker.consumeResults: invalid run_id=%q topic=%s partition=%d offset=%d",
				res.RunID, msg.Topic, msg.Partition, msg.Offset,
			)
			if err := w.commitResultMessage(ctx, msg); err != nil {
				return err
			}
			continue
		}

		res.Kind = strings.TrimSpace(res.Kind)
		if res.Kind != "" && res.Kind != jobKindExperimentRun {
			log.Printf(
				"worker.consumeResults: skip kind=%q run_id=%q topic=%s partition=%d offset=%d",
				res.Kind, res.RunID, msg.Topic, msg.Partition, msg.Offset,
			)
			if err := w.commitResultMessage(ctx, msg); err != nil {
				return err
			}
			continue
		}

		res.Status = strings.TrimSpace(res.Status)
		if res.Status != "done" && res.Status != "error" {
			log.Printf(
				"worker.consumeResults: invalid status=%q run_id=%q topic=%s partition=%d offset=%d",
				res.Status, res.RunID, msg.Topic, msg.Partition, msg.Offset,
			)
			if err := w.commitResultMessage(ctx, msg); err != nil {
				return err
			}
			continue
		}

		if err := w.applyExperimentRunResult(ctx, res); err != nil {
			log.Printf(
				"worker.applyExperimentRunResult error run_id=%q topic=%s partition=%d offset=%d: %v",
				res.RunID, msg.Topic, msg.Partition, msg.Offset, err,
			)
			time.Sleep(750 * time.Millisecond)
			continue
		}

		if err := w.commitResultMessage(ctx, msg); err != nil {
			return err
		}
	}
}

func (w *Worker) commitResultMessage(ctx context.Context, msg kafka.Message) error {
	if err := w.kr.CommitMessages(ctx, msg); err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return err
	}
	return nil
}
