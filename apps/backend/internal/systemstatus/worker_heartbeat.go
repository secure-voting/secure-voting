package systemstatus

import (
	"context"
	"encoding/json"
	"time"

	"github.com/redis/go-redis/v9"
)

const WorkerHeartbeatKey = "secure-voting:system:worker:heartbeat"

type workerHeartbeatPayload struct {
	LastSeen        string `json:"last_seen"`
	PollIntervalSec int64  `json:"poll_interval_seconds"`
}

type stringTTLGetter interface {
	Get(ctx context.Context, key string) *redis.StringCmd
	TTL(ctx context.Context, key string) *redis.DurationCmd
}

func PublishWorkerHeartbeat(ctx context.Context, rdb *redis.Client, pollInterval, ttl time.Duration) error {
	if rdb == nil {
		return nil
	}
	if ttl <= 0 {
		ttl = 20 * time.Second
	}
	if pollInterval <= 0 {
		pollInterval = 5 * time.Second
	}

	payload := workerHeartbeatPayload{
		LastSeen:        time.Now().UTC().Format(time.RFC3339),
		PollIntervalSec: int64(pollInterval / time.Second),
	}

	raw, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	return rdb.Set(ctx, WorkerHeartbeatKey, raw, ttl).Err()
}

func ReadWorkerStatus(ctx context.Context, rdb stringTTLGetter) (bool, string, map[string]any) {
	if rdb == nil {
		return false, "unavailable", map[string]any{
			"reason": "redis client is nil",
		}
	}

	raw, err := rdb.Get(ctx, WorkerHeartbeatKey).Result()
	if err == redis.Nil {
		return false, "stale", map[string]any{
			"reason": "heartbeat not found",
			"key":    WorkerHeartbeatKey,
		}
	}
	if err != nil {
		return false, "error", map[string]any{
			"reason": err.Error(),
			"key":    WorkerHeartbeatKey,
		}
	}

	ttl, ttlErr := rdb.TTL(ctx, WorkerHeartbeatKey).Result()

	var payload workerHeartbeatPayload
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		return false, "error", map[string]any{
			"reason": "invalid heartbeat payload",
			"key":    WorkerHeartbeatKey,
		}
	}

	details := map[string]any{
		"key":                   WorkerHeartbeatKey,
		"last_seen":             payload.LastSeen,
		"poll_interval_seconds": payload.PollIntervalSec,
	}

	if ttlErr == nil && ttl > 0 {
		details["heartbeat_ttl_seconds"] = int64(ttl / time.Second)
	}

	return true, "ready", details
}
