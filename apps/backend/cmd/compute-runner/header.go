package main

import (
	"encoding/json"
	"strings"

	"google.golang.org/protobuf/types/known/wrapperspb"

	pb "secure-voting/apps/backend/internal/compute/pb"
	"secure-voting/apps/backend/internal/worker"
)

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

	rawTallyRule := getString(params, "tally_rule")
	if rawTallyRule == "" {
		rawTallyRule = getString(params, "rule")
	}
	if rawTallyRule == "" {
		rawTallyRule = "plurality"
	}

	tallyRule := normalizeComputeTallyRule(rawTallyRule)
	if tallyRule == "" {
		return nil, "unsupported_tally_rule_for_compute"
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
		return nil, "unsupported_ballot_format_for_compute"
	}

	_ = json.Valid(h.ParamsJson)

	return h, ""
}

func buildElectionHeader(task worker.ElectionTallyTask) (*pb.RunHeader, string) {
	ballotFormat := normalizeBallotFormat(task.BallotFormat)
	if ballotFormat != "ranking" {
		return nil, "unsupported_ballot_format_for_compute"
	}

	tallyRule := normalizeComputeTallyRule(task.TallyRule)
	if tallyRule == "" {
		return nil, "unsupported_tally_rule_for_compute"
	}
	if tallyRule == "minmax" {
		tallyRule = "Minmax"
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
	if task.RankingTopK != nil {
		params["ranking_top_k"] = *task.RankingTopK
	}
	params["show_aggregates"] = task.ShowAggregates

	paramsJSON, _ := json.Marshal(params)

	h := &pb.RunHeader{
		Kind:           strings.TrimSpace(task.Kind),
		RunId:          strings.TrimSpace(task.JobID),
		ElectionId:     strings.TrimSpace(task.ElectionID),
		TallyRule:      tallyRule,
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
	if task.RankingTopK != nil {
		h.RankingTopK = wrapperspb.Int32(int32(*task.RankingTopK))
	}

	for _, c := range task.Candidates {
		h.Candidates = append(h.Candidates, &pb.Candidate{
			Id:   strings.TrimSpace(c.ID),
			Name: strings.TrimSpace(c.Name),
		})
	}

	return h, ""
}
