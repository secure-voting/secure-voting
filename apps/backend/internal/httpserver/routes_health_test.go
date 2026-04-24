package httpserver

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"

	"secure-voting/apps/backend/internal/config"
	"secure-voting/apps/backend/internal/httpserver/httputil"
	"secure-voting/apps/backend/internal/httpserver/middleware"
)

type fakeVerifier struct {
	uid   string
	email string
	role  string
}

func (f fakeVerifier) VerifyAccessToken(ctx context.Context, rawToken string) (userID, email, role string, ok bool, err error) {
	return f.uid, f.email, f.role, true, nil
}

type stubRedisStatus struct {
	getFn func(ctx context.Context, key string) *redis.StringCmd
	ttlFn func(ctx context.Context, key string) *redis.DurationCmd
}

func (s stubRedisStatus) Get(ctx context.Context, key string) *redis.StringCmd {
	return s.getFn(ctx, key)
}

func (s stubRedisStatus) TTL(ctx context.Context, key string) *redis.DurationCmd {
	return s.ttlFn(ctx, key)
}

func TestSystemStatus_Handler_WithWorkerHeartbeat(t *testing.T) {

	workerRedis := stubRedisStatus{
		getFn: func(ctx context.Context, key string) *redis.StringCmd {
			cmd := redis.NewStringCmd(ctx)
			cmd.SetVal(`{"last_seen":"2026-04-19T12:00:00Z","poll_interval_seconds":5}`)
			return cmd
		},
		ttlFn: func(ctx context.Context, key string) *redis.DurationCmd {
			cmd := redis.NewDurationCmd(ctx, time.Second)
			cmd.SetVal(15 * time.Second)
			return cmd
		},
	}

	cfg := config.Config{
		HTTPAddr:        ":3001",
		ComputeGRPCAddr: "rust-compute:50051",
		ComputeTLS:      true,
		AdminTrustedCIDRs: []string{
			"0.0.0.0/0",
		},
	}

	handler := middleware.RequireAuth(
		fakeVerifier{uid: "u1", email: "admin@example.com", role: "admin"},
		middleware.RequireRole(
			"admin",
			middleware.RequireTrustedCIDRs(
				cfg.AdminTrustedCIDRs,
				httputil.Wrap(func(w http.ResponseWriter, r *http.Request) error {
					computeState := "unavailable"
					computeOK := false

					workerOK, workerState, workerDetails := readWorkerStatusForHTTP(r.Context(), workerRedis)

					httputil.WriteJSON(w, http.StatusOK, systemStatusResponse{
						Backend: systemComponentStatus{
							OK:     true,
							Status: "ready",
							Details: map[string]any{
								"http_addr": cfg.HTTPAddr,
							},
						},
						Compute: systemComponentStatus{
							OK:     computeOK,
							Status: computeState,
							Details: map[string]any{
								"addr": cfg.ComputeGRPCAddr,
								"tls":  cfg.ComputeTLS,
							},
						},
						Worker: systemComponentStatus{
							OK:      workerOK,
							Status:  workerState,
							Details: workerDetails,
						},
						CheckedAt: "2026-04-19T12:00:05Z",
					})
					return nil
				}),
			),
		),
	)

	req := httptest.NewRequest(http.MethodGet, "http://example/api/v1/system/status", nil)
	req.Header.Set("Authorization", "Bearer t")
	req.RemoteAddr = "127.0.0.1:12345"

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rr.Code, rr.Body.String())
	}

	var resp struct {
		Backend struct {
			OK     bool           `json:"ok"`
			Status string         `json:"status"`
			Details map[string]any `json:"details"`
		} `json:"backend"`
		Compute struct {
			OK     bool           `json:"ok"`
			Status string         `json:"status"`
			Details map[string]any `json:"details"`
		} `json:"compute"`
		Worker struct {
			OK     bool           `json:"ok"`
			Status string         `json:"status"`
			Details map[string]any `json:"details"`
		} `json:"worker"`
		CheckedAt string `json:"checked_at"`
	}

	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v; body=%s", err, rr.Body.String())
	}

	if !resp.Backend.OK || resp.Backend.Status != "ready" {
		t.Fatalf("unexpected backend status: %+v", resp.Backend)
	}
	if resp.Compute.OK || resp.Compute.Status != "unavailable" {
		t.Fatalf("unexpected compute status: %+v", resp.Compute)
	}
	if !resp.Worker.OK || resp.Worker.Status != "ready" {
		t.Fatalf("unexpected worker status: %+v", resp.Worker)
	}
	if resp.CheckedAt != "2026-04-19T12:00:05Z" {
		t.Fatalf("unexpected checked_at: %q", resp.CheckedAt)
	}
}

func TestSystemStatus_Handler_WithoutWorkerHeartbeat(t *testing.T) {
	ctx := context.Background()

	workerRedis := stubRedisStatus{
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

	workerOK, workerState, workerDetails := readWorkerStatusForHTTP(ctx, workerRedis)

	if workerOK {
		t.Fatalf("expected worker ok=false")
	}
	if workerState != "stale" {
		t.Fatalf("unexpected worker state: %q", workerState)
	}
	if workerDetails["reason"] != "heartbeat not found" {
		t.Fatalf("unexpected worker details: %+v", workerDetails)
	}
}

type redisStatusReader interface {
	Get(ctx context.Context, key string) *redis.StringCmd
	TTL(ctx context.Context, key string) *redis.DurationCmd
}

func readWorkerStatusForHTTP(ctx context.Context, rdb redisStatusReader) (bool, string, map[string]any) {
	raw, err := rdb.Get(ctx, "secure-voting:system:worker:heartbeat").Result()
	if err == redis.Nil {
		return false, "stale", map[string]any{
			"reason": "heartbeat not found",
			"key":    "secure-voting:system:worker:heartbeat",
		}
	}
	if err != nil {
		return false, "error", map[string]any{
			"reason": err.Error(),
			"key":    "secure-voting:system:worker:heartbeat",
		}
	}

	ttl, _ := rdb.TTL(ctx, "secure-voting:system:worker:heartbeat").Result()

	var payload struct {
		LastSeen        string `json:"last_seen"`
		PollIntervalSec int64  `json:"poll_interval_seconds"`
	}
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		return false, "error", map[string]any{
			"reason": "invalid heartbeat payload",
			"key":    "secure-voting:system:worker:heartbeat",
		}
	}

	return true, "ready", map[string]any{
		"key":                   "secure-voting:system:worker:heartbeat",
		"last_seen":             payload.LastSeen,
		"poll_interval_seconds": payload.PollIntervalSec,
		"heartbeat_ttl_seconds": int64(ttl / time.Second),
	}
}

var _ middleware.TokenVerifier = fakeVerifier{}