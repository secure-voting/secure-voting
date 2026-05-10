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

	PostgresDSN     string
	TokenTTL        time.Duration
	RefreshTokenTTL time.Duration

	RedisAddr      string
	RedisPassword  string
	IdempotencyTTL time.Duration

	AdminTrustedCIDRs  []string
	RedisTLS           bool
	RedisTLSCA         string
	RedisTLSServerName string

	MongoURI    string
	MongoDBName string

	MaxUploadBytes int64

	KafkaBrokers           []string
	KafkaTasksTopic        string
	KafkaResultsTopic      string
	KafkaGroupID           string
	WorkerPollInterval     time.Duration
	WorkerScheduleInterval time.Duration
	KafkaTLS               bool
	KafkaTLSCA             string
	KafkaTLSServerName     string

	ComputeGRPCAddr      string
	ComputeTLS           bool
	ComputeTLSCA         string
	ComputeTLSServerName string

	BootstrapAdminEmail         string
	BootstrapAdminPassword      string
	BootstrapResearcherEmail    string
	BootstrapResearcherPassword string

	AuthRateLimit    int
	AuthRateLimitTTL time.Duration

	WriteRateLimit    int
	WriteRateLimitTTL time.Duration

	EmailVerificationMode string
	SMTPHost              string
	SMTPPort              int
	SMTPUsername          string
	SMTPPassword          string
	SMTPFromEmail         string
	SMTPFromName          string
	SMTPTLSMode           string
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

	tokenTTL := 15 * time.Minute
	if ttlStr := os.Getenv("TOKEN_TTL"); ttlStr != "" {
		if parsed, err := time.ParseDuration(ttlStr); err == nil {
			tokenTTL = parsed
		}
	}

	refreshTokenTTL := 30 * 24 * time.Hour
	if ttlStr := os.Getenv("REFRESH_TOKEN_TTL"); ttlStr != "" {
		if parsed, err := time.ParseDuration(ttlStr); err == nil {
			refreshTokenTTL = parsed
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

	redisTLS := false
	if s := strings.TrimSpace(os.Getenv("REDIS_TLS")); s != "" {
		if s == "1" || strings.EqualFold(s, "true") {
			redisTLS = true
		}
	}

	redisTLSCA := strings.TrimSpace(os.Getenv("REDIS_TLS_CA"))
	if redisTLS && redisTLSCA == "" {
		redisTLSCA = "/certs/ca.pem"
	}

	redisTLSServerName := strings.TrimSpace(os.Getenv("REDIS_TLS_SERVER_NAME"))
	if redisTLS && redisTLSServerName == "" {
		redisTLSServerName = "cache"
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

	maxUpload := int64(10 << 20)
	if s := os.Getenv("MAX_UPLOAD_BYTES"); s != "" {
		if v, err := strconv.ParseInt(s, 10, 64); err == nil && v > 0 {
			maxUpload = v
		}
	}

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

	kafkaTLS := false
	if s := strings.TrimSpace(os.Getenv("KAFKA_TLS")); s != "" {
		if s == "1" || strings.EqualFold(s, "true") {
			kafkaTLS = true
		}
	}

	kafkaTLSCA := strings.TrimSpace(os.Getenv("KAFKA_TLS_CA"))
	if kafkaTLS && kafkaTLSCA == "" {
		kafkaTLSCA = "/certs/ca.pem"
	}

	kafkaTLSServerName := strings.TrimSpace(os.Getenv("KAFKA_TLS_SERVER_NAME"))
	if kafkaTLS && kafkaTLSServerName == "" {
		kafkaTLSServerName = "kafka"
	}

	poll := 1 * time.Second
	if s := os.Getenv("WORKER_POLL_INTERVAL"); s != "" {
		if d, err := time.ParseDuration(s); err == nil && d > 0 {
			poll = d
		}
	}

	schedulePoll := 5 * time.Second
	if s := os.Getenv("WORKER_SCHEDULE_INTERVAL"); s != "" {
		if d, err := time.ParseDuration(s); err == nil && d > 0 {
			schedulePoll = d
		}
	}

	computeDisabled := false
	if s := strings.TrimSpace(os.Getenv("DISABLE_COMPUTE")); s != "" {
		if s == "1" || strings.EqualFold(s, "true") {
			computeDisabled = true
		}
	}
	if strings.TrimSpace(os.Getenv("SECURE_VOTING_INTEGRATION")) == "1" {
		computeDisabled = true
	}

	computeAddr := os.Getenv("COMPUTE_GRPC_ADDR")
	if computeAddr == "" && !computeDisabled {
		computeAddr = "rust-compute:50051"
	}

	computeTLS := true
	if s := strings.TrimSpace(os.Getenv("COMPUTE_TLS")); s != "" {
		if s == "0" || strings.EqualFold(s, "false") {
			computeTLS = false
		}
	}
	if computeDisabled {
		computeTLS = false
	}

	computeCA := os.Getenv("COMPUTE_TLS_CA")
	if computeCA == "" && !computeDisabled {
		computeCA = "/certs/ca.pem"
	}
	if computeDisabled {
		computeCA = ""
	}

	computeSN := os.Getenv("COMPUTE_TLS_SERVER_NAME")
	if computeSN == "" && !computeDisabled {
		computeSN = "rust-compute"
	}
	if computeDisabled {
		computeSN = ""
	}

	bootstrapAdminEmail := strings.TrimSpace(os.Getenv("BOOTSTRAP_ADMIN_EMAIL"))
	bootstrapAdminPassword := strings.TrimSpace(os.Getenv("BOOTSTRAP_ADMIN_PASSWORD"))
	bootstrapResearcherEmail := strings.TrimSpace(os.Getenv("BOOTSTRAP_RESEARCHER_EMAIL"))
	bootstrapResearcherPassword := strings.TrimSpace(os.Getenv("BOOTSTRAP_RESEARCHER_PASSWORD"))

	authRateLimit := 10
	if s := os.Getenv("AUTH_RATE_LIMIT"); s != "" {
		if v, err := strconv.Atoi(strings.TrimSpace(s)); err == nil && v > 0 {
			authRateLimit = v
		}
	}

	authRateLimitTTL := time.Minute
	if s := os.Getenv("AUTH_RATE_LIMIT_TTL"); s != "" {
		if d, err := time.ParseDuration(strings.TrimSpace(s)); err == nil && d > 0 {
			authRateLimitTTL = d
		}
	}

	writeRateLimit := 30
	if s := os.Getenv("WRITE_RATE_LIMIT"); s != "" {
		if v, err := strconv.Atoi(strings.TrimSpace(s)); err == nil && v > 0 {
			writeRateLimit = v
		}
	}

	writeRateLimitTTL := time.Minute
	if s := os.Getenv("WRITE_RATE_LIMIT_TTL"); s != "" {
		if d, err := time.ParseDuration(strings.TrimSpace(s)); err == nil && d > 0 {
			writeRateLimitTTL = d
		}
	}

	emailVerificationMode := strings.ToLower(strings.TrimSpace(os.Getenv("EMAIL_VERIFICATION_MODE")))
	if emailVerificationMode == "" {
		emailVerificationMode = "dev"
	}

	smtpPort := 587
	if s := strings.TrimSpace(os.Getenv("SMTP_PORT")); s != "" {
		if v, err := strconv.Atoi(s); err == nil && v > 0 {
			smtpPort = v
		}
	}

	smtpTLSMode := strings.ToLower(strings.TrimSpace(os.Getenv("SMTP_TLS_MODE")))
	if smtpTLSMode == "" {
		smtpTLSMode = "starttls"
	}

	return Config{
		HTTPAddr:        addr,
		ShutdownTimeout: 10 * time.Second,

		PostgresDSN:     dsn,
		TokenTTL:        tokenTTL,
		RefreshTokenTTL: refreshTokenTTL,

		RedisAddr:          redisAddr,
		RedisPassword:      redisPass,
		RedisTLS:           redisTLS,
		RedisTLSCA:         redisTLSCA,
		RedisTLSServerName: redisTLSServerName,
		IdempotencyTTL:     idemTTL,

		MongoURI:    mongoURI,
		MongoDBName: mongoDB,

		MaxUploadBytes: maxUpload,

		KafkaBrokers:           brokers,
		KafkaTasksTopic:        tasksTopic,
		KafkaResultsTopic:      resultsTopic,
		KafkaGroupID:           groupID,
		WorkerPollInterval:     poll,
		WorkerScheduleInterval: schedulePoll,
		KafkaTLS:               kafkaTLS,
		KafkaTLSCA:             kafkaTLSCA,
		KafkaTLSServerName:     kafkaTLSServerName,

		ComputeGRPCAddr:      computeAddr,
		ComputeTLS:           computeTLS,
		ComputeTLSCA:         computeCA,
		ComputeTLSServerName: computeSN,

		BootstrapAdminEmail:         bootstrapAdminEmail,
		BootstrapAdminPassword:      bootstrapAdminPassword,
		BootstrapResearcherEmail:    bootstrapResearcherEmail,
		BootstrapResearcherPassword: bootstrapResearcherPassword,

		AuthRateLimit:    authRateLimit,
		AuthRateLimitTTL: authRateLimitTTL,

		WriteRateLimit:    writeRateLimit,
		WriteRateLimitTTL: writeRateLimitTTL,

		EmailVerificationMode: emailVerificationMode,
		SMTPHost:              strings.TrimSpace(os.Getenv("SMTP_HOST")),
		SMTPPort:              smtpPort,
		SMTPUsername:          strings.TrimSpace(os.Getenv("SMTP_USERNAME")),
		SMTPPassword:          strings.TrimSpace(os.Getenv("SMTP_PASSWORD")),
		SMTPFromEmail:         strings.TrimSpace(os.Getenv("SMTP_FROM_EMAIL")),
		SMTPFromName:          strings.TrimSpace(os.Getenv("SMTP_FROM_NAME")),
		SMTPTLSMode:           smtpTLSMode,

		AdminTrustedCIDRs: splitCSV(os.Getenv("ADMIN_TRUSTED_CIDRS")),
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
