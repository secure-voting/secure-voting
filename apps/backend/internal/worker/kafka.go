package worker

import (
	"time"

	"github.com/segmentio/kafka-go"
)

func newKafkaWriter(cfg Config) *kafka.Writer {
	return &kafka.Writer{
		Addr:         kafka.TCP(cfg.Brokers...),
		Topic:        cfg.TasksTopic,
		RequiredAcks: kafka.RequireAll,
		Balancer:     &kafka.LeastBytes{},
		BatchTimeout: 50 * time.Millisecond,
	}
}

func newKafkaReader(cfg Config) *kafka.Reader {
	return kafka.NewReader(kafka.ReaderConfig{
		Brokers:        cfg.Brokers,
		GroupID:        cfg.GroupID,
		Topic:          cfg.ResultsTopic,
		MinBytes:       1e3,
		MaxBytes:       10e6,
		MaxWait:        250 * time.Millisecond,
		StartOffset:    kafka.FirstOffset,
		CommitInterval: 0,
	})
}
