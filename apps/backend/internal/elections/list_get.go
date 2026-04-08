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
			SELECT
				e.id::text,
				e.title,
				e.description,
				e.status,
				e.access_mode,
				e.start_at,
				e.end_at,
				e.published_at,
				u.email,
				e.ballot_format,
				e.tally_rule,
				(
					SELECT count(*)::int
					FROM candidates c
					WHERE c.election_id = e.id
				) AS candidate_count
			FROM elections e
			JOIN users u ON u.id = e.created_by
			WHERE e.created_by = $1::uuid
			ORDER BY e.created_at DESC
		`, userID)
	} else {
		rows, err = s.db.Query(ctx, `
			SELECT
				e.id::text,
				e.title,
				e.description,
				e.status,
				e.access_mode,
				e.start_at,
				e.end_at,
				e.published_at,
				u.email,
				e.ballot_format,
				e.tally_rule,
				(
					SELECT count(*)::int
					FROM candidates c
					WHERE c.election_id = e.id
				) AS candidate_count
			FROM elections e
			JOIN users u ON u.id = e.created_by
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

		if err := rows.Scan(
			&e.ID,
			&e.Title,
			&e.Description,
			&e.Status,
			&e.AccessMode,
			&startAt,
			&endAt,
			&publishedAt,
			&e.OrganizerEmail,
			&e.BallotFormat,
			&e.TallyRule,
			&e.CandidateCount,
		); err != nil {
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

	if err := rows.Err(); err != nil {
		return nil, err
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
	var organizerEmail string
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
			e.created_at,
			u.email
		FROM elections e
		JOIN users u ON u.id = e.created_by
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
		&organizerEmail,
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

	d.CreatedBy = createdBy
	d.CreatedAt = createdAt.UTC().Format(time.RFC3339)
	d.OrganizerEmail = organizerEmail

	if role == "admin" && createdBy == userID {
		var submittedBallotsCount int
		if err := s.db.QueryRow(ctx, `
			SELECT count(*)
			FROM ballots
			WHERE election_id = $1::uuid
			  AND status = 'accepted'
		`, electionID).Scan(&submittedBallotsCount); err != nil {
			return ElectionDetail{}, "", err
		}
		d.SubmittedBallotsCount = &submittedBallotsCount

		if d.AccessMode == "invite" {
			var invitesTotalCount int
			var invitesAcceptedCount int
			var invitesPendingCount int
			var invitesRevokedCount int
			var invitesFailedCount int

			if err := s.db.QueryRow(ctx, `
				SELECT
					count(*) AS total_count,
					count(*) FILTER (WHERE status = 'accepted') AS accepted_count,
					count(*) FILTER (WHERE status IN ('created', 'sent')) AS pending_count,
					count(*) FILTER (WHERE status = 'revoked') AS revoked_count,
					count(*) FILTER (WHERE status = 'failed') AS failed_count
				FROM election_invites
				WHERE election_id = $1::uuid
			`, electionID).Scan(
				&invitesTotalCount,
				&invitesAcceptedCount,
				&invitesPendingCount,
				&invitesRevokedCount,
				&invitesFailedCount,
			); err != nil {
				return ElectionDetail{}, "", err
			}

			d.InvitesTotalCount = &invitesTotalCount
			d.InvitesAcceptedCount = &invitesAcceptedCount
			d.InvitesPendingCount = &invitesPendingCount
			d.InvitesRevokedCount = &invitesRevokedCount
			d.InvitesFailedCount = &invitesFailedCount

			var invitesRegistrationRequiredCount int
			if err := s.db.QueryRow(ctx, `
				SELECT count(DISTINCT lower(a.details #>> '{after,email}'))
				FROM audit_log a
				WHERE a.event_type = 'invite_registration_required'
				  AND a.details->>'target_type' = 'election'
				  AND a.details->>'target_id' = $1
				  AND NOT EXISTS (
					SELECT 1
					FROM election_invites i
					WHERE i.election_id = $1::uuid
					  AND lower(i.email) = lower(a.details #>> '{after,email}')
				  )
			`, electionID).Scan(&invitesRegistrationRequiredCount); err != nil {
				return ElectionDetail{}, "", err
			}

			d.InvitesRegistrationRequiredCount = &invitesRegistrationRequiredCount
		}
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
