package experiments

import (
	"context"
	"encoding/json"
	"strings"
)

func (s *Service) Create(ctx context.Context, createdBy string, in CreateReq) (string, string, error) {
	if strings.TrimSpace(createdBy) == "" {
		return "", "unauthorized", nil
	}

	in.Type = norm(in.Type)
	if !allowedTypes[in.Type] {
		return "", "invalid_type", nil
	}

	params := in.Params
	if params == nil {
		params = map[string]any{}
	}

	if code := validateParams(params); code != "" {
		return "", code, nil
	}

	b, err := json.Marshal(params)
	if err != nil {
		return "", "", err
	}

	var seed any
	if in.Seed != nil {
		seed = *in.Seed
	}

	var id string
	err = createExperimentQueryRowFn(ctx, s.db, `
		INSERT INTO experiments (type, params, created_by, status, seed)
		VALUES ($1, $2::jsonb, $3::uuid, 'draft', $4)
		RETURNING id::text
	`, in.Type, string(b), createdBy, seed).Scan(&id)
	if err != nil {
		return "", "", err
	}

	_ = insertAuditFn(ctx, s.db, createdBy, "experiment_created", map[string]any{
		"target_type": "experiment",
		"target_id":   id,
		"after": map[string]any{
			"type":   in.Type,
			"status": "draft",
		},
	})

	return id, "", nil
}
