package main

import (
	"encoding/json"
	"strings"

	"google.golang.org/protobuf/types/known/wrapperspb"

	pb "secure-voting/apps/backend/internal/compute/pb"
	"secure-voting/apps/backend/internal/worker"
)

func grpcTallyRuleName(rule string) string {
	if rule == "minmax" {
		return "Minmax"
	}
	return rule
}

func buildHeader(task worker.ExperimentRunTask) (*pb.RunHeader, string) {
	params := parseParams(task.ExperimentParams)

	rawBallotFormat := getString(params, "ballot_format")
	if rawBallotFormat == "" {
		rawBallotFormat = strings.TrimSpace(task.Dataset.Format)
	}
	if rawBallotFormat == "" {
		rawBallotFormat = "ranking"
	}

	ballotFormat := normalizeBallotFormat(rawBallotFormat)
	if ballotFormat == "" {
		return nil, "unsupported_ballot_format_for_compute"
	}

	var approvalMaxChoicesPtr *int32
	if n, ok := getInt32(params, "approval_max_choices"); ok {
		approvalMaxChoicesPtr = &n
	}

	rawTallyRule := getString(params, "tally_rule")
	if rawTallyRule == "" {
		rawTallyRule = getString(params, "rule")
	}
	if rawTallyRule == "" {
		rawTallyRule = "plurality"
	}

	tallyRule := resolveComputeTallyRule(rawTallyRule, approvalMaxChoicesPtr)
	if tallyRule == "" {
		return nil, "unsupported_tally_rule_for_compute"
	}

	h := &pb.RunHeader{
		Kind:         strings.TrimSpace(task.Kind),
		RunId:        strings.TrimSpace(task.RunID),
		ExperimentId: strings.TrimSpace(task.ExperimentID),
		DatasetId:    strings.TrimSpace(task.DatasetID),
		TallyRule:    grpcTallyRuleName(tallyRule),
		BallotFormat: ballotFormat,
		ParamsJson:   []byte(task.ExperimentParams),
	}

	if n, ok := getInt32(params, "committee_size"); ok {
		h.CommitteeSize = wrapperspb.Int32(n)
	}
	if s := getString(params, "quota_type"); s != "" {
		h.QuotaType = wrapperspb.String(s)
	}
	if approvalMaxChoicesPtr != nil {
		h.ApprovalMaxChoices = wrapperspb.Int32(*approvalMaxChoicesPtr)
	}
	if n, ok := getInt32(params, "ranking_top_k"); ok {
		h.RankingTopK = wrapperspb.Int32(n)
	}
	if n, ok := getInt32(params, "score_min"); ok {
		h.ScoreMin = wrapperspb.Int32(n)
	}
	if n, ok := getInt32(params, "score_max"); ok {
		h.ScoreMax = wrapperspb.Int32(n)
	}
	if n, ok := getInt32(params, "score_step"); ok {
		h.ScoreStep = wrapperspb.Int32(n)
	}
	if b, ok := getBool(params, "score_allow_skip"); ok {
		h.ScoreAllowSkip = wrapperspb.Bool(b)
	}
	if b, ok := getBool(params, "show_aggregates"); ok {
		h.ShowAggregates = wrapperspb.Bool(b)
	}

	for _, c := range task.Dataset.Candidates {
		h.Candidates = append(h.Candidates, &pb.Candidate{Id: c.ID, Name: c.Name})
	}

	_ = json.Valid(h.ParamsJson)

	return h, ""
}

func buildElectionHeader(task worker.ElectionTallyTask) (*pb.RunHeader, string) {
	ballotFormat := normalizeBallotFormat(task.BallotFormat)
	if ballotFormat == "" {
		return nil, "unsupported_ballot_format_for_compute"
	}

	var approvalMaxChoicesPtr *int32
	if task.ApprovalMaxChoices != nil {
		v := int32(*task.ApprovalMaxChoices)
		approvalMaxChoicesPtr = &v
	}

	tallyRule := resolveComputeTallyRule(task.TallyRule, approvalMaxChoicesPtr)
	if tallyRule == "" {
		return nil, "unsupported_tally_rule_for_compute"
	}

	params := map[string]any{
		"ballot_format": ballotFormat,
		"tally_rule":    tallyRule,
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

	paramsJSON, _ := json.Marshal(params)

	h := &pb.RunHeader{
		Kind:           strings.TrimSpace(task.Kind),
		RunId:          strings.TrimSpace(task.JobID),
		ElectionId:     strings.TrimSpace(task.ElectionID),
		TallyRule:      grpcTallyRuleName(tallyRule),
		BallotFormat:   ballotFormat,
		ParamsJson:     paramsJSON,
		ShowAggregates: wrapperspb.Bool(task.ShowAggregates),
	}

	if task.CommitteeSize != nil {
		h.CommitteeSize = wrapperspb.Int32(int32(*task.CommitteeSize))
	}
	if task.QuotaType != nil && strings.TrimSpace(*task.QuotaType) != "" {
		h.QuotaType = wrapperspb.String(strings.TrimSpace(*task.QuotaType))
	}
	if task.ApprovalMaxChoices != nil {
		h.ApprovalMaxChoices = wrapperspb.Int32(int32(*task.ApprovalMaxChoices))
	}
	if task.RankingTopK != nil {
		h.RankingTopK = wrapperspb.Int32(int32(*task.RankingTopK))
	}
	if task.ScoreMin != nil {
		h.ScoreMin = wrapperspb.Int32(int32(*task.ScoreMin))
	}
	if task.ScoreMax != nil {
		h.ScoreMax = wrapperspb.Int32(int32(*task.ScoreMax))
	}
	if task.ScoreStep != nil {
		h.ScoreStep = wrapperspb.Int32(int32(*task.ScoreStep))
	}
	h.ScoreAllowSkip = wrapperspb.Bool(task.ScoreAllowSkip)

	for _, c := range task.Candidates {
		h.Candidates = append(h.Candidates, &pb.Candidate{
			Id:   strings.TrimSpace(c.ID),
			Name: strings.TrimSpace(c.Name),
		})
	}

	return h, ""
}
