package main

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"strings"
	"time"

	"github.com/segmentio/kafka-go"

	"secure-voting/apps/backend/internal/worker"
)

type decodedTask struct {
	Kind          string
	Experiment    *worker.ExperimentRunTask
	ElectionTally *worker.ElectionTallyTask
}

type taskEnvelope struct {
	Kind string `json:"kind"`
}

func newTaskReader(cfg Config) *kafka.Reader {
	return kafka.NewReader(kafka.ReaderConfig{
		Brokers:     cfg.Brokers,
		GroupID:     cfg.GroupID,
		Topic:       cfg.TasksTopic,
		MinBytes:    cfg.KafkaMinBytes,
		MaxBytes:    cfg.KafkaMaxBytes,
		MaxWait:     cfg.KafkaMaxWait,
		StartOffset: kafka.FirstOffset,
	})
}

func newResultWriter(cfg Config) *kafka.Writer {
	return &kafka.Writer{
		Addr:         kafka.TCP(cfg.Brokers...),
		Topic:        cfg.ResultsTopic,
		RequiredAcks: kafka.RequireAll,
		Balancer:     &kafka.LeastBytes{},
		BatchTimeout: cfg.KafkaBatchTimeout,
	}
}

func fetchTaskMessage(ctx context.Context, r *kafka.Reader) (kafka.Message, bool, error) {
	msg, err := r.FetchMessage(ctx)
	if err != nil {
		if errors.Is(err, context.Canceled) {
			return kafka.Message{}, false, nil
		}
		return kafka.Message{}, false, err
	}
	return msg, true, nil
}

func decodeTask(msg kafka.Message) (decodedTask, bool) {
	var env taskEnvelope
	if err := json.Unmarshal(msg.Value, &env); err != nil {
		log.Printf("bad task json: %v", err)
		return decodedTask{}, false
	}

	kind := strings.TrimSpace(env.Kind)
	switch kind {
	case "", "experiment_run":
		var task worker.ExperimentRunTask
		if err := json.Unmarshal(msg.Value, &task); err != nil {
			log.Printf("bad experiment task json: %v", err)
			return decodedTask{}, false
		}
		if strings.TrimSpace(task.Kind) == "" {
			task.Kind = "experiment_run"
		}
		return decodedTask{
			Kind:       "experiment_run",
			Experiment: &task,
		}, true

	case "election_tally":
		var task worker.ElectionTallyTask
		if err := json.Unmarshal(msg.Value, &task); err != nil {
			log.Printf("bad election_tally task json: %v", err)
			return decodedTask{}, false
		}
		if strings.TrimSpace(task.Kind) == "" {
			task.Kind = "election_tally"
		}
		return decodedTask{
			Kind:          "election_tally",
			ElectionTally: &task,
		}, true

	default:
		log.Printf("unsupported task kind: %q", kind)
		return decodedTask{}, false
	}
}

func commitTask(ctx context.Context, r *kafka.Reader, msg kafka.Message) {
	_ = r.CommitMessages(ctx, msg)
}

func writeResult(ctx context.Context, w *kafka.Writer, res any) error {
	out, err := json.Marshal(res)
	if err != nil {
		return err
	}

	var key []byte
	switch v := res.(type) {
	case worker.ExperimentRunResult:
		key = []byte(v.RunID)
	case worker.ElectionTallyResult:
		key = []byte(v.JobID)
	default:
		return errors.New("unsupported result type")
	}

	return w.WriteMessages(ctx, kafka.Message{
		Key:   key,
		Value: out,
		Time:  time.Now().UTC(),
	})
}
