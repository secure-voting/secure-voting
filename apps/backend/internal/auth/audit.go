package auth

import (
	"context"
	"encoding/json"

	"github.com/jackc/pgx/v5"
)

func (s *Service) insertAudit(ctx context.Context, tx pgx.Tx, actorUserID *string, eventType string, details map[string]any) error {
	if details == nil {
		details = map[string]any{}
	}
	b, err := json.Marshal(details)
	if err != nil {
		return err
	}

	if actorUserID == nil {
		_, err = tx.Exec(ctx,
			`INSERT INTO audit_log (actor_user_id, event_type, details)
			 VALUES (NULL, $1, $2::jsonb)`,
			eventType, string(b),
		)
		return err
	}

	_, err = tx.Exec(ctx,
		`INSERT INTO audit_log (actor_user_id, event_type, details)
		 VALUES ($1::uuid, $2, $3::jsonb)`,
		*actorUserID, eventType, string(b),
	)
	return err
}
