package elections

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func (s *Service) isAccessible(ctx context.Context, electionID, userID, email, role string) (bool, error) {
	var accessMode string
	var createdBy string

	err := s.db.QueryRow(ctx, `
		SELECT access_mode, created_by::text
		FROM elections
		WHERE id=$1::uuid
	`, electionID).Scan(&accessMode, &createdBy)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return false, nil
		}
		return false, err
	}

	if userID != "" && createdBy == userID {
		return true, nil
	}

	if accessMode == "open" {
		return true, nil
	}

	var x int
	err = s.db.QueryRow(ctx, `
		SELECT 1
		FROM election_invites i
		WHERE i.election_id=$1::uuid
		  AND lower(i.email)=lower($2)
		  AND i.status = 'accepted'
		LIMIT 1
	`, electionID, email).Scan(&x)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return false, nil
		}
		return false, err
	}

	return true, nil
}

func isVisibleToNonOwnerStatus(status string) bool {
	switch status {
	case "scheduled", "active", "paused", "closed", "results_ready", "published":
		return true
	default:
		return false
	}
}

func nextStatus(cur, action string) (string, bool) {
	switch action {
	case "schedule":
		return "scheduled", cur == "draft"
	case "open":
		return "active", cur == "scheduled"
	case "pause":
		return "paused", cur == "active"
	case "resume":
		return "active", cur == "paused"
	case "close":
		return "closed", cur == "active" || cur == "paused"
	case "publish":
		return "published", cur == "closed" || cur == "results_ready"
	default:
		return "", false
	}
}

func generateInviteCode() (raw string, hashHex string) {
	b := make([]byte, 12)
	_, _ = rand.Read(b)
	raw = hex.EncodeToString(b)
	h := sha256.Sum256([]byte(raw))
	hashHex = hex.EncodeToString(h[:])
	return raw, hashHex
}

func nullableJSON(b []byte) any {
	if len(b) == 0 {
		return nil
	}
	return string(b)
}
func insertAudit(ctx context.Context, tx any, actorUserID *string, eventType string, details map[string]any) error {
	if details == nil {
		details = map[string]any{}
	}
	b, err := json.Marshal(details)
	if err != nil {
		return err
	}

	switch v := tx.(type) {
	case pgx.Tx:
		if actorUserID == nil {
			_, err = v.Exec(ctx,
				`INSERT INTO audit_log (actor_user_id, event_type, details)
				 VALUES (NULL, $1, $2::jsonb)`,
				eventType, string(b),
			)
			return err
		}
		_, err = v.Exec(ctx,
			`INSERT INTO audit_log (actor_user_id, event_type, details)
			 VALUES ($1::uuid, $2, $3::jsonb)`,
			*actorUserID, eventType, string(b),
		)
		return err
	case *pgxpool.Pool:
		if actorUserID == nil {
			_, err = v.Exec(ctx,
				`INSERT INTO audit_log (actor_user_id, event_type, details)
				 VALUES (NULL, $1, $2::jsonb)`,
				eventType, string(b),
			)
			return err
		}
		_, err = v.Exec(ctx,
			`INSERT INTO audit_log (actor_user_id, event_type, details)
			 VALUES ($1::uuid, $2, $3::jsonb)`,
			*actorUserID, eventType, string(b),
		)
		return err
	default:
		return nil
	}
}

type ElectionDetail struct {
	ID          string  `json:"id"`
	Title       string  `json:"title"`
	Description *string `json:"description,omitempty"`

	StartAt string `json:"start_at"`
	EndAt   string `json:"end_at"`

	TallyRule    string `json:"tally_rule"`
	BallotFormat string `json:"ballot_format"`

	CommitteeSize *int    `json:"committee_size,omitempty"`
	QuotaType     *string `json:"quota_type,omitempty"`

	Status         string  `json:"status"`
	AccessMode     string  `json:"access_mode"`
	PublishAt      *string `json:"publish_at,omitempty"`
	PublishedAt    *string `json:"published_at,omitempty"`
	ShowAggregates bool    `json:"show_aggregates"`

	ApprovalMaxChoices *int `json:"approval_max_choices,omitempty"`
	RankingTopK        *int `json:"ranking_top_k,omitempty"`
	ScoreMin           *int `json:"score_min,omitempty"`
	ScoreMax           *int `json:"score_max,omitempty"`
	ScoreStep          *int `json:"score_step,omitempty"`
	ScoreAllowSkip     bool `json:"score_allow_skip"`

	Candidates []Candidate `json:"candidates"`
}
