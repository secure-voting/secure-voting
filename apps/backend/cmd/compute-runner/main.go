package main

import (
	"context"
	"errors"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/segmentio/kafka-go"
)

func run() error {
	cfg := loadConfig()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	mc, mdb, err := connectMongo(ctx, cfg)
	if err != nil {
		log.Printf("mongo connect: %v", err)
		stop()
		return err
	}
	defer func() { _ = mc.Disconnect(context.Background()) }()

	cc, err := connectCompute(ctx, cfg)
	if err != nil {
		stop()
		return err
	}
	defer func() { _ = cc.Close() }()

	reader := newTaskReader(cfg)
	defer func() { _ = reader.Close() }()

	writer := newResultWriter(cfg)
	defer func() { _ = writer.Close() }()

	for {
		msg, ok, err := fetchTaskMessage(ctx, reader)
		if err != nil {
			log.Printf("kafka fetch: %v", err)
			time.Sleep(250 * time.Millisecond)
			continue
		}
		if !ok {
			return nil
		}

		task, ok := decodeTask(msg)
		if !ok {
			commitTask(ctx, reader, msg)
			continue
		}

		task.RunID = strings.TrimSpace(task.RunID)
		if task.RunID == "" && len(msg.Key) > 0 {
			task.RunID = strings.TrimSpace(string(msg.Key))
		}

		res := processTask(ctx, mdb, cfg, cc.Compute(), task)

		if err := writeResult(ctx, writer, res); err != nil {
			if errors.Is(err, context.Canceled) {
				return nil
			}
			log.Printf("kafka write result: %v", err)
			time.Sleep(500 * time.Millisecond)
			continue
		}

		commitTask(ctx, reader, msg)
	}
}

func main() {
	if err := run(); err != nil {
		log.Printf("fatal: %v", err)
		os.Exit(1)
	}
}

var _ = kafka.FirstOffset
