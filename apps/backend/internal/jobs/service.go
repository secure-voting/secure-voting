package jobs

import (
	"context"
	"errors"
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

type Job struct {
	ID             string  `json:"id"`
	Kind           string  `json:"kind"`
	Status         string  `json:"status"`
	Progress       int     `json:"progress"`
	CreatedBy      string  `json:"created_by"`
	ElectionID     *string `json:"election_id,omitempty"`
	ExperimentID   *string `json:"experiment_id,omitempty"`
	ExperimentRunID *string `json:"experiment_run_id,omitempty"`
	ErrorText      *string `json:"error_text,omitempty"`
	CreatedAt      string  `json:"created_at"`
	StartedAt      *string `json:"started_at,omitempty"`
	FinishedAt     *string `json:"finished_at,omitempty"`
}

type ListFilter struct {
	Status *string
	Kind   *string
	Limit  int
	Offset int
}

func (s *Service) List(ctx context.Context, role, userID string, f ListFilter) ([]Job, error) {
	limit := f.Limit
	if limit <= 0 || limit > 200 {
		limit = 50
	}

	// admin видит все; остальные — только свои
	base := `
		SELECT id::text, kind, status, progress, created_by::text,
		       election_id::text, experiment_id::text, experiment_run_id::text,
		       error_text, created_at, started_at, finished_at
		FROM jobs
		WHERE 1=1
	`
	args := []any{}
	argn := 1

	if role != "admin" {
		base += ` AND created_by = $` + itoa(argn)
		args = append(args, userID)
		argn++
	}

	if f.Status != nil {
		base += ` AND status = $` + itoa(argn)
		args = append(args, *f.Status)
		argn++
	}
	if f.Kind != nil {
		base += ` AND kind = $` + itoa(argn)
		args = append(args, *f.Kind)
		argn++
	}

	base += ` ORDER BY created_at DESC LIMIT $` + itoa(argn) + ` OFFSET $` + itoa(argn+1)
	args = append(args, limit, f.Offset)

	rows, err := s.db.Query(ctx, base, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Job
	for rows.Next() {
		var j Job
		var createdAt time.Time
		var startedAt, finishedAt *time.Time

		if err := rows.Scan(
			&j.ID, &j.Kind, &j.Status, &j.Progress, &j.CreatedBy,
			&j.ElectionID, &j.ExperimentID, &j.ExperimentRunID,
			&j.ErrorText, &createdAt, &startedAt, &finishedAt,
		); err != nil {
			return nil, err
		}

		j.CreatedAt = createdAt.UTC().Format(time.RFC3339)
		if startedAt != nil {
			s := startedAt.UTC().Format(time.RFC3339)
			j.StartedAt = &s
		}
		if finishedAt != nil {
			s := finishedAt.UTC().Format(time.RFC3339)
			j.FinishedAt = &s
		}

		out = append(out, j)
	}
	return out, nil
}

func (s *Service) Get(ctx context.Context, role, userID, jobID string) (Job, string, error) {
	if _, err := uuid.Parse(jobID); err != nil {
		return Job{}, "invalid_id", nil
	}

	var j Job
	var createdAt time.Time
	var startedAt, finishedAt *time.Time

	q := `
		SELECT id::text, kind, status, progress, created_by::text,
		       election_id::text, experiment_id::text, experiment_run_id::text,
		       error_text, created_at, started_at, finished_at
		FROM jobs
		WHERE id = $1::uuid
	`
	err := s.db.QueryRow(ctx, q, jobID).Scan(
		&j.ID, &j.Kind, &j.Status, &j.Progress, &j.CreatedBy,
		&j.ElectionID, &j.ExperimentID, &j.ExperimentRunID,
		&j.ErrorText, &createdAt, &startedAt, &finishedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Job{}, "not_found", nil
		}
		return Job{}, "", err
	}

	if role != "admin" && j.CreatedBy != userID {
		return Job{}, "not_found", nil
	}

	j.CreatedAt = createdAt.UTC().Format(time.RFC3339)
	if startedAt != nil {
		s := startedAt.UTC().Format(time.RFC3339)
		j.StartedAt = &s
	}
	if finishedAt != nil {
		s := finishedAt.UTC().Format(time.RFC3339)
		j.FinishedAt = &s
	}

	return j, "", nil
}

func itoa(i int) string {
	// без strconv ради минимума импортов
	if i == 0 {
		return "0"
	}
	neg := false
	if i < 0 {
		neg = true
		i = -i
	}
	var b [32]byte
	pos := len(b)
	for i > 0 {
		pos--
		b[pos] = byte('0' + (i % 10))
		i /= 10
	}
	if neg {
		pos--
		b[pos] = '-'
	}
	return string(b[pos:])
}
