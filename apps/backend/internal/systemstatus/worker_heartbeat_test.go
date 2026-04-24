package systemstatus

import (
	"context"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

type stubRedis struct {
	getFn func(ctx context.Context, key string) *redis.StringCmd
	ttlFn func(ctx context.Context, key string) *redis.DurationCmd
}

func (s stubRedis) Get(ctx context.Context, key string) *redis.StringCmd {
	return s.getFn(ctx, key)
}

func (s stubRedis) TTL(ctx context.Context, key string) *redis.DurationCmd {
	return s.ttlFn(ctx, key)
}

func TestReadWorkerStatus_HeartbeatMissing(t *testing.T) {
	ctx := context.Background()

	rdb := stubRedis{
		getFn: func(ctx context.Context, key string) *redis.StringCmd {
			cmd := redis.NewStringCmd(ctx)
			cmd.SetErr(redis.Nil)
			return cmd
		},
		ttlFn: func(ctx context.Context, key string) *redis.DurationCmd {
			cmd := redis.NewDurationCmd(ctx, time.Second)
			cmd.SetVal(0)
			return cmd
		},
	}

	ok, status, details := ReadWorkerStatus(ctx, rdb)
	if ok {
		t.Fatalf("expected ok=false")
	}
	if status != "stale" {
		t.Fatalf("unexpected status: %q", status)
	}
	if details["reason"] != "heartbeat not found" {
		t.Fatalf("unexpected details: %+v", details)
	}
}

func TestReadWorkerStatus_Success(t *testing.T) {
	ctx := context.Background()

	rdb := stubRedis{
		getFn: func(ctx context.Context, key string) *redis.StringCmd {
			cmd := redis.NewStringCmd(ctx)
			cmd.SetVal(`{"last_seen":"2026-04-19T12:00:00Z","poll_interval_seconds":5}`)
			return cmd
		},
		ttlFn: func(ctx context.Context, key string) *redis.DurationCmd {
			cmd := redis.NewDurationCmd(ctx, time.Second)
			cmd.SetVal(12 * time.Second)
			return cmd
		},
	}

	ok, status, details := ReadWorkerStatus(ctx, rdb)
	if !ok {
		t.Fatalf("expected ok=true")
	}
	if status != "ready" {
		t.Fatalf("unexpected status: %q", status)
	}
	if details["last_seen"] != "2026-04-19T12:00:00Z" {
		t.Fatalf("unexpected details: %+v", details)
	}
	if details["poll_interval_seconds"] != int64(5) {
		t.Fatalf("unexpected details: %+v", details)
	}
	if details["heartbeat_ttl_seconds"] != int64(12) {
		t.Fatalf("unexpected details: %+v", details)
	}
}