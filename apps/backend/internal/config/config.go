package config

import (
	"os"
	"time"
)

type Config struct {
	HTTPAddr        string
	ShutdownTimeout time.Duration

	PostgresDSN string
	TokenTTL    time.Duration

	RedisAddr        string
	RedisPassword    string
	IdempotencyTTL   time.Duration
}

func FromEnv() Config {
	addr := os.Getenv("HTTP_ADDR")
	if addr == "" {
		addr = ":3001"
	}

	pgPass := os.Getenv("POSTGRES_PASSWORD")
	if pgPass == "" {
		pgPass = "postgres_dev_pass"
	}

	dsn := os.Getenv("POSTGRES_DSN")
	if dsn == "" {
		dsn = "postgres://admin:" + pgPass + "@db:5432/secure-voting?sslmode=disable"
	}

	ttlStr := os.Getenv("TOKEN_TTL")
	tokenTTL := 30 * 24 * time.Hour
	if ttlStr != "" {
		if parsed, err := time.ParseDuration(ttlStr); err == nil {
			tokenTTL = parsed
		}
	}

	redisAddr := os.Getenv("REDIS_ADDR")
	if redisAddr == "" {
		redisAddr = "cache:6379"
	}
	redisPass := os.Getenv("REDIS_PASSWORD")
	if redisPass == "" {
		redisPass = "redis_dev_pass"
	}

	idemStr := os.Getenv("IDEMPOTENCY_TTL")
	idemTTL := 24 * time.Hour
	if idemStr != "" {
		if parsed, err := time.ParseDuration(idemStr); err == nil {
			idemTTL = parsed
		}
	}

	return Config{
		HTTPAddr:        addr,
		ShutdownTimeout: 10 * time.Second,
		PostgresDSN:     dsn,
		TokenTTL:        tokenTTL,
		RedisAddr:       redisAddr,
		RedisPassword:   redisPass,
		IdempotencyTTL:  idemTTL,
	}
}
