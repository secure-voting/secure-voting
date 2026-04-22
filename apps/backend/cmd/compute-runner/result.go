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

func parseRunResult(resp *pb.RunResult) (string, string, []any, map[string]any, any, map[string]any, map[string]any) {
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

	var protocol any
	if len(resp.GetProtocolJson()) > 0 {
		_ = json.Unmarshal(resp.GetProtocolJson(), &protocol)
	}

	var timings map[string]any
	if len(resp.GetTimingsJson()) > 0 {
		_ = json.Unmarshal(resp.GetTimingsJson(), &timings)
	}

	var artifacts map[string]any
	if len(resp.GetArtifactsJson()) > 0 {
		_ = json.Unmarshal(resp.GetArtifactsJson(), &artifacts)
	}

	return status, errText, winners, metrics, protocol, timings, artifacts
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

	var approvalMax *int32
	if task.ApprovalMaxChoices != nil {
		v := int32(*task.ApprovalMaxChoices)
		approvalMax = &v
	}
	method := resolveComputeTallyRule(task.TallyRule, approvalMax)

	return worker.ElectionTallyResult{
		Kind:       "election_tally_result",
		JobID:      strings.TrimSpace(task.JobID),
		ElectionID: strings.TrimSpace(task.ElectionID),
		Status:     "error",
		ErrorText:  errText,
		Method:     method,
		TallyRule:  method,
	}
}

func makeElectionDoneResult(
	task worker.ElectionTallyTask,
	winners []string,
	metrics map[string]any,
	protocol any,
	timings map[string]any,
	artifacts map[string]any,
) worker.ElectionTallyResult {
	var approvalMax *int32
	if task.ApprovalMaxChoices != nil {
		v := int32(*task.ApprovalMaxChoices)
		approvalMax = &v
	}
	method := resolveComputeTallyRule(task.TallyRule, approvalMax)

	params := map[string]any{
		"ballot_format": normalizeBallotFormat(task.BallotFormat),
		"tally_rule":    method,
	}
	if task.CommitteeSize != nil {
		params["committee_size"] = *task.CommitteeSize
	}
	if task.QuotaType != nil {
		params["quota_type"] = *task.QuotaType
	}
	if task.ApprovalMaxChoices != nil {
		params["approval_max_choices"] = *task.ApprovalMaxChoices
	}
	if task.RankingTopK != nil {
		params["ranking_top_k"] = *task.RankingTopK
	}
	if task.ScoreMin != nil {
		params["score_min"] = *task.ScoreMin
	}
	if task.ScoreMax != nil {
		params["score_max"] = *task.ScoreMax
	}
	if task.ScoreStep != nil {
		params["score_step"] = *task.ScoreStep
	}
	params["score_allow_skip"] = task.ScoreAllowSkip
	params["show_aggregates"] = task.ShowAggregates

	return worker.ElectionTallyResult{
		Kind:       "election_tally_result",
		JobID:      strings.TrimSpace(task.JobID),
		ElectionID: strings.TrimSpace(task.ElectionID),
		Status:     "done",
		Method:     method,
		TallyRule:  method,
		Params:     params,
		Winners:    winners,
		Metrics:    metrics,
		Protocol:   protocol,
		Timings:    timings,
		Artifacts:  artifacts,
	}
}