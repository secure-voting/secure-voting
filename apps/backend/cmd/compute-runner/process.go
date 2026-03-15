package main

import (
	"context"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/mongo"

	pb "secure-voting/apps/backend/internal/compute/pb"
	"secure-voting/apps/backend/internal/worker"
)

func processTask(ctx context.Context, mdb *mongo.Database, cfg Config, compute pb.ComputeClient, task worker.ExperimentRunTask) worker.ExperimentRunResult {
	task.RunID = strings.TrimSpace(task.RunID)
	if task.RunID == "" {
		return makeErrorResult(task.RunID, "missing run_id")
	}

	header, code := buildHeader(task)
	if code != "" {
		return makeErrorResult(task.RunID, code)
	}

	rctx, cancel := context.WithTimeout(ctx, cfg.RunTimeout)
	defer cancel()

	stream, err := compute.Run(rctx)
	if err != nil {
		return makeErrorResult(task.RunID, "grpc run start failed: "+err.Error())
	}

	if err := stream.Send(&pb.RunChunk{Part: &pb.RunChunk_Header{Header: header}}); err != nil {
		return makeErrorResult(task.RunID, "grpc send header failed: "+err.Error())
	}

	_, code, err = streamRankingBallots(rctx, mdb, cfg, task.DatasetID, stream)
	if err != nil {
		return makeErrorResult(task.RunID, "stream ballots failed: "+err.Error())
	}
	if code != "" {
		return makeErrorResult(task.RunID, code)
	}

	resp, err := stream.CloseAndRecv()
	if err != nil {
		return makeErrorResult(task.RunID, "grpc close/recv failed: "+err.Error())
	}

	status, errText, winnersAny, metrics, timings, artifacts := parseRunResult(resp)

	if status != "done" && status != "error" {
		status = "error"
		if strings.TrimSpace(errText) == "" {
			errText = "compute returned invalid status"
		}
	}

	if status == "error" {
		return makeErrorResult(task.RunID, errText)
	}

	winners := anySliceToStringSlice(winnersAny)

	_ = time.Now // чтобы goimports не трогал time при локальных правках (можно убрать, если не нужно)

	return makeDoneResult(task.RunID, winners, metrics, timings, artifacts)
}
