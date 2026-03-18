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

const experimentRunResultKind = "experiment_run_result"

var writeTaskMessageFn = func(ctx context.Context, w *Worker, msg kafka.Message) error {
	return w.kw.WriteMessages(ctx, msg)
}

var loadExperimentFn = func(w *Worker, ctx context.Context, experimentID string) (expType string, expSeed *int64, params json.RawMessage, code string, err error) {
	return w.loadExperiment(ctx, experimentID)
}

var loadDatasetMetaFn = func(w *Worker, ctx context.Context, datasetHex string) (DatasetInfo, string, error) {
	return w.loadDatasetMeta(ctx, datasetHex)
}

var markRunRunningFn = func(w *Worker, ctx context.Context, runID, kernelTaskID string) error {
	return w.markRunRunning(ctx, runID, kernelTaskID)
}

var failRunAndJobFn = func(w *Worker, ctx context.Context, runID, jobID, errText string) error {
	return w.failRunAndJob(ctx, runID, jobID, errText)
}

var fetchResultMessageFn = func(ctx context.Context, w *Worker) (kafka.Message, error) {
	return w.kr.FetchMessage(ctx)
}

var applyExperimentRunResultFn = func(ctx context.Context, w *Worker, res ExperimentRunResult) error {
	return w.applyExperimentRunResult(ctx, res)
}

var commitResultFn = func(ctx context.Context, w *Worker, msg kafka.Message) error {
	return w.commitResultMessage(ctx, msg)
}

type expRunPayload struct {
	ExperimentID string `json:"experiment_id"`
	DatasetID    string `json:"dataset_id"`
	RunID        string `json:"run_id"`
}

func (w *Worker) handleExperimentRun(ctx context.Context, job jobs.ClaimedJob) error {
	if job.ExperimentRunID == nil || strings.TrimSpace(*job.ExperimentRunID) == "" {
		_ = markJobErrorFn(w, ctx, job.ID, "missing experiment_run_id in jobs row")
		return nil
	}

	var pl expRunPayload
	if len(job.Payload) == 0 || string(job.Payload) == "null" {
		_ = markJobErrorFn(w, ctx, job.ID, "missing payload")
		return nil
	}
	if err := json.Unmarshal(job.Payload, &pl); err != nil {
		_ = markJobErrorFn(w, ctx, job.ID, "invalid payload json")
		return nil
	}

	pl.RunID = strings.TrimSpace(pl.RunID)
	pl.ExperimentID = strings.TrimSpace(pl.ExperimentID)
	pl.DatasetID = strings.TrimSpace(pl.DatasetID)

	if _, err := uuid.Parse(pl.RunID); err != nil {
		_ = markJobErrorFn(w, ctx, job.ID, "invalid run_id in payload")
		return nil
	}
	if _, err := uuid.Parse(pl.ExperimentID); err != nil {
		_ = markJobErrorFn(w, ctx, job.ID, "invalid experiment_id in payload")
		return nil
	}
	if pl.RunID != *job.ExperimentRunID {
		_ = markJobErrorFn(w, ctx, job.ID, "payload run_id mismatch with jobs.experiment_run_id")
		return nil
	}

	expType, expSeed, expParams, code, err := loadExperimentFn(w, ctx, pl.ExperimentID)
	if err != nil {
		return err
	}
	if code != "" {
		_ = failRunAndJobFn(w, ctx, pl.RunID, job.ID, "experiment not found")
		return nil
	}

	_ = updateProgressFn(w, ctx, job.ID, 15)

	ds, code, err := loadDatasetMetaFn(w, ctx, pl.DatasetID)
	if err != nil {
		return err
	}
	if code != "" {
		_ = failRunAndJobFn(w, ctx, pl.RunID, job.ID, "dataset not found")
		return nil
	}

	_ = updateProgressFn(w, ctx, job.ID, 30)

	if err := markRunRunningFn(w, ctx, pl.RunID, job.ID); err != nil {
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
		_ = failRunAndJobFn(w, ctx, pl.RunID, job.ID, "failed to marshal task")
		return nil
	}

	err = writeTaskMessageFn(ctx, w, kafka.Message{
		Key:   []byte(pl.RunID),
		Value: value,
		Time:  time.Now().UTC(),
	})
	if err != nil {
		_ = failRunAndJobFn(w, ctx, pl.RunID, job.ID, "kafka publish failed: "+err.Error())
		return nil
	}

	log.Printf("worker.handleExperimentRun: published task run_id=%s experiment_id=%s dataset_id=%s", pl.RunID, pl.ExperimentID, pl.DatasetID)

	_ = updateProgressFn(w, ctx, job.ID, 45)
	return nil
}

