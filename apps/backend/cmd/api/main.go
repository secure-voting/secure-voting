package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"secure-voting/apps/backend/internal/config"
	"secure-voting/apps/backend/internal/db"
	"secure-voting/apps/backend/internal/httpserver"
)

func main() {
	cfg := config.FromEnv()

	bootCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pg, err := db.NewPostgresPool(bootCtx, cfg.PostgresDSN)
	if err != nil {
		log.Fatalf("failed to init postgres: %v", err)
	}
	defer pg.Close()

	rdb, err := db.NewRedisClient(bootCtx, cfg.RedisAddr, cfg.RedisPassword)
	if err != nil {
		log.Fatalf("failed to init redis: %v", err)
	}
	defer func() { _ = rdb.Close() }()

	mc, err := db.NewMongoClient(bootCtx, cfg.MongoURI)
	if err != nil {
		log.Fatalf("failed to init mongo: %v", err)
	}
	defer func() { _ = mc.Disconnect(context.Background()) }()

	mdb := mc.Database(cfg.MongoDBName)

	handler := httpserver.Routes(cfg, pg, rdb, mdb)
	srv := httpserver.New(cfg.HTTPAddr, handler)

	errCh := make(chan error, 1)
	go func() {
		log.Printf("server listening on %s", cfg.HTTPAddr)
		errCh <- srv.Run()
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)

	select {
	case <-stop:
		log.Printf("shutdown signal received")
	case err := <-errCh:
		if err != nil {
			log.Printf("server stopped with error: %v", err)
		}
	}

	ctx, cancel2 := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel2()

	if err := srv.Shutdown(ctx); err != nil {
		log.Printf("shutdown error: %v", err)
	}

	log.Printf("bye")
}
