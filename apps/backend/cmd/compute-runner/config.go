package main

import (
	"log"
	"os"
	"strings"
	"time"
)

type Config struct {
	Brokers      []string
	TasksTopic   string
	ResultsTopic string
	GroupID      string

	MongoURI    string
	MongoDB     string
	PostgresDSN string

	GRPCAddr   string
	UseTLS     bool
	CACertPath string
	ServerName string

	RunTimeout time.Duration

	KafkaMinBytes        int
	KafkaMaxBytes        int
	KafkaMaxWait         time.Duration
	KafkaBatchTimeout    time.Duration
	KafkaBallotBatchSize int

	KafkaTLS           bool
	KafkaTLSCA         string
	KafkaTLSServerName string
}

func loadConfig() Config {
	brokers := splitCSV(envOr("KAFKA_BROKERS", "kafka:9092"))

	cfg := Config{
		Brokers:      brokers,
		TasksTopic:   envOr("KAFKA_TASKS_TOPIC", "secure-voting.compute.tasks"),
		ResultsTopic: envOr("KAFKA_RESULTS_TOPIC", "secure-voting.compute.results"),
		GroupID:      envOr("KAFKA_GROUP_ID", "secure-voting-compute-runner"),

		MongoURI:    mustEnv("MONGO_URI"),
		MongoDB:     envOr("MONGO_DB", "secure_voting"),
		PostgresDSN: mustEnv("POSTGRES_DSN"),

		GRPCAddr:   envOr("COMPUTE_GRPC_ADDR", "rust-compute:50051"),
		UseTLS:     parseBool(envOr("COMPUTE_TLS", "false")),
		CACertPath: env("COMPUTE_TLS_CA"),
		ServerName: envOr("COMPUTE_TLS_SERVER_NAME", "rust-compute"),

		RunTimeout: 120 * time.Second,

		KafkaMinBytes:        1_000,
		KafkaMaxBytes:        10_000_000,
		KafkaMaxWait:         250 * time.Millisecond,
		KafkaBatchTimeout:    50 * time.Millisecond,
		KafkaBallotBatchSize: 500,

		KafkaTLS:           parseBool(envOr("KAFKA_TLS", "false")),
		KafkaTLSCA:         env("KAFKA_TLS_CA"),
		KafkaTLSServerName: envOr("KAFKA_TLS_SERVER_NAME", "kafka"),
	}

	if cfg.UseTLS && strings.TrimSpace(cfg.CACertPath) == "" {
		log.Fatalf("missing env COMPUTE_TLS_CA when COMPUTE_TLS=true")
	}
	if cfg.KafkaTLS && strings.TrimSpace(cfg.KafkaTLSCA) == "" {
		log.Fatalf("missing env KAFKA_TLS_CA when KAFKA_TLS=true")
	}

	return cfg
}

func env(key string) string {
	return strings.TrimSpace(os.Getenv(key))
}

func envOr(key, def string) string {
	v := env(key)
	if v == "" {
		return def
	}
	return v
}

func mustEnv(key string) string {
	v := env(key)
	if v == "" {
		log.Fatalf("missing env %s", key)
	}
	return v
}

func splitCSV(s string) []string {
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

func parseBool(s string) bool {
	s = strings.ToLower(strings.TrimSpace(s))
	return s == "1" || s == "true" || s == "yes" || s == "y" || s == "on"
}
