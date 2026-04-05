package ballots

import (
	"context"
	"encoding/json"
)

func insertAuditTx(ctx context.Context, tx txLike, actorUserID, eventType string, details map[string]any) error {
	if details == nil {
		details = map[string]any{}
	}
	b, err := json.Marshal(details)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx,
		`INSERT INTO audit_log (actor_user_id, event_type, details)
                 VALUES ($1::uuid, $2, $3::jsonb)`,
		actorUserID, eventType, string(b),
	)
	return err
}
