package config

import (
	"os"
	"strconv"
	"strings"
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

	// Kafka / worker
	KafkaBrokers        []string
	KafkaTasksTopic     string
	KafkaResultsTopic   string
	KafkaGroupID        string
	WorkerPollInterval  time.Duration
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

	// Kafka defaults for compose
	brokers := splitCSV(os.Getenv("KAFKA_BROKERS"))
	if len(brokers) == 0 {
		brokers = []string{"kafka:9092"}
	}

	tasksTopic := os.Getenv("KAFKA_TASKS_TOPIC")
	if tasksTopic == "" {
		tasksTopic = "secure-voting.compute.tasks"
	}

	resultsTopic := os.Getenv("KAFKA_RESULTS_TOPIC")
	if resultsTopic == "" {
		resultsTopic = "secure-voting.compute.results"
	}

	groupID := os.Getenv("KAFKA_GROUP_ID")
	if groupID == "" {
		groupID = "secure-voting-backend-worker"
	}

	poll := 1 * time.Second
	if s := os.Getenv("WORKER_POLL_INTERVAL"); s != "" {
		if d, err := time.ParseDuration(s); err == nil && d > 0 {
			poll = d
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

		KafkaBrokers:       brokers,
		KafkaTasksTopic:    tasksTopic,
		KafkaResultsTopic:  resultsTopic,
		KafkaGroupID:       groupID,
		WorkerPollInterval: poll,
	}
}

func splitCSV(s string) []string {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}
