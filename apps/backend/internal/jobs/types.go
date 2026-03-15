package jobs

import "github.com/jackc/pgx/v5/pgxpool"

type Service struct {
	db *pgxpool.Pool
}

type Job struct {
	ID              string  `json:"id"`
	Kind            string  `json:"kind"`
	Status          string  `json:"status"`
	Progress        int     `json:"progress"`
	CreatedBy       string  `json:"created_by"`
	ElectionID      *string `json:"election_id,omitempty"`
	ExperimentID    *string `json:"experiment_id,omitempty"`
	ExperimentRunID *string `json:"experiment_run_id,omitempty"`
	ErrorText       *string `json:"error_text,omitempty"`
	CreatedAt       string  `json:"created_at"`
	StartedAt       *string `json:"started_at,omitempty"`
	FinishedAt      *string `json:"finished_at,omitempty"`
}

type ListFilter struct {
	Status *string
	Kind   *string
	Limit  int
	Offset int
}
