package middleware

import (
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

type RateLimiter struct {
	rdb          *redis.Client
	keyPrefix    string
	limit        int
	ttl          time.Duration
	methods      map[string]struct{}
	pathPrefixes []string
}

func NewRateLimiter(
	rdb *redis.Client,
	keyPrefix string,
	limit int,
	ttl time.Duration,
	methods []string,
	pathPrefixes []string,
) *RateLimiter {
	mm := make(map[string]struct{}, len(methods))
	for _, m := range methods {
		m = strings.ToUpper(strings.TrimSpace(m))
		if m != "" {
			mm[m] = struct{}{}
		}
	}

	pp := make([]string, 0, len(pathPrefixes))
	for _, p := range pathPrefixes {
		p = strings.TrimSpace(p)
		if p != "" {
			pp = append(pp, p)
		}
	}

	keyPrefix = strings.TrimSpace(keyPrefix)
	if keyPrefix == "" {
		keyPrefix = "rl"
	}

	return &RateLimiter{
		rdb:          rdb,
		keyPrefix:    keyPrefix,
		limit:        limit,
		ttl:          ttl,
		methods:      mm,
		pathPrefixes: pp,
	}
}

func (rl *RateLimiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if rl == nil || rl.rdb == nil || rl.limit <= 0 || rl.ttl <= 0 {
			next.ServeHTTP(w, r)
			return
		}

		if !rl.shouldLimit(r) {
			next.ServeHTTP(w, r)
			return
		}

		ip := clientIP(r)
		if ip == "" {
			next.ServeHTTP(w, r)
			return
		}

		key := rl.keyPrefix + ":" + ip

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

func (rl *RateLimiter) shouldLimit(r *http.Request) bool {
	if len(rl.methods) > 0 {
		if _, ok := rl.methods[strings.ToUpper(strings.TrimSpace(r.Method))]; !ok {
			return false
		}
	}

	if len(rl.pathPrefixes) == 0 {
		return true
	}

	path := strings.TrimSpace(r.URL.Path)
	for _, p := range rl.pathPrefixes {
		if strings.HasPrefix(path, p) {
			return true
		}
	}
	return false
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
