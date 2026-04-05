package config

import (
	"reflect"
	"testing"
	"time"
)

func TestSplitCSV(t *testing.T) {
	got := splitCSV(" a, b ,, c ")
	want := []string{"a", "b", "c"}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("splitCSV mismatch: got=%#v want=%#v", got, want)
	}

	if splitCSV("   ") != nil {
		t.Fatal("expected nil for empty csv")
	}
}

func TestFromEnv_Defaults(t *testing.T) {
	keys := []string{
		"HTTP_ADDR",
		"POSTGRES_PASSWORD",
		"POSTGRES_DSN",
		"TOKEN_TTL",
		"REDIS_ADDR",
		"REDIS_PASSWORD",
		"IDEMPOTENCY_TTL",
		"MONGO_DB",
		"MONGO_URI",
		"MONGO_INITDB_ROOT_USERNAME",
		"MONGO_INITDB_ROOT_PASSWORD",
		"MAX_UPLOAD_BYTES",
		"KAFKA_BROKERS",
		"KAFKA_TASKS_TOPIC",
		"KAFKA_RESULTS_TOPIC",
		"KAFKA_GROUP_ID",
		"WORKER_POLL_INTERVAL",
		"COMPUTE_GRPC_ADDR",
		"COMPUTE_TLS",
		"COMPUTE_TLS_CA",
		"COMPUTE_TLS_SERVER_NAME",
		"BOOTSTRAP_ADMIN_EMAIL",
		"BOOTSTRAP_ADMIN_PASSWORD",
		"BOOTSTRAP_RESEARCHER_EMAIL",
		"BOOTSTRAP_RESEARCHER_PASSWORD",
	}
	for _, k := range keys {
		t.Setenv(k, "")
	}

	cfg := FromEnv()

	if cfg.HTTPAddr != ":3001" {
		t.Fatalf("unexpected HTTPAddr: %q", cfg.HTTPAddr)
	}
	if cfg.PostgresDSN != "postgres://admin:postgres_dev_pass@db:5432/secure-voting?sslmode=disable" {
		t.Fatalf("unexpected PostgresDSN: %q", cfg.PostgresDSN)
	}
	if cfg.TokenTTL != 30*24*time.Hour {
		t.Fatalf("unexpected TokenTTL: %v", cfg.TokenTTL)
	}
	if cfg.RedisAddr != "cache:6379" || cfg.RedisPassword != "redis_dev_pass" {
		t.Fatalf("unexpected redis config: %#v", cfg)
	}
	if cfg.IdempotencyTTL != 24*time.Hour {
		t.Fatalf("unexpected IdempotencyTTL: %v", cfg.IdempotencyTTL)
	}
	if cfg.MongoDBName != "secure_voting" {
		t.Fatalf("unexpected MongoDBName: %q", cfg.MongoDBName)
	}
	if cfg.MongoURI != "mongodb://root:mongo_dev_pass@mongo:27017/?authSource=admin" {
		t.Fatalf("unexpected MongoURI: %q", cfg.MongoURI)
	}
	if cfg.MaxUploadBytes != int64(10<<20) {
		t.Fatalf("unexpected MaxUploadBytes: %d", cfg.MaxUploadBytes)
	}
	if !reflect.DeepEqual(cfg.KafkaBrokers, []string{"kafka:9092"}) {
		t.Fatalf("unexpected KafkaBrokers: %#v", cfg.KafkaBrokers)
	}
	if cfg.KafkaTasksTopic != "secure-voting.compute.tasks" {
		t.Fatalf("unexpected tasks topic: %q", cfg.KafkaTasksTopic)
	}
	if cfg.KafkaResultsTopic != "secure-voting.compute.results" {
		t.Fatalf("unexpected results topic: %q", cfg.KafkaResultsTopic)
	}
	if cfg.KafkaGroupID != "secure-voting-backend-worker" {
		t.Fatalf("unexpected group id: %q", cfg.KafkaGroupID)
	}
	if cfg.WorkerPollInterval != time.Second {
		t.Fatalf("unexpected poll interval: %v", cfg.WorkerPollInterval)
	}
	if cfg.ComputeGRPCAddr != "rust-compute:50051" {
		t.Fatalf("unexpected compute addr: %q", cfg.ComputeGRPCAddr)
	}
	if cfg.ComputeTLS != true {
		t.Fatalf("expected ComputeTLS=true by default")
	}
	if cfg.ComputeTLSCA != "/certs/ca.pem" {
		t.Fatalf("unexpected compute tls ca: %q", cfg.ComputeTLSCA)
	}
	if cfg.ComputeTLSServerName != "rust-compute" {
		t.Fatalf("unexpected compute tls server name: %q", cfg.ComputeTLSServerName)
	}
	if cfg.ShutdownTimeout != 10*time.Second {
		t.Fatalf("unexpected shutdown timeout: %v", cfg.ShutdownTimeout)
	}
}

