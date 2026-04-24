package worker

import (
	"context"
	"encoding/json"
	"time"
)

func (w *Worker) runSchedulers(ctx context.Context) error {
	if w == nil || w.db == nil {
		return nil
	}

	if err := w.autoCloseDueElections(ctx); err != nil {
		return err
	}
	if err := w.autoPublishDueElections(ctx); err != nil {
		return err
	}
	return nil
}

func (w *Worker) autoCloseDueElections(ctx context.Context) error {
	tx, err := w.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	rows, err := tx.Query(ctx, `
		SELECT id::text, created_by::text, status
		FROM elections
		WHERE status IN ('active', 'paused')
		  AND end_at <= now()
		ORDER BY end_at ASC
		FOR UPDATE SKIP LOCKED
	`)
	if err != nil {
		return err
	}
	defer rows.Close()

	type dueElection struct {
		ID        string
		CreatedBy string
		Status    string
	}
	due := make([]dueElection, 0)

	for rows.Next() {
		var item dueElection
		if err := rows.Scan(&item.ID, &item.CreatedBy, &item.Status); err != nil {
			return err
		}
		due = append(due, item)
	}
	if err := rows.Err(); err != nil {
		return err
	}

	for _, item := range due {
		_, err := tx.Exec(ctx, `
			UPDATE elections
			SET status = 'closed'
			WHERE id = $1::uuid
			  AND status IN ('active', 'paused')
		`, item.ID)
		if err != nil {
			return err
		}

		payload := map[string]any{
			"election_id": item.ID,
			"action":      "auto_close",
		}
		payloadJSON, err := json.Marshal(payload)
		if err != nil {
			return err
		}

		_, err = tx.Exec(ctx, `
			INSERT INTO jobs (kind, status, progress, created_by, election_id, payload)
			VALUES ('tally', 'queued', 0, $2::uuid, $1::uuid, $3::jsonb)
		`, item.ID, item.CreatedBy, string(payloadJSON))
		if err != nil {
			return err
		}

		details := map[string]any{
			"target_type": "election",
			"target_id":   item.ID,
			"before": map[string]any{
				"status": item.Status,
			},
			"after": map[string]any{
				"status": "closed",
			},
			"job": map[string]any{
				"kind":   "tally",
				"status": "queued",
			},
			"trigger": "scheduler",
		}
		detailsJSON, err := json.Marshal(details)
		if err != nil {
			return err
		}

		_, err = tx.Exec(ctx, `
			INSERT INTO audit_log (actor_user_id, event_type, details)
			VALUES (NULL, 'election_closed', $1::jsonb)
		`, string(detailsJSON))
		if err != nil {
			return err
		}
	}

	return tx.Commit(ctx)
}

func (w *Worker) autoPublishDueElections(ctx context.Context) error {
	tx, err := w.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	rows, err := tx.Query(ctx, `
		SELECT id::text
		FROM elections
		WHERE status = 'results_ready'
		  AND publish_at IS NOT NULL
		  AND publish_at <= now()
		ORDER BY publish_at ASC
		FOR UPDATE SKIP LOCKED
	`)
	if err != nil {
		return err
	}
	defer rows.Close()

	electionIDs := make([]string, 0)
	for rows.Next() {
		var electionID string
		if err := rows.Scan(&electionID); err != nil {
			return err
		}
		electionIDs = append(electionIDs, electionID)
	}
	if err := rows.Err(); err != nil {
		return err
	}

	now := time.Now().UTC()

	for _, electionID := range electionIDs {
		tag, err := tx.Exec(ctx, `
			WITH latest AS (
				SELECT id
				FROM results
				WHERE election_id = $1::uuid
				ORDER BY version DESC
				LIMIT 1
			)
			UPDATE results r
			SET published_at = COALESCE(r.published_at, $2)
			FROM latest
			WHERE r.id = latest.id
		`, electionID, now)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			continue
		}

		_, err = tx.Exec(ctx, `
			UPDATE elections
			SET status = 'published', published_at = $2
			WHERE id = $1::uuid
			  AND status = 'results_ready'
		`, electionID, now)
		if err != nil {
			return err
		}

		details := map[string]any{
			"target_type": "election",
			"target_id":   electionID,
			"before": map[string]any{
				"status": "results_ready",
			},
			"after": map[string]any{
				"status":       "published",
				"published_at": now.Format(time.RFC3339),
			},
			"trigger": "scheduler",
		}
		detailsJSON, err := json.Marshal(details)
		if err != nil {
			return err
		}

		_, err = tx.Exec(ctx, `
			INSERT INTO audit_log (actor_user_id, event_type, details)
			VALUES (NULL, 'election_published', $1::jsonb)
		`, string(detailsJSON))
		if err != nil {
			return err
		}
	}

	return tx.Commit(ctx)
}