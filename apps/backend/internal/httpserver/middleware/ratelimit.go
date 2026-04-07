package middleware

import (
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

type RateLimiter struct {
	rdb   *redis.Client
	limit int
	ttl   time.Duration
}

func NewRateLimiter(rdb *redis.Client, limit int, ttl time.Duration) *RateLimiter {
	return &RateLimiter{
		rdb:   rdb,
		limit: limit,
		ttl:   ttl,
	}
}

func (rl *RateLimiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if rl == nil || rl.rdb == nil || rl.limit <= 0 || rl.ttl <= 0 {
			next.ServeHTTP(w, r)
			return
		}

		if strings.HasPrefix(r.URL.Path, "/api/v1/auth") {
			next.ServeHTTP(w, r)
			return
		}

		ip := clientIP(r)
		if ip == "" {
			next.ServeHTTP(w, r)
			return
		}

		key := "rl:" + ip

		n, err := rl.rdb.Incr(r.Context(), key).Result()
		if err != nil {
			next.ServeHTTP(w, r)
			return
		}

		if n == 1 {
			_ = rl.rdb.Expire(r.Context(), key, rl.ttl).Err()
		}

		if int(n) > rl.limit {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte(`{"error":{"code":"rate_limited","message":"too many requests"}}`))
			return
		}

		next.ServeHTTP(w, r)
	})
}

func clientIP(r *http.Request) string {
	if xf := strings.TrimSpace(r.Header.Get("X-Forwarded-For")); xf != "" {
		parts := strings.Split(xf, ",")
		if len(parts) > 0 {
			return strings.TrimSpace(parts[0])
		}
	}

	ip, _, err := net.SplitHostPort(strings.TrimSpace(r.RemoteAddr))
	if err == nil {
		return strings.TrimSpace(ip)
	}

	return strings.TrimSpace(r.RemoteAddr)
}