package elections

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func (s *Service) Action(ctx context.Context, electionID, adminUserID, action string) (string, error) {
	if _, err := uuid.Parse(electionID); err != nil {
		return "invalid_id", nil
	}

	tx, err := s.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return "", err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var status string
	var tallyRule string
	var ballotFormat string
	var committeeSize *int
	var rankingTopK *int

	err = tx.QueryRow(ctx, `
		SELECT status, tally_rule, ballot_format, committee_size, ranking_top_k
		FROM elections
		WHERE id = $1::uuid AND created_by = $2::uuid
		FOR UPDATE
	`, electionID, adminUserID).Scan(
		&status,
		&tallyRule,
		&ballotFormat,
		&committeeSize,
		&rankingTopK,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "not_found", nil
		}
		return "", err
	}

	var next string
	if action == "open" {
		if !canOpenElection(status) {
			return "invalid_transition", nil
		}

		var candidateCount int
		err = tx.QueryRow(ctx, `
			SELECT count(*)
			FROM candidates
			WHERE election_id = $1::uuid
		`, electionID).Scan(&candidateCount)
		if err != nil {
			return "", err
		}

		if err := validateElectionReadyToOpen(
			tallyRule,
			ballotFormat,
			committeeSize,
			rankingTopK,
			candidateCount,
		); err != nil {
			return actionValidationCode(err), nil
		}

		next = "active"
	} else {
		var ok bool
		next, ok = nextStatus(status, action)
		if !ok {
			return "invalid_transition", nil
		}
	}

	now := time.Now().UTC()

	switch action {
	case "close":
		_, err = tx.Exec(ctx, `
			UPDATE elections
			SET status = $2
			WHERE id = $1::uuid
		`, electionID, next)
		if err != nil {
			return "", err
		}

		payload := map[string]any{
			"election_id": electionID,
			"action":      "close",
		}
		pb, _ := json.Marshal(payload)

		_, err = tx.Exec(ctx, `
			INSERT INTO jobs (kind, status, progress, created_by, election_id, payload)
			VALUES ('tally', 'queued', 0, $2::uuid, $1::uuid, $3::jsonb)
		`, electionID, adminUserID, string(pb))
		if err != nil {
			return "", err
		}

	case "publish":
		_, err = tx.Exec(ctx, `
			UPDATE elections
			SET status = $2, published_at = $3
			WHERE id = $1::uuid
		`, electionID, next, now)
		if err != nil {
			return "", err
		}

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
			return "", err
		}
		if tag.RowsAffected() == 0 {
			return "no_results", nil
		}

	default:
		_, err = tx.Exec(ctx, `
			UPDATE elections
			SET status = $2
			WHERE id = $1::uuid
		`, electionID, next)
		if err != nil {
			return "", err
		}
	}

	eventType := map[string]string{
		"schedule": "election_scheduled",
		"open":     "election_opened",
		"pause":    "election_paused",
		"resume":   "election_resumed",
		"close":    "election_closed",
		"publish":  "election_published",
	}[action]

	details := map[string]any{
		"target_type": "election",
		"target_id":   electionID,
		"before": map[string]any{
			"status": status,
		},
		"after": map[string]any{
			"status": next,
		},
	}

	if action == "publish" {
		details["after"].(map[string]any)["published_at"] = now.Format(time.RFC3339)
	}

	if action == "close" {
		details["job"] = map[string]any{
			"kind":   "tally",
			"status": "queued",
		}
	}

	if eventType != "" {
		_ = insertAudit(ctx, tx, &adminUserID, eventType, details)
	}

	return "", tx.Commit(ctx)
}

func validateElectionReadyToOpen(
	rule string,
	ballotFormat string,
	committeeSize *int,
	rankingTopK *int,
	candidateCount int,
) error {
	if candidateCount < 2 {
		return errors.New("at least 2 candidates required")
	}

	if _, err := normalizeCommitteeSize(rule, committeeSize, candidateCount); err != nil {
		return err
	}

	if _, err := normalizeRankingTopK(ballotFormat, rankingTopK, candidateCount); err != nil {
		return err
	}

	return nil
}

func actionValidationCode(err error) string {
	if err == nil {
		return ""
	}

	if code := candidateNormalizationCode(err); code != "" && code != "invalid_candidates" {
		return code
	}
	if code := committeeSizeCode(err); code != "" {
		return code
	}
	if code := rankingTopKCode(err); code != "" {
		return code
	}

	return "invalid_configuration"
}
