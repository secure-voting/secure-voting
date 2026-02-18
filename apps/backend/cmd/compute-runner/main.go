package main

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/segmentio/kafka-go"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"google.golang.org/protobuf/types/known/wrapperspb"

	pb "secure-voting/apps/backend/internal/compute/pb"
	"secure-voting/apps/backend/internal/computeclient"
	"secure-voting/apps/backend/internal/worker"
)

type ballotDoc struct {
	VoterRef string   `bson:"voter_ref"`
	Ranking  []string `bson:"ranking"`
}

func env(key string) string {
	return strings.TrimSpace(os.Getenv(key))
}

func envOr(key, def string) string {
	v := env(key)
	if v == "" {
		return def
	}
	return v
}

func mustEnv(key string) string {
	v := env(key)
	if v == "" {
		log.Fatalf("missing env %s", key)
	}
	return v
}

func splitCSV(s string) []string {
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

func parseBool(s string) bool {
	s = strings.ToLower(strings.TrimSpace(s))
	return s == "1" || s == "true" || s == "yes" || s == "y" || s == "on"
}

func getString(m map[string]any, key string) string {
	v, ok := m[key]
	if !ok {
		return ""
	}
	s, ok := v.(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(s)
}

func getInt32(m map[string]any, key string) (int32, bool) {
	v, ok := m[key]
	if !ok {
		return 0, false
	}
	switch t := v.(type) {
	case float64:
		if t == 0 {
			return 0, true
		}
		return int32(t), true
	case string:
		n, err := strconv.Atoi(strings.TrimSpace(t))
		if err != nil {
			return 0, false
		}
		return int32(n), true
	default:
		return 0, false
	}
}

func parseParams(raw json.RawMessage) map[string]any {
	if len(raw) == 0 || string(raw) == "null" {
		return map[string]any{}
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		return map[string]any{}
	}
	return m
}

func buildHeader(task worker.ExperimentRunTask) (*pb.RunHeader, string) {
	params := parseParams(task.ExperimentParams)

	ballotFormat := getString(params, "ballot_format")
	if ballotFormat == "" {
		ballotFormat = strings.TrimSpace(task.Dataset.Format)
	}
	if ballotFormat == "" {
		ballotFormat = "ranking"
	}

	tallyRule := getString(params, "tally_rule")
	if tallyRule == "" {
		tallyRule = getString(params, "rule")
	}
	if tallyRule == "" {
		tallyRule = "plurality"
	}

	h := &pb.RunHeader{
		Kind:         strings.TrimSpace(task.Kind),
		RunId:        strings.TrimSpace(task.RunID),
		ExperimentId: strings.TrimSpace(task.ExperimentID),
		DatasetId:    strings.TrimSpace(task.DatasetID),
		TallyRule:    tallyRule,
		BallotFormat: ballotFormat,
		ParamsJson:   []byte(task.ExperimentParams),
	}

	if n, ok := getInt32(params, "committee_size"); ok {
		h.CommitteeSize = wrapperspb.Int32(n)
	}
	if s := getString(params, "quota_type"); s != "" {
		h.QuotaType = wrapperspb.String(s)
	}
	if b, ok := params["show_aggregates"].(bool); ok {
		h.ShowAggregates = wrapperspb.Bool(b)
	}

	for _, c := range task.Dataset.Candidates {
		h.Candidates = append(h.Candidates, &pb.Candidate{Id: c.ID, Name: c.Name})
	}

	if h.BallotFormat != "ranking" {
		return nil, "unsupported ballot_format (only ranking supported now)"
	}
	return h, ""
}

func streamRankingBallots(ctx context.Context, mdb *mongo.Database, datasetHex string, stream pb.Compute_RunClient) (int, string, error) {
	oid, err := primitive.ObjectIDFromHex(strings.TrimSpace(datasetHex))
	if err != nil {
		return 0, "invalid_dataset_id", nil
	}

	coll := mdb.Collection("dataset_ballots")
	cur, err := coll.Find(
		ctx,
		bson.M{"dataset_id": oid},
		options.Find().
			SetBatchSize(500).
			SetProjection(bson.M{"voter_ref": 1, "ranking": 1}),
	)
	if err != nil {
		return 0, "", err
	}
	defer func() { _ = cur.Close(ctx) }()

	batch := make([]*pb.Ballot, 0, 500)
	total := 0

	flush := func() error {
		if len(batch) == 0 {
			return nil
		}
		err := stream.Send(&pb.RunChunk{
			Part: &pb.RunChunk_Batch{
				Batch: &pb.BallotBatch{Ballots: batch},
			},
		})
		batch = batch[:0]
		return err
	}

	for cur.Next(ctx) {
		var d ballotDoc
		if err := cur.Decode(&d); err != nil {
			return total, "", err
		}
		if len(d.Ranking) == 0 {
			continue
		}

		b := &pb.Ballot{
			VoterRef: d.VoterRef,
			Payload: &pb.Ballot_Ranking{
				Ranking: &pb.RankingBallot{Ranking: d.Ranking},
			},
		}
		batch = append(batch, b)
		total++

		if len(batch) >= 500 {
			if err := flush(); err != nil {
				return total, "", err
			}
		}
	}

	if err := cur.Err(); err != nil {
		return total, "", err
	}

	if err := flush(); err != nil {
		return total, "", err
	}

	if total == 0 {
		return 0, "empty_dataset_ballots", nil
	}

	return total, "", nil
}

func parseRunResult(resp *pb.RunResult) (string, string, []any, map[string]any, map[string]any, map[string]any) {
	status := strings.TrimSpace(resp.GetStatus())
	errText := strings.TrimSpace(resp.GetErrorText())

	var winners []any
	if len(resp.GetWinnersJson()) > 0 {
		_ = json.Unmarshal(resp.GetWinnersJson(), &winners)
	}

	var metrics map[string]any
	if len(resp.GetMetricsJson()) > 0 {
		_ = json.Unmarshal(resp.GetMetricsJson(), &metrics)
	}
	if metrics == nil {
		metrics = map[string]any{}
	}

	var timings map[string]any
	if len(resp.GetTimingsJson()) > 0 {
		_ = json.Unmarshal(resp.GetTimingsJson(), &timings)
	}
	if timings == nil {
		timings = map[string]any{}
	}

	artifacts := map[string]any{}
	if len(resp.GetArtifactsJson()) > 0 {
		var a map[string]any
		if err := json.Unmarshal(resp.GetArtifactsJson(), &a); err == nil && a != nil {
			for k, v := range a {
				artifacts[k] = v
			}
		}
	}
	if len(resp.GetProtocolJson()) > 0 {
		var p any
		if err := json.Unmarshal(resp.GetProtocolJson(), &p); err == nil {
			artifacts["protocol"] = p
		}
	}

	return status, errText, winners, metrics, timings, artifacts
}

func processTask(ctx context.Context, mdb *mongo.Database, compute pb.ComputeClient, task worker.ExperimentRunTask) worker.ExperimentRunResult {
	task.RunID = strings.TrimSpace(task.RunID)
	if task.RunID == "" {
		return worker.ExperimentRunResult{
			Kind:      "experiment_run_result",
			RunID:     task.RunID,
			Status:    "error",
			ErrorText: "missing run_id",
		}
	}

	header, code := buildHeader(task)
	if code != "" {
		return worker.ExperimentRunResult{
			Kind:      "experiment_run_result",
			RunID:     task.RunID,
			Status:    "error",
			ErrorText: code,
		}
	}

	rctx, cancel := context.WithTimeout(ctx, 120*time.Second)
	defer cancel()

	stream, err := compute.Run(rctx)
	if err != nil {
		return worker.ExperimentRunResult{
			Kind:      "experiment_run_result",
			RunID:     task.RunID,
			Status:    "error",
			ErrorText: "grpc run start failed: " + err.Error(),
		}
	}

	if err := stream.Send(&pb.RunChunk{Part: &pb.RunChunk_Header{Header: header}}); err != nil {
		return worker.ExperimentRunResult{
			Kind:      "experiment_run_result",
			RunID:     task.RunID,
			Status:    "error",
			ErrorText: "grpc send header failed: " + err.Error(),
		}
	}

	_, code, err = streamRankingBallots(rctx, mdb, task.DatasetID, stream)
	if err != nil {
		return worker.ExperimentRunResult{
			Kind:      "experiment_run_result",
			RunID:     task.RunID,
			Status:    "error",
			ErrorText: "stream ballots failed: " + err.Error(),
		}
	}
	if code != "" {
		return worker.ExperimentRunResult{
			Kind:      "experiment_run_result",
			RunID:     task.RunID,
			Status:    "error",
			ErrorText: code,
		}
	}

	resp, err := stream.CloseAndRecv()
	if err != nil {
		return worker.ExperimentRunResult{
			Kind:      "experiment_run_result",
			RunID:     task.RunID,
			Status:    "error",
			ErrorText: "grpc close/recv failed: " + err.Error(),
		}
	}

	status, errText, winners, metrics, timings, artifacts := parseRunResult(resp)

	if status != "done" && status != "error" {
		status = "error"
		if errText == "" {
			errText = "compute returned invalid status"
		}
	}

	return worker.ExperimentRunResult{
		Kind:      "experiment_run_result",
		RunID:     task.RunID,
		Status:    status,
		ErrorText: errText,
		Winners:   winners,
		Metrics:   metrics,
		Timings:   timings,
		Artifacts: artifacts,
	}
}

func run() error {
	brokers := splitCSV(envOr("KAFKA_BROKERS", "kafka:9092"))
	tasksTopic := envOr("KAFKA_TASKS_TOPIC", "secure-voting.compute.tasks")
	resultsTopic := envOr("KAFKA_RESULTS_TOPIC", "secure-voting.compute.results")
	groupID := envOr("KAFKA_GROUP_ID", "secure-voting-compute-runner")

	mongoURI := mustEnv("MONGO_URI")
	mongoDB := envOr("MONGO_DB", "secure_voting")

	grpcAddr := envOr("COMPUTE_GRPC_ADDR", "rust-compute:50051")
	useTLS := parseBool(envOr("COMPUTE_TLS", "false"))
	caPath := env("COMPUTE_TLS_CA")
	serverName := env("COMPUTE_TLS_SERVER_NAME")

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	mc, err := mongo.Connect(ctx, options.Client().ApplyURI(mongoURI))
	if err != nil {
		log.Printf("mongo connect: %v", err)
		stop()
		return err
	}
	defer func() { _ = mc.Disconnect(context.Background()) }()
	mdb := mc.Database(mongoDB)

	cc, err := computeclient.New(ctx, computeclient.Config{
		Addr:       grpcAddr,
		UseTLS:     useTLS,
		CACertPath: caPath,
		ServerName: serverName,
	})
	if err != nil {
		log.Printf("grpc dial: %v", err)
		stop()
		return err
	}
	defer func() { _ = cc.Close() }()

	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:     brokers,
		GroupID:     groupID,
		Topic:       tasksTopic,
		MinBytes:    1e3,
		MaxBytes:    10e6,
		MaxWait:     250 * time.Millisecond,
		StartOffset: kafka.FirstOffset,
	})
	defer func() { _ = reader.Close() }()

	writer := &kafka.Writer{
		Addr:         kafka.TCP(brokers...),
		Topic:        resultsTopic,
		RequiredAcks: kafka.RequireAll,
		Balancer:     &kafka.LeastBytes{},
		BatchTimeout: 50 * time.Millisecond,
	}
	defer func() { _ = writer.Close() }()

	for {
		msg, err := reader.FetchMessage(ctx)
		if err != nil {
			if errors.Is(err, context.Canceled) {
				return nil
			}
			log.Printf("kafka fetch: %v", err)
			continue
		}

		var task worker.ExperimentRunTask
		if err := json.Unmarshal(msg.Value, &task); err != nil {
			log.Printf("bad task json: %v", err)
			_ = reader.CommitMessages(ctx, msg)
			continue
		}

		res := processTask(ctx, mdb, cc.Compute(), task)
		out, _ := json.Marshal(res)

		if err := writer.WriteMessages(ctx, kafka.Message{
			Key:   []byte(res.RunID),
			Value: out,
			Time:  time.Now().UTC(),
		}); err != nil {
			log.Printf("kafka write result: %v", err)
			time.Sleep(500 * time.Millisecond)
			continue
		}

		_ = reader.CommitMessages(ctx, msg)
	}
}

func main() {
	if err := run(); err != nil {
		log.Printf("fatal: %v", err)
		os.Exit(1)
	}
}
