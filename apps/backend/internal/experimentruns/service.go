package experimentruns

import (
	"github.com/jackc/pgx/v5/pgxpool"
	"go.mongodb.org/mongo-driver/mongo"
)

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
	ID           string  `json:"id"`
	ExperimentID string  `json:"experiment_id"`
	DatasetID    string  `json:"dataset_id"`
	Status       string  `json:"status"`
	KernelTaskID *string `json:"kernel_task_id,omitempty"`
	StartedAt    *string `json:"started_at,omitempty"`
	FinishedAt   *string `json:"finished_at,omitempty"`
}

type Result struct {
	RunID     string         `json:"run_id" bson:"run_id"`
	Winners   []any          `json:"winners,omitempty" bson:"winners,omitempty"`
	Metrics   map[string]any `json:"metrics,omitempty" bson:"metrics,omitempty"`
	Timings   map[string]any `json:"timings,omitempty" bson:"timings,omitempty"`
	Artifacts map[string]any `json:"artifacts,omitempty" bson:"artifacts,omitempty"`
}
