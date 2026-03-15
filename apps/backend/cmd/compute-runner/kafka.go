package main

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"time"

	"github.com/segmentio/kafka-go"

	"secure-voting/apps/backend/internal/worker"
)

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

func decodeTask(msg kafka.Message) (worker.ExperimentRunTask, bool) {
	var task worker.ExperimentRunTask
	if err := json.Unmarshal(msg.Value, &task); err != nil {
		log.Printf("bad task json: %v", err)
		return worker.ExperimentRunTask{}, false
	}
	return task, true
}

func commitTask(ctx context.Context, r *kafka.Reader, msg kafka.Message) {
	_ = r.CommitMessages(ctx, msg)
}

func writeResult(ctx context.Context, w *kafka.Writer, res worker.ExperimentRunResult) error {
	out, _ := json.Marshal(res)

	key := []byte(res.RunID)
	return w.WriteMessages(ctx, kafka.Message{
		Key:   key,
		Value: out,
		Time:  time.Now().UTC(),
	})
}
