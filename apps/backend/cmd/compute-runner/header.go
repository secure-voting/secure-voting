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

	_ = json.Valid(h.ParamsJson)

	return h, ""
}
