package main

import (
	"context"
	"errors"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/redis/go-redis/v9"

	"secure-voting/apps/backend/internal/config"
	"secure-voting/apps/backend/internal/db"
	"secure-voting/apps/backend/internal/systemstatus"
	"secure-voting/apps/backend/internal/worker"
)

func startWorkerHeartbeat(ctx context.Context, rdb *redis.Client, cfg config.Config) {
	const heartbeatTTL = 20 * time.Second
	const heartbeatInterval = 5 * time.Second

	_ = systemstatus.PublishWorkerHeartbeat(ctx, rdb, cfg.WorkerPollInterval, heartbeatTTL)

	ticker := time.NewTicker(heartbeatInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := systemstatus.PublishWorkerHeartbeat(ctx, rdb, cfg.WorkerPollInterval, heartbeatTTL); err != nil {
				log.Printf("worker heartbeat publish failed: %v", err)
			}
		}
	}
}

func run() error {
	cfg := config.FromEnv()

	bootCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pg, err := db.NewPostgresPool(bootCtx, cfg.PostgresDSN)
	if err != nil {
		log.Printf("failed to init postgres: %v", err)
		cancel()
		return err
	}
	defer pg.Close()

	rdb, err := db.NewRedisClient(
		bootCtx,
		cfg.RedisAddr,
		cfg.RedisPassword,
		cfg.RedisTLS,
		cfg.RedisTLSCA,
		cfg.RedisTLSServerName,
	)
	if err != nil {
		log.Printf("failed to init redis: %v", err)
		cancel()
		return err
	}
	defer func() { _ = rdb.Close() }()

	mc, err := db.NewMongoClient(bootCtx, cfg.MongoURI)
	if err != nil {
		log.Printf("failed to init mongo: %v", err)
		cancel()
		return err
	}
	defer func() { _ = mc.Disconnect(context.Background()) }()

	mdb := mc.Database(cfg.MongoDBName)

	w := worker.New(pg, mdb, worker.Config{
		PollInterval:     cfg.WorkerPollInterval,
		ScheduleInterval: cfg.WorkerScheduleInterval,
		TasksTopic:       cfg.KafkaTasksTopic,
		ResultsTopic:     cfg.KafkaResultsTopic,
		GroupID:          cfg.KafkaGroupID,
		Brokers:          cfg.KafkaBrokers,

		KafkaTLS:           cfg.KafkaTLS,
		KafkaTLSCA:         cfg.KafkaTLSCA,
		KafkaTLSServerName: cfg.KafkaTLSServerName,
	})
	defer w.Close()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)

	ctx, cancelRun := context.WithCancel(context.Background())
	defer cancelRun()

	go startWorkerHeartbeat(ctx, rdb, cfg)

	errCh := make(chan error, 1)
	go func() {
		log.Printf(
			"worker started: poll=%s schedule=%s brokers=%v tasks=%s results=%s group=%s",
			cfg.WorkerPollInterval,
			cfg.WorkerScheduleInterval,
			cfg.KafkaBrokers,
			cfg.KafkaTasksTopic,
			cfg.KafkaResultsTopic,
			cfg.KafkaGroupID,
		)
		errCh <- w.Run(ctx)
	}()

	select {
	case <-stop:
		log.Printf("shutdown signal received")
		cancelRun()
	case err := <-errCh:
		if err != nil && !errors.Is(err, context.Canceled) {
			log.Printf("worker stopped with error: %v", err)
		}
		cancelRun()
	}

	log.Printf("bye")
	return nil
}

func main() {
	if err := run(); err != nil {
		log.Printf("fatal: %v", err)
		os.Exit(1)
	}
}
