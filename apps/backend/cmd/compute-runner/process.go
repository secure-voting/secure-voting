package main

import (
	"context"
	"log"
	"strings"

	"go.mongodb.org/mongo-driver/mongo"

	pb "secure-voting/apps/backend/internal/compute/pb"
	"secure-voting/apps/backend/internal/worker"
)

func processTask(ctx context.Context, mdb *mongo.Database, cfg Config, compute pb.ComputeClient, task worker.ExperimentRunTask) worker.ExperimentRunResult {
	task.RunID = strings.TrimSpace(task.RunID)
	if task.RunID == "" {
		log.Printf("processTask: missing run_id")
		return makeErrorResult(task.RunID, "missing run_id")
	}

	log.Printf(
		"processTask: start run_id=%s experiment_id=%s dataset_id=%s",
		task.RunID,
		task.ExperimentID,
		task.DatasetID,
	)

	header, code := buildHeader(task)
	if code != "" {
		log.Printf("processTask: buildHeader failed run_id=%s code=%s", task.RunID, code)
		return makeErrorResult(task.RunID, code)
	}

	log.Printf(
		"processTask: header built run_id=%s tally_rule=%s ballot_format=%s",
		task.RunID,
		header.GetTallyRule(),
		header.GetBallotFormat(),
	)

	rctx, cancel := context.WithTimeout(ctx, cfg.RunTimeout)
	defer cancel()

	stream, err := compute.Run(rctx)
	if err != nil {
		log.Printf("processTask: compute.Run failed run_id=%s err=%v", task.RunID, err)
		return makeErrorResult(task.RunID, "grpc run start failed: "+err.Error())
	}
	log.Printf("processTask: grpc stream opened run_id=%s", task.RunID)

	if err := stream.Send(&pb.RunChunk{Part: &pb.RunChunk_Header{Header: header}}); err != nil {
		log.Printf("processTask: send header failed run_id=%s err=%v", task.RunID, err)
		return makeErrorResult(task.RunID, "grpc send header failed: "+err.Error())
	}
	log.Printf("processTask: header sent run_id=%s", task.RunID)

	sent, code, err := streamRankingBallots(rctx, mdb, cfg, task.DatasetID, stream)
	if err != nil {
		log.Printf("processTask: streamRankingBallots failed run_id=%s err=%v", task.RunID, err)
		return makeErrorResult(task.RunID, "stream ballots failed: "+err.Error())
	}
	if code != "" {
		log.Printf("processTask: streamRankingBallots code run_id=%s code=%s", task.RunID, code)
		return makeErrorResult(task.RunID, code)
	}
	log.Printf("processTask: streamed ballots run_id=%s count=%d", task.RunID, sent)

	log.Printf("processTask: waiting CloseAndRecv run_id=%s", task.RunID)
	resp, err := stream.CloseAndRecv()
	if err != nil {
		log.Printf("processTask: CloseAndRecv failed run_id=%s err=%v", task.RunID, err)
		return makeErrorResult(task.RunID, "grpc close/recv failed: "+err.Error())
	}

	log.Printf("processTask: response received run_id=%s status=%s", task.RunID, strings.TrimSpace(resp.GetStatus()))

	status, errText, winnersAny, metrics, timings, artifacts := parseRunResult(resp)

	if status != "done" && status != "error" {
		status = "error"
		if strings.TrimSpace(errText) == "" {
			errText = "compute returned invalid status"
		}
		log.Printf("processTask: invalid status normalized to error run_id=%s err=%q", task.RunID, errText)
	}

	if status == "error" {
		log.Printf("processTask: compute returned error run_id=%s err=%q", task.RunID, errText)
		return makeErrorResult(task.RunID, errText)
	}

	winners := anySliceToStringSlice(winnersAny)
	log.Printf("processTask: done run_id=%s winners=%v", task.RunID, winners)

	return makeDoneResult(task.RunID, winners, metrics, timings, artifacts)
}
