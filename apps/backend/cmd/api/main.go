package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	//"time"

	"secure-voting/apps/backend/internal/config"
	"secure-voting/apps/backend/internal/httpserver"
)

// main starts the HTTP server and shuts it down gracefully.
func main() {
	cfg := config.FromEnv()

	handler := httpserver.Routes()
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

	ctx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Printf("shutdown error: %v", err)
	}

	log.Printf("bye")
}
