package main

import (
	"context"
	"log"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.mongodb.org/mongo-driver/mongo"

	pb "secure-voting/apps/backend/internal/compute/pb"
	"secure-voting/apps/backend/internal/worker"
)

func processExperimentRunTask(ctx context.Context, mdb *mongo.Database, cfg Config, compute pb.ComputeClient, task worker.ExperimentRunTask) worker.ExperimentRunResult {
	task.RunID = strings.TrimSpace(task.RunID)
	if task.RunID == "" {
		log.Printf("processExperimentRunTask: missing run_id")
		return makeErrorResult(task.RunID, "missing run_id")
	}

	log.Printf(
		"processExperimentRunTask: start run_id=%s experiment_id=%s dataset_id=%s",
		task.RunID,
		task.ExperimentID,
		task.DatasetID,
	)

	candidates, code, err := loadDatasetCandidates(ctx, mdb, task.DatasetID)
	if err != nil {
		log.Printf("processExperimentRunTask: loadDatasetCandidates failed run_id=%s err=%v", task.RunID, err)
		return makeErrorResult(task.RunID, "load dataset candidates failed: "+err.Error())
	}
	if code != "" {
		log.Printf("processExperimentRunTask: loadDatasetCandidates code run_id=%s code=%s", task.RunID, code)
		return makeErrorResult(task.RunID, code)
	}

	task.Dataset.Candidates = candidates
	log.Printf("processExperimentRunTask: loaded dataset candidates run_id=%s count=%d", task.RunID, len(candidates))

	header, code := buildHeader(task)
	log.Printf(
		"processExperimentRunTask: built header run_id=%s ballot_format=%s tally_rule=%s candidates=%d",
		task.RunID,
		header.BallotFormat,
		header.TallyRule,
		len(header.Candidates),
	)

	if code != "" {
		log.Printf("processExperimentRunTask: buildHeader failed run_id=%s code=%s", task.RunID, code)
		return makeErrorResult(task.RunID, code)
	}

	rctx, cancel := context.WithTimeout(ctx, cfg.RunTimeout)
	defer cancel()

	stream, err := compute.Run(rctx)
	if err != nil {
		log.Printf("processExperimentRunTask: compute.Run failed run_id=%s err=%v", task.RunID, err)
		return makeErrorResult(task.RunID, "grpc run start failed: "+err.Error())
	}

	if err := stream.Send(&pb.RunChunk{Part: &pb.RunChunk_Header{Header: header}}); err != nil {
		log.Printf("processExperimentRunTask: send header failed run_id=%s err=%v", task.RunID, err)
		return makeErrorResult(task.RunID, "grpc send header failed: "+err.Error())
	}

	sent, code, err := streamDatasetBallots(rctx, mdb, cfg, task.DatasetID, header.BallotFormat, stream)
	if err != nil {
		log.Printf("processExperimentRunTask: streamDatasetBallots failed run_id=%s err=%v", task.RunID, err)
		return makeErrorResult(task.RunID, "stream ballots failed: "+err.Error())
	}
	if code != "" {
		log.Printf("processExperimentRunTask: streamDatasetBallots code run_id=%s code=%s", task.RunID, code)
		return makeErrorResult(task.RunID, code)
	}
	log.Printf("processExperimentRunTask: streamed ballots run_id=%s count=%d", task.RunID, sent)

	resp, err := stream.CloseAndRecv()
	if err != nil {
		log.Printf("processExperimentRunTask: CloseAndRecv failed run_id=%s err=%v", task.RunID, err)
		return makeErrorResult(task.RunID, "grpc close/recv failed: "+err.Error())
	}

	status, errText, winnersAny, metrics, protocol, timings, artifacts := parseRunResult(resp)

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
	return makeDoneResult(task.RunID, winners, metrics, protocol, timings, artifacts)
}

func processElectionTallyTask(ctx context.Context, db *pgxpool.Pool, cfg Config, compute pb.ComputeClient, task worker.ElectionTallyTask) worker.ElectionTallyResult {
	task.JobID = strings.TrimSpace(task.JobID)
	task.ElectionID = strings.TrimSpace(task.ElectionID)

	if task.JobID == "" {
		log.Printf("processElectionTallyTask: missing job_id")
		return makeElectionErrorResult(task, "missing job_id")
	}
	if task.ElectionID == "" {
		log.Printf("processElectionTallyTask: missing election_id")
		return makeElectionErrorResult(task, "missing election_id")
	}

	log.Printf(
		"processElectionTallyTask: start job_id=%s election_id=%s tally_rule=%s ballot_format=%s",
		task.JobID,
		task.ElectionID,
		task.TallyRule,
		task.BallotFormat,
	)

	header, code := buildElectionHeader(task)
	if code != "" {
		log.Printf("processElectionTallyTask: buildElectionHeader failed job_id=%s code=%s", task.JobID, code)
		return makeElectionErrorResult(task, code)
	}

	rctx, cancel := context.WithTimeout(ctx, cfg.RunTimeout)
	defer cancel()

	stream, err := compute.Run(rctx)
	if err != nil {
		log.Printf("processElectionTallyTask: compute.Run failed job_id=%s err=%v", task.JobID, err)
		return makeElectionErrorResult(task, "grpc run start failed: "+err.Error())
	}

	if err := stream.Send(&pb.RunChunk{Part: &pb.RunChunk_Header{Header: header}}); err != nil {
		log.Printf("processElectionTallyTask: send header failed job_id=%s err=%v", task.JobID, err)
		return makeElectionErrorResult(task, "grpc send header failed: "+err.Error())
	}

	sent, code, err := streamElectionBallots(rctx, db, cfg, task.ElectionID, header.BallotFormat, stream)
	if err != nil {
		log.Printf("processElectionTallyTask: streamElectionBallots failed job_id=%s err=%v", task.JobID, err)
		return makeElectionErrorResult(task, "stream ballots failed: "+err.Error())
	}
	if code != "" {
		log.Printf("processElectionTallyTask: streamElectionBallots code job_id=%s code=%s", task.JobID, code)
		return makeElectionErrorResult(task, code)
	}
	log.Printf("processElectionTallyTask: streamed ballots job_id=%s count=%d", task.JobID, sent)

	resp, err := stream.CloseAndRecv()
	if err != nil {
		log.Printf("processElectionTallyTask: CloseAndRecv failed job_id=%s err=%v", task.JobID, err)
		return makeElectionErrorResult(task, "grpc close/recv failed: "+err.Error())
	}

	status, errText, winnersAny, metrics, protocol, timings, artifacts := parseRunResult(resp)

	if status != "done" && status != "error" {
		status = "error"
		if strings.TrimSpace(errText) == "" {
			errText = "compute returned invalid status"
		}
	}

	if status == "error" {
		return makeElectionErrorResult(task, errText)
	}

	winners := anySliceToStringSlice(winnersAny)
	return makeElectionDoneResult(task, winners, metrics, protocol, timings, artifacts)
}
