package config

import (
	"os"
	"strconv"
	"time"
)

type Config struct {
	HTTPAddr        string
	ShutdownTimeout time.Duration

	PostgresDSN string
	TokenTTL    time.Duration

	RedisAddr      string
	RedisPassword  string
	IdempotencyTTL time.Duration

	MongoURI    string
	MongoDBName string

	MaxUploadBytes int64
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

	tokenTTL := 30 * 24 * time.Hour
	if ttlStr := os.Getenv("TOKEN_TTL"); ttlStr != "" {
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

	idemTTL := 24 * time.Hour
	if s := os.Getenv("IDEMPOTENCY_TTL"); s != "" {
		if parsed, err := time.ParseDuration(s); err == nil {
			idemTTL = parsed
		}
	}

	mongoDB := os.Getenv("MONGO_DB")
	if mongoDB == "" {
		mongoDB = "secure_voting"
	}

	mongoURI := os.Getenv("MONGO_URI")
	if mongoURI == "" {
		// дефолт под compose: пользователь root/rootpass, authSource=admin
		mu := os.Getenv("MONGO_INITDB_ROOT_USERNAME")
		if mu == "" {
			mu = "root"
		}
		mp := os.Getenv("MONGO_INITDB_ROOT_PASSWORD")
		if mp == "" {
			mp = "mongo_dev_pass"
		}
		mongoURI = "mongodb://" + mu + ":" + mp + "@mongo:27017/?authSource=admin"
	}

	maxUpload := int64(10 << 20) // 10 MiB
	if s := os.Getenv("MAX_UPLOAD_BYTES"); s != "" {
		if v, err := strconv.ParseInt(s, 10, 64); err == nil && v > 0 {
			maxUpload = v
		}
	}

	return Config{
		HTTPAddr:        addr,
		ShutdownTimeout: 10 * time.Second,

		PostgresDSN: dsn,
		TokenTTL:    tokenTTL,

		RedisAddr:      redisAddr,
		RedisPassword:  redisPass,
		IdempotencyTTL: idemTTL,

		MongoURI:    mongoURI,
		MongoDBName: mongoDB,

		MaxUploadBytes: maxUpload,
	}
}
