package results

import (
	"context"
	"encoding/json"
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

type ResultResp struct {
	ElectionID  string          `json:"election_id"`
	Version     int             `json:"version"`
	Method      string          `json:"method"`
	Params      json.RawMessage `json:"params,omitempty"`
	Winners     json.RawMessage `json:"winners"`
	Metrics     json.RawMessage `json:"metrics,omitempty"`
	Protocol    json.RawMessage `json:"protocol,omitempty"`
	PublishedAt *string         `json:"published_at,omitempty"`
}

func (s *Service) Get(ctx context.Context, electionID, role, userID, email string) (ResultResp, string, error) {
	if _, err := uuid.Parse(electionID); err != nil {
		return ResultResp{}, "invalid_id", nil
	}

	var eStatus, accessMode, createdBy string
	var showAggregates bool

	err := resultQueryRowFn(ctx, s.db, `
		SELECT status, access_mode, show_aggregates, created_by::text
		FROM elections
		WHERE id=$1::uuid
	`, electionID).Scan(&eStatus, &accessMode, &showAggregates, &createdBy)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ResultResp{}, "not_found", nil
		}
		return ResultResp{}, "", err
	}

	if role == "admin" {
		if createdBy != userID {
			return ResultResp{}, "not_found", nil
		}
	} else {
		accessible := false

		switch accessMode {
		case "open":
			accessible = true
		case "invite":
			var x int
			err := resultQueryRowFn(ctx, s.db, `
				SELECT 1
				FROM election_invites
				WHERE election_id=$1::uuid
				  AND lower(email)=lower($2)
				  AND status = 'accepted'
				LIMIT 1
			`, electionID, email).Scan(&x)
			if err == nil {
				accessible = true
			} else if errors.Is(err, pgx.ErrNoRows) {
				accessible = false
			} else {
				return ResultResp{}, "", err
			}
		default:
			accessible = false
		}

		if !accessible {
			return ResultResp{}, "not_found", nil
		}

		if eStatus != "published" {
			return ResultResp{}, "not_published", nil
		}
	}

	var r ResultResp
	r.ElectionID = electionID

	var params, winners, metrics, protocol []byte
	var publishedAt *time.Time

	err = resultQueryRowFn(ctx, s.db, `
		SELECT version, method, COALESCE(params,'{}'::jsonb), winners,
		       COALESCE(metrics,'null'::jsonb), COALESCE(protocol,'null'::jsonb), published_at
		FROM results
		WHERE election_id=$1::uuid
		ORDER BY version DESC
		LIMIT 1
	`, electionID).Scan(&r.Version, &r.Method, &params, &winners, &metrics, &protocol, &publishedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ResultResp{}, "no_results", nil
		}
		return ResultResp{}, "", err
	}

	r.Winners = winners
	r.Params = params

	if string(metrics) != "null" {
		r.Metrics = metrics
	}
	if string(protocol) != "null" {
		r.Protocol = protocol
	}
	if publishedAt != nil {
		s := publishedAt.UTC().Format(time.RFC3339)
		r.PublishedAt = &s
	}

	if role != "admin" && !showAggregates {
		r.Params = nil
		r.Metrics = nil
		r.Protocol = nil
	}

	return r, "", nil
}
