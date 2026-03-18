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

	log.Printf(
		"compute-runner starting: brokers=%v tasks_topic=%s results_topic=%s group=%s grpc_addr=%s tls=%v mongo_db=%s",
		cfg.Brokers,
		cfg.TasksTopic,
		cfg.ResultsTopic,
		cfg.GroupID,
		cfg.GRPCAddr,
		cfg.UseTLS,
		cfg.MongoDB,
	)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	mc, mdb, err := connectMongo(ctx, cfg)
	if err != nil {
		log.Printf("mongo connect: %v", err)
		stop()
		return err
	}
	log.Printf("mongo connected: db=%s", cfg.MongoDB)
	defer func() { _ = mc.Disconnect(context.Background()) }()

	cc, err := connectCompute(ctx, cfg)
	if err != nil {
		log.Printf("compute connect failed: %v", err)
		stop()
		return err
	}
	log.Printf("compute client connected: addr=%s tls=%v", cfg.GRPCAddr, cfg.UseTLS)
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
			log.Printf("context canceled, stopping compute-runner")
			return nil
		}

		log.Printf(
			"task message fetched: topic=%s partition=%d offset=%d key=%q bytes=%d",
			msg.Topic,
			msg.Partition,
			msg.Offset,
			string(msg.Key),
			len(msg.Value),
		)

		task, ok := decodeTask(msg)
		if !ok {
			log.Printf("bad task payload: commit and skip offset=%d", msg.Offset)
			commitTask(ctx, reader, msg)
			continue
		}

		task.RunID = strings.TrimSpace(task.RunID)
		if task.RunID == "" && len(msg.Key) > 0 {
			task.RunID = strings.TrimSpace(string(msg.Key))
		}

		log.Printf(
			"processing task: run_id=%s experiment_id=%s dataset_id=%s",
			task.RunID,
			task.ExperimentID,
			task.DatasetID,
		)

		res := processTask(ctx, mdb, cfg, cc.Compute(), task)

		log.Printf(
			"task processed: run_id=%s status=%s error=%q winners=%d",
			res.RunID,
			res.Status,
			res.ErrorText,
			len(res.Winners),
		)

		if err := writeResult(ctx, writer, res); err != nil {
			if errors.Is(err, context.Canceled) {
				log.Printf("context canceled while writing result")
				return nil
			}
			log.Printf("kafka write result: %v", err)
			time.Sleep(500 * time.Millisecond)
			continue
		}

		log.Printf("result written: run_id=%s status=%s", res.RunID, res.Status)
		commitTask(ctx, reader, msg)
		log.Printf("task committed: offset=%d run_id=%s", msg.Offset, task.RunID)
	}
}

func main() {
	if err := run(); err != nil {
		log.Printf("fatal: %v", err)
		os.Exit(1)
	}
}

var _ = kafka.FirstOffset
