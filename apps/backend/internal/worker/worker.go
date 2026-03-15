package worker

import (
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/segmentio/kafka-go"
	"go.mongodb.org/mongo-driver/mongo"

	"secure-voting/apps/backend/internal/jobs"
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
	kw := newKafkaWriter(cfg)
	kr := newKafkaReader(cfg)

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
