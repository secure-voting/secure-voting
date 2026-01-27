package experiments

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Service struct {
	db *pgxpool.Pool
}

func NewService(db *pgxpool.Pool) *Service {
	return &Service{db: db}
}

type CreateReq struct {
	Type   string         `json:"type"`
	Params map[string]any  `json:"params,omitempty"`
	Seed   *int64          `json:"seed,omitempty"`
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

func (s *Service) Create(ctx context.Context, createdBy string, in CreateReq) (string, string, error) {
	if in.Type != "algo" && in.Type != "behavior" {
		return "", "invalid_type", nil
	}
	params := in.Params
	if params == nil {
		params = map[string]any{}
	}
	b, err := json.Marshal(params)
	if err != nil {
		return "", "", err
	}

	var seed any
	if in.Seed != nil {
		seed = *in.Seed
	} else {
		seed = nil
	}

	var id string
	err = s.db.QueryRow(ctx, `
		INSERT INTO experiments (type, params, created_by, status, seed)
		VALUES ($1, $2::jsonb, $3::uuid, 'draft', $4)
		RETURNING id::text
	`, in.Type, string(b), createdBy, seed).Scan(&id)
	if err != nil {
		return "", "", err
	}
	return id, "", nil
}

func (s *Service) List(ctx context.Context, p ListParams) ([]Experiment, error) {
	limit := p.Limit
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	offset := p.Offset
	if offset < 0 {
		offset = 0
	}

	q := `
		SELECT id::text, type, COALESCE(params,'{}'::jsonb), status, seed, created_by::text, created_at
		FROM experiments
	`
	var where []string
	var args []any
	i := 1
	if strings.TrimSpace(p.Type) != "" {
		where = append(where, "type=$"+strconv.Itoa(i))
		args = append(args, p.Type)
		i++
	}
	if strings.TrimSpace(p.Status) != "" {
		where = append(where, "status=$"+strconv.Itoa(i))
		args = append(args, p.Status)
		i++
	}
	if len(where) > 0 {
		q += " WHERE " + strings.Join(where, " AND ")
	}
	q += " ORDER BY created_at DESC LIMIT $" + strconv.Itoa(i) + " OFFSET $" + strconv.Itoa(i+1)
	args = append(args, limit, offset)

	rows, err := s.db.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Experiment
	for rows.Next() {
		var e Experiment
		var createdAt time.Time
		var params []byte
		if err := rows.Scan(&e.ID, &e.Type, &params, &e.Status, &e.Seed, &e.CreatedBy, &createdAt); err != nil {
			return nil, err
		}
		e.Params = params
		e.CreatedAt = createdAt.UTC().Format(time.RFC3339)
		out = append(out, e)
	}
	return out, nil
}

func (s *Service) Get(ctx context.Context, id string) (Experiment, string, error) {
	if _, err := uuid.Parse(id); err != nil {
		return Experiment{}, "invalid_id", nil
	}

	var e Experiment
	var createdAt time.Time
	var params []byte

	err := s.db.QueryRow(ctx, `
		SELECT id::text, type, COALESCE(params,'{}'::jsonb), status, seed, created_by::text, created_at
		FROM experiments
		WHERE id=$1::uuid
	`, id).Scan(&e.ID, &e.Type, &params, &e.Status, &e.Seed, &e.CreatedBy, &createdAt)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Experiment{}, "not_found", nil
		}
		return Experiment{}, "", err
	}

	e.Params = params
	e.CreatedAt = createdAt.UTC().Format(time.RFC3339)
	return e, "", nil
}
