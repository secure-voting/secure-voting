package main

import (
	"encoding/json"
	"fmt"
	"strings"

	pb "secure-voting/apps/backend/internal/compute/pb"
	"secure-voting/apps/backend/internal/worker"
)

const (
	resultKind = "experiment_run_result"
)

func anySliceToStringSlice(in []any) []string {
	if len(in) == 0 {
		return nil
	}

	out := make([]string, 0, len(in))
	for _, v := range in {
		switch x := v.(type) {
		case string:
			out = append(out, x)
		case []byte:
			out = append(out, string(x))
		default:
			out = append(out, fmt.Sprint(v))
		}
	}
	return out
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

func makeErrorResult(runID, errText string) worker.ExperimentRunResult {
	runID = strings.TrimSpace(runID)
	errText = strings.TrimSpace(errText)
	if errText == "" {
		errText = "error"
	}

	return worker.ExperimentRunResult{
		Kind:      resultKind,
		RunID:     runID,
		Status:    "error",
		ErrorText: errText,
	}
}

func makeDoneResult(runID string, winners []string, metrics, timings, artifacts map[string]any) worker.ExperimentRunResult {
	runID = strings.TrimSpace(runID)

	if metrics == nil {
		metrics = map[string]any{}
	}
	if timings == nil {
		timings = map[string]any{}
	}
	if artifacts == nil {
		artifacts = map[string]any{}
	}

	return worker.ExperimentRunResult{
		Kind:      resultKind,
		RunID:     runID,
		Status:    "done",
		ErrorText: "",
		Winners:   winners,
		Metrics:   metrics,
		Timings:   timings,
		Artifacts: artifacts,
	}
}

func makeElectionErrorResult(task worker.ElectionTallyTask, errText string) worker.ElectionTallyResult {
	errText = strings.TrimSpace(errText)
	if errText == "" {
		errText = "error"
	}

	return worker.ElectionTallyResult{
		Kind:       "election_tally_result",
		JobID:      strings.TrimSpace(task.JobID),
		ElectionID: strings.TrimSpace(task.ElectionID),
		Status:     "error",
		ErrorText:  errText,
		Method:     normalizeComputeTallyRule(task.TallyRule),
		TallyRule:  normalizeComputeTallyRule(task.TallyRule),
	}
}

func makeElectionDoneResult(task worker.ElectionTallyTask, winners []string, metrics, timings, artifacts map[string]any) worker.ElectionTallyResult {
	if metrics == nil {
		metrics = map[string]any{}
	}
	if timings == nil {
		timings = map[string]any{}
	}
	if artifacts == nil {
		artifacts = map[string]any{}
	}

	protocol := map[string]any{}
	if rawProtocol, ok := artifacts["protocol"]; ok && rawProtocol != nil {
		if p, ok := rawProtocol.(map[string]any); ok {
			protocol = p
		} else {
			protocol["raw"] = rawProtocol
		}
	}

	params := map[string]any{
		"ballot_format": "ranking",
		"tally_rule":    normalizeComputeTallyRule(task.TallyRule),
	}
	if task.CommitteeSize != nil {
		params["committee_size"] = *task.CommitteeSize
	}
	if task.QuotaType != nil {
		params["quota_type"] = *task.QuotaType
	}
	if task.RankingTopK != nil {
		params["ranking_top_k"] = *task.RankingTopK
	}
	params["show_aggregates"] = task.ShowAggregates

	return worker.ElectionTallyResult{
		Kind:       "election_tally_result",
		JobID:      strings.TrimSpace(task.JobID),
		ElectionID: strings.TrimSpace(task.ElectionID),
		Status:     "done",
		Method:     normalizeComputeTallyRule(task.TallyRule),
		TallyRule:  normalizeComputeTallyRule(task.TallyRule),
		Params:     params,
		Winners:    winners,
		Metrics:    metrics,
		Protocol:   protocol,
		Timings:    timings,
		Artifacts:  artifacts,
	}
}
