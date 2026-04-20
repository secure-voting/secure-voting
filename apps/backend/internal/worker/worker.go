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

	pollInterval     time.Duration
	scheduleInterval time.Duration
	nextScheduleAt   time.Time
}

type Config struct {
	PollInterval     time.Duration
	ScheduleInterval time.Duration

	TasksTopic   string
	ResultsTopic string
	GroupID      string
	Brokers      []string

	KafkaTLS           bool
	KafkaTLSCA         string
	KafkaTLSServerName string
}

func New(db *pgxpool.Pool, mdb *mongo.Database, cfg Config) *Worker {
	kw := newKafkaWriter(cfg)
	kr := newKafkaReader(cfg)

	pi := cfg.PollInterval
	if pi <= 0 {
		pi = 1 * time.Second
	}

	si := cfg.ScheduleInterval
	if si <= 0 {
		si = 5 * time.Second
	}

	return &Worker{
		db:               db,
		mdb:              mdb,
		runner:           jobs.NewRunner(db),
		kw:               kw,
		kr:               kr,
		pollInterval:     pi,
		scheduleInterval: si,
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