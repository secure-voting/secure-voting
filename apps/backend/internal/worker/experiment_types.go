package worker

import "encoding/json"

type ExperimentRunTask struct {
	Kind             string          `json:"kind"`
	JobID            string          `json:"job_id"`
	RunID            string          `json:"run_id"`
	ExperimentID     string          `json:"experiment_id"`
	DatasetID        string          `json:"dataset_id"`
	ExperimentType   string          `json:"experiment_type"`
	ExperimentSeed   *int64          `json:"experiment_seed,omitempty"`
	ExperimentParams json.RawMessage `json:"experiment_params"`
	Dataset          DatasetInfo     `json:"dataset"`
}

type DatasetInfo struct {
	ID          string             `json:"id"`
	Name        string             `json:"name"`
	Description string             `json:"description,omitempty"`
	Source      string             `json:"source"`
	Format      string             `json:"format"`
	Candidates  []DatasetCandidate `json:"candidates"`
	Seed        *int64             `json:"seed,omitempty"`
	Parameters  map[string]any     `json:"parameters,omitempty"`
	CreatedAt   string             `json:"created_at"`
}

type DatasetCandidate struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type ExperimentRunResult struct {
	Kind      string         `json:"kind,omitempty"`
	RunID     string         `json:"run_id"`
	Status    string         `json:"status"`
	Winners   []string       `json:"winners,omitempty"`
	Metrics   map[string]any `json:"metrics,omitempty"`
	Protocol  any            `json:"protocol,omitempty"`
	Timings   map[string]any `json:"timings,omitempty"`
	Artifacts map[string]any `json:"artifacts,omitempty"`
	ErrorText string         `json:"error_text,omitempty"`
}