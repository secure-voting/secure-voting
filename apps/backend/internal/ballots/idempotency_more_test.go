package ballots

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func TestTryGetCached_NoRedis(t *testing.T) {
	svc := NewService(nil, nil, time.Minute)

	got, ok := svc.tryGetCached(context.Background(), "k")
	if ok {
		t.Fatalf("expected cache miss, got %+v", got)
	}
}

func TestTryGetCached_InvalidPayloads(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis: %v", err)
	}
	defer mr.Close()

	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer func() { _ = rdb.Close() }()

	svc := NewService(nil, rdb, time.Minute)
	ctx := context.Background()

	cases := []struct {
		name  string
		value string
	}{
		{name: "empty", value: ""},
		{name: "bad json", value: "{]"},
		{name: "missing ballot id", value: `{"ok":true,"status":"accepted"}`},
		{name: "missing status", value: `{"ok":true,"ballot_id":"b1"}`},
	}

	for _, tc := range cases {
		if err := rdb.Set(ctx, "k", tc.value, time.Minute).Err(); err != nil {
			t.Fatalf("%s: set: %v", tc.name, err)
		}

		got, ok := svc.tryGetCached(ctx, "k")
		if ok {
			t.Fatalf("%s: expected miss, got %+v", tc.name, got)
		}
	}
}

func TestCacheResp_AndTryGetCached_Success(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis: %v", err)
	}
	defer mr.Close()

	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer func() { _ = rdb.Close() }()

	svc := NewService(nil, rdb, time.Minute)
	ctx := context.Background()

	want := SubmitResp{
		Ok:       true,
		BallotID: "ballot-1",
		Status:   "accepted",
	}

	svc.cacheResp(ctx, "idem-key", want)

	got, ok := svc.tryGetCached(ctx, "idem-key")
	if !ok {
		t.Fatal("expected cache hit")
	}
	if got != want {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}
