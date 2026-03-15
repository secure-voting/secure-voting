package experiments

import "encoding/json"

type CreateReq struct {
	Type   string         `json:"type"`
	Params map[string]any `json:"params,omitempty"`
	Seed   *int64         `json:"seed,omitempty"`
}

type Experiment struct {
	ID        string          `json:"id"`
	Type      string          `json:"type"`
	Params    json.RawMessage `json:"params"`
	Status    string          `json:"status"`
	Seed      *int64          `json:"seed,omitempty"`
	CreatedBy string          `json:"created_by"`
	CreatedAt string          `json:"created_at"`
}

type ListParams struct {
	Type   string
	Status string
	Limit  int
	Offset int
}
