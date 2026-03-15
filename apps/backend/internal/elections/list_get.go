package elections

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func (s *Service) ListForUser(ctx context.Context, userID, email, role string) ([]ElectionSummary, error) {
	var rows pgx.Rows
	var err error

	if role == "admin" {
		rows, err = s.db.Query(ctx, `
			SELECT id::text, title, description, status, access_mode, start_at, end_at, published_at
			FROM elections
			WHERE created_by = $1::uuid
			ORDER BY created_at DESC
		`, userID)
	} else {
		rows, err = s.db.Query(ctx, `
			SELECT e.id::text, e.title, e.description, e.status, e.access_mode, e.start_at, e.end_at, e.published_at
			FROM elections e
			WHERE e.status IN ('scheduled','active','paused','closed','results_ready','published')
			AND (
				e.access_mode = 'open'
				OR EXISTS (
					SELECT 1
					FROM election_invites i
					WHERE i.election_id = e.id
					AND lower(i.email) = lower($1)
					AND i.status = 'accepted'
				)
			)
			ORDER BY e.created_at DESC
		`, email)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]ElectionSummary, 0)
	for rows.Next() {
		var e ElectionSummary
		var startAt, endAt time.Time
		var publishedAt *time.Time
		if err := rows.Scan(&e.ID, &e.Title, &e.Description, &e.Status, &e.AccessMode, &startAt, &endAt, &publishedAt); err != nil {
			return nil, err
		}
		e.StartAt = startAt.UTC().Format(time.RFC3339)
		e.EndAt = endAt.UTC().Format(time.RFC3339)
		if publishedAt != nil {
			sv := publishedAt.UTC().Format(time.RFC3339)
			e.PublishedAt = &sv
		}
		out = append(out, e)
	}
	return out, nil
}

func (s *Service) Get(ctx context.Context, electionID, userID, email, role string) (ElectionDetail, string, error) {
	if _, err := uuid.Parse(electionID); err != nil {
		return ElectionDetail{}, "invalid_id", nil
	}

	allowed, err := s.isAccessible(ctx, electionID, userID, email, role)
	if err != nil {
		return ElectionDetail{}, "", err
	}
	if !allowed {
		return ElectionDetail{}, "not_found", nil
	}

	var d ElectionDetail
	d.ID = electionID

	var createdBy string
	var startAt, endAt time.Time
	var createdAt time.Time
	var publishAt, publishedAt *time.Time

	err = s.db.QueryRow(ctx, `
		SELECT
			e.title,
			e.description,
			e.status,
			e.access_mode,
			e.start_at,
			e.end_at,
			e.published_at,
			e.ballot_format,
			e.tally_rule,
			e.committee_size,
			e.quota_type,
			e.publish_at,
			e.show_aggregates,
			e.approval_max_choices,
			e.ranking_top_k,
			e.score_min,
			e.score_max,
			e.score_step,
			e.score_allow_skip,
			e.created_by::text,
			e.created_at
		FROM elections e
		WHERE e.id = $1::uuid
	`, electionID).Scan(
		&d.Title,
		&d.Description,
		&d.Status,
		&d.AccessMode,
		&startAt,
		&endAt,
		&publishedAt,
		&d.BallotFormat,
		&d.TallyRule,
		&d.CommitteeSize,
		&d.QuotaType,
		&publishAt,
		&d.ShowAggregates,
		&d.ApprovalMaxChoices,
		&d.RankingTopK,
		&d.ScoreMin,
		&d.ScoreMax,
		&d.ScoreStep,
		&d.ScoreAllowSkip,
		&createdBy,
		&createdAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ElectionDetail{}, "not_found", nil
		}
		return ElectionDetail{}, "", err
	}

	if role != "admin" && createdBy != userID && !isVisibleToNonOwnerStatus(d.Status) {
		return ElectionDetail{}, "not_found", nil
	}

	d.StartAt = startAt.UTC().Format(time.RFC3339)
	d.EndAt = endAt.UTC().Format(time.RFC3339)

	if publishAt != nil {
		v := publishAt.UTC().Format(time.RFC3339)
		d.PublishAt = &v
	}
	if publishedAt != nil {
		v := publishedAt.UTC().Format(time.RFC3339)
		d.PublishedAt = &v
	}

	rows, err := s.db.Query(ctx, `
		SELECT id::text, name, meta
		FROM candidates
		WHERE election_id = $1::uuid
		ORDER BY name
	`, electionID)
	if err != nil {
		return ElectionDetail{}, "", err
	}
	defer rows.Close()

	for rows.Next() {
		var c Candidate
		var metaJSON []byte

		if err := rows.Scan(&c.ID, &c.Name, &metaJSON); err != nil {
			return ElectionDetail{}, "", err
		}
		if len(metaJSON) > 0 && string(metaJSON) != "null" {
			_ = json.Unmarshal(metaJSON, &c.Meta)
		}

		d.Candidates = append(d.Candidates, c)
	}

	if err := rows.Err(); err != nil {
		return ElectionDetail{}, "", err
	}

	return d, "", nil
}