func (w *Worker) consumeResults(ctx context.Context) error {
	type resultKindEnvelope struct {
		Kind string `json:"kind"`
	}

	for {
		msg, err := fetchResultMessageFn(ctx, w)
		if err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			return err
		}

		log.Printf(
			"worker.consumeResults: fetched topic=%s partition=%d offset=%d key=%q bytes=%d",
			msg.Topic, msg.Partition, msg.Offset, string(msg.Key), len(msg.Value),
		)

		var env resultKindEnvelope
		if err := json.Unmarshal(msg.Value, &env); err != nil {
			log.Printf(
				"worker.consumeResults: bad json topic=%s partition=%d offset=%d: %v",
				msg.Topic, msg.Partition, msg.Offset, err,
			)
			if err := commitResultFn(ctx, w, msg); err != nil {
				return err
			}
			continue
		}

		kind := strings.TrimSpace(env.Kind)

		switch kind {
		case "", jobKindExperimentRun, experimentRunResultKind:
			var res ExperimentRunResult
			if err := json.Unmarshal(msg.Value, &res); err != nil {
				log.Printf(
					"worker.consumeResults: bad experiment result json topic=%s partition=%d offset=%d: %v",
					msg.Topic, msg.Partition, msg.Offset, err,
				)
				if err := commitResultFn(ctx, w, msg); err != nil {
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
				if err := commitResultFn(ctx, w, msg); err != nil {
					return err
				}
				continue
			}
			if _, err := uuid.Parse(res.RunID); err != nil {
				log.Printf(
					"worker.consumeResults: invalid run_id=%q topic=%s partition=%d offset=%d",
					res.RunID, msg.Topic, msg.Partition, msg.Offset,
				)
				if err := commitResultFn(ctx, w, msg); err != nil {
					return err
				}
				continue
			}

			res.Kind = strings.TrimSpace(res.Kind)
			if res.Kind != "" && res.Kind != jobKindExperimentRun && res.Kind != experimentRunResultKind {
				log.Printf(
					"worker.consumeResults: skip kind=%q run_id=%q topic=%s partition=%d offset=%d",
					res.Kind, res.RunID, msg.Topic, msg.Partition, msg.Offset,
				)
				if err := commitResultFn(ctx, w, msg); err != nil {
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
				if err := commitResultFn(ctx, w, msg); err != nil {
					return err
				}
				continue
			}

			log.Printf(
				"worker.consumeResults: applying result run_id=%q kind=%q status=%q",
				res.RunID, res.Kind, res.Status,
			)

			if err := applyExperimentRunResultFn(ctx, w, res); err != nil {
				log.Printf(
					"worker.applyExperimentRunResult error run_id=%q topic=%s partition=%d offset=%d: %v",
					res.RunID, msg.Topic, msg.Partition, msg.Offset, err,
				)
				time.Sleep(750 * time.Millisecond)
				continue
			}

			log.Printf("worker.consumeResults: applied result run_id=%q status=%q", res.RunID, res.Status)

			if err := commitResultFn(ctx, w, msg); err != nil {
				return err
			}

			log.Printf(
				"worker.consumeResults: committed topic=%s partition=%d offset=%d run_id=%q",
				msg.Topic, msg.Partition, msg.Offset, res.RunID,
			)

		case electionTallyTaskKind, electionTallyResultKind:
			var res ElectionTallyResult
			if err := json.Unmarshal(msg.Value, &res); err != nil {
				log.Printf(
					"worker.consumeResults: bad election result json topic=%s partition=%d offset=%d: %v",
					msg.Topic, msg.Partition, msg.Offset, err,
				)
				if err := commitResultFn(ctx, w, msg); err != nil {
					return err
				}
				continue
			}

			res.JobID = strings.TrimSpace(res.JobID)
			res.ElectionID = strings.TrimSpace(res.ElectionID)
			res.Kind = strings.TrimSpace(res.Kind)
			res.Status = strings.TrimSpace(res.Status)

			if res.JobID == "" || res.ElectionID == "" {
				log.Printf(
					"worker.consumeResults: missing job_id or election_id kind=%q topic=%s partition=%d offset=%d",
					res.Kind, msg.Topic, msg.Partition, msg.Offset,
				)
				if err := commitResultFn(ctx, w, msg); err != nil {
					return err
				}
				continue
			}

			if _, err := uuid.Parse(res.JobID); err != nil {
				log.Printf(
					"worker.consumeResults: invalid job_id=%q topic=%s partition=%d offset=%d",
					res.JobID, msg.Topic, msg.Partition, msg.Offset,
				)
				if err := commitResultFn(ctx, w, msg); err != nil {
					return err
				}
				continue
			}

			if _, err := uuid.Parse(res.ElectionID); err != nil {
				log.Printf(
					"worker.consumeResults: invalid election_id=%q topic=%s partition=%d offset=%d",
					res.ElectionID, msg.Topic, msg.Partition, msg.Offset,
				)
				if err := commitResultFn(ctx, w, msg); err != nil {
					return err
				}
				continue
			}

			if res.Status != "done" && res.Status != "error" {
				log.Printf(
					"worker.consumeResults: invalid election status=%q job_id=%q topic=%s partition=%d offset=%d",
					res.Status, res.JobID, msg.Topic, msg.Partition, msg.Offset,
				)
				if err := commitResultFn(ctx, w, msg); err != nil {
					return err
				}
				continue
			}

			log.Printf(
				"worker.consumeResults: applying election result job_id=%q election_id=%q kind=%q status=%q",
				res.JobID, res.ElectionID, res.Kind, res.Status,
			)

			if err := applyElectionTallyResultFn(ctx, w, res); err != nil {
				log.Printf(
					"worker.applyElectionTallyResult error job_id=%q election_id=%q topic=%s partition=%d offset=%d: %v",
					res.JobID, res.ElectionID, msg.Topic, msg.Partition, msg.Offset, err,
				)
				time.Sleep(750 * time.Millisecond)
				continue
			}

			log.Printf(
				"worker.consumeResults: applied election result job_id=%q election_id=%q status=%q",
				res.JobID, res.ElectionID, res.Status,
			)

			if err := commitResultFn(ctx, w, msg); err != nil {
				return err
			}

			log.Printf(
				"worker.consumeResults: committed topic=%s partition=%d offset=%d job_id=%q",
				msg.Topic, msg.Partition, msg.Offset, res.JobID,
			)

		default:
			log.Printf(
				"worker.consumeResults: skip unsupported kind=%q topic=%s partition=%d offset=%d",
				kind, msg.Topic, msg.Partition, msg.Offset,
			)
			if err := commitResultFn(ctx, w, msg); err != nil {
				return err
			}
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