func TestFromEnv_CustomValues(t *testing.T) {
	t.Setenv("HTTP_ADDR", ":9999")
	t.Setenv("POSTGRES_DSN", "postgres://custom")
	t.Setenv("TOKEN_TTL", "48h")
	t.Setenv("REDIS_ADDR", "redis:6380")
	t.Setenv("REDIS_PASSWORD", "secret")
	t.Setenv("IDEMPOTENCY_TTL", "12h")
	t.Setenv("MONGO_DB", "dbx")
	t.Setenv("MONGO_URI", "mongodb://custom")
	t.Setenv("MAX_UPLOAD_BYTES", "12345")
	t.Setenv("KAFKA_BROKERS", "k1:9092, k2:9092")
	t.Setenv("KAFKA_TASKS_TOPIC", "tasks-x")
	t.Setenv("KAFKA_RESULTS_TOPIC", "results-x")
	t.Setenv("KAFKA_GROUP_ID", "group-x")
	t.Setenv("WORKER_POLL_INTERVAL", "5s")
	t.Setenv("COMPUTE_GRPC_ADDR", "compute:6000")
	t.Setenv("COMPUTE_TLS", "false")
	t.Setenv("COMPUTE_TLS_CA", "/tmp/ca.pem")
	t.Setenv("COMPUTE_TLS_SERVER_NAME", "compute.local")
	t.Setenv("BOOTSTRAP_ADMIN_EMAIL", "admin@example.com")
	t.Setenv("BOOTSTRAP_ADMIN_PASSWORD", "adminpass")
	t.Setenv("BOOTSTRAP_RESEARCHER_EMAIL", "researcher@example.com")
	t.Setenv("BOOTSTRAP_RESEARCHER_PASSWORD", "researcherpass")

	cfg := FromEnv()

	if cfg.HTTPAddr != ":9999" {
		t.Fatalf("unexpected HTTPAddr: %q", cfg.HTTPAddr)
	}
	if cfg.PostgresDSN != "postgres://custom" {
		t.Fatalf("unexpected PostgresDSN: %q", cfg.PostgresDSN)
	}
	if cfg.TokenTTL != 48*time.Hour {
		t.Fatalf("unexpected TokenTTL: %v", cfg.TokenTTL)
	}
	if cfg.RedisAddr != "redis:6380" || cfg.RedisPassword != "secret" {
		t.Fatalf("unexpected redis config: %#v", cfg)
	}
	if cfg.IdempotencyTTL != 12*time.Hour {
		t.Fatalf("unexpected IdempotencyTTL: %v", cfg.IdempotencyTTL)
	}
	if cfg.MongoDBName != "dbx" || cfg.MongoURI != "mongodb://custom" {
		t.Fatalf("unexpected mongo config: %#v", cfg)
	}
	if cfg.MaxUploadBytes != 12345 {
		t.Fatalf("unexpected MaxUploadBytes: %d", cfg.MaxUploadBytes)
	}
	if !reflect.DeepEqual(cfg.KafkaBrokers, []string{"k1:9092", "k2:9092"}) {
		t.Fatalf("unexpected KafkaBrokers: %#v", cfg.KafkaBrokers)
	}
	if cfg.KafkaTasksTopic != "tasks-x" || cfg.KafkaResultsTopic != "results-x" || cfg.KafkaGroupID != "group-x" {
		t.Fatalf("unexpected kafka config: %#v", cfg)
	}
	if cfg.WorkerPollInterval != 5*time.Second {
		t.Fatalf("unexpected poll: %v", cfg.WorkerPollInterval)
	}
	if cfg.ComputeGRPCAddr != "compute:6000" {
		t.Fatalf("unexpected compute addr: %q", cfg.ComputeGRPCAddr)
	}
	if cfg.ComputeTLS != false {
		t.Fatalf("expected ComputeTLS=false")
	}
	if cfg.ComputeTLSCA != "/tmp/ca.pem" || cfg.ComputeTLSServerName != "compute.local" {
		t.Fatalf("unexpected compute tls config: %#v", cfg)
	}
	if cfg.BootstrapAdminEmail != "admin@example.com" || cfg.BootstrapAdminPassword != "adminpass" {
		t.Fatalf("unexpected admin bootstrap: %#v", cfg)
	}
	if cfg.BootstrapResearcherEmail != "researcher@example.com" || cfg.BootstrapResearcherPassword != "researcherpass" {
		t.Fatalf("unexpected researcher bootstrap: %#v", cfg)
	}
}

func TestFromEnv_InvalidValuesFallback(t *testing.T) {
	t.Setenv("TOKEN_TTL", "bad")
	t.Setenv("IDEMPOTENCY_TTL", "bad")
	t.Setenv("MAX_UPLOAD_BYTES", "-1")
	t.Setenv("WORKER_POLL_INTERVAL", "bad")
	t.Setenv("COMPUTE_TLS", "0")

	cfg := FromEnv()

	if cfg.TokenTTL != 30*24*time.Hour {
		t.Fatalf("unexpected TokenTTL fallback: %v", cfg.TokenTTL)
	}
	if cfg.IdempotencyTTL != 24*time.Hour {
		t.Fatalf("unexpected IdempotencyTTL fallback: %v", cfg.IdempotencyTTL)
	}
	if cfg.MaxUploadBytes != int64(10<<20) {
		t.Fatalf("unexpected MaxUploadBytes fallback: %d", cfg.MaxUploadBytes)
	}
	if cfg.WorkerPollInterval != time.Second {
		t.Fatalf("unexpected WorkerPollInterval fallback: %v", cfg.WorkerPollInterval)
	}
	if cfg.ComputeTLS != false {
		t.Fatalf("expected ComputeTLS=false with 0")
	}
}
