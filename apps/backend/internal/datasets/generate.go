package datasets

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

const (
	maxGeneratedVoters             = 10_000_000
	generatedBallotInsertBatchSize = 1_000
	generatedRawExportBallotLimit  = 10_000
)

func normalizeGenerationModel(v string) (string, bool) {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "", "uniform":
		return "uniform", true
	case "consensus":
		return "consensus", true
	case "polarized":
		return "polarized", true
	default:
		return "", false
	}
}

func reverseStrings(items []string) []string {
	out := append([]string(nil), items...)
	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	return out
}

func noisyOrder(rng *lcg, base []string, swaps int) []string {
	out := append([]string(nil), base...)
	if len(out) < 2 {
		return out
	}
	if swaps < 1 {
		swaps = 1
	}
	for i := 0; i < swaps; i++ {
		j := int(rng.next() % uint64(len(out)-1))
		out[j], out[j+1] = out[j+1], out[j]
	}
	return out
}

func buildPreferenceOrder(rng *lcg, candidateIDs []string, model string, consensusOrder []string, oppositeOrder []string) []string {
	switch model {
	case "uniform":
		return shuffle(rng, candidateIDs)
	case "consensus":
		return noisyOrder(rng, consensusOrder, 1+int(rng.next()%3))
	case "polarized":
		if rng.next()%2 == 0 {
			return noisyOrder(rng, consensusOrder, 1+int(rng.next()%2))
		}
		return noisyOrder(rng, oppositeOrder, 1+int(rng.next()%2))
	default:
		return shuffle(rng, candidateIDs)
	}
}

func scoreLevels(scoreMin, scoreMax, scoreStep int) []int {
	if scoreStep <= 0 || scoreMax < scoreMin {
		return nil
	}

	out := make([]int, 0, ((scoreMax-scoreMin)/scoreStep)+1)
	for v := scoreMin; v <= scoreMax; v += scoreStep {
		out = append(out, v)
	}
	return out
}

func scoresFromOrder(order []string, levels []int) map[string]int {
	out := make(map[string]int, len(order))
	if len(order) == 0 || len(levels) == 0 {
		return out
	}

	if len(order) == 1 {
		out[order[0]] = levels[len(levels)-1]
		return out
	}

	maxLevelIdx := len(levels) - 1
	for idx, id := range order {
		levelIdx := maxLevelIdx - (idx*maxLevelIdx)/(len(order)-1)
		if levelIdx < 0 {
			levelIdx = 0
		}
		out[id] = levels[levelIdx]
	}

	return out
}

func (s *Service) insertGeneratedBallotBatch(ctx context.Context, batch []BallotDoc) error {
	if len(batch) == 0 {
		return nil
	}

	_, err := s.db.Collection("dataset_ballots").InsertMany(ctx, toAny(batch))
	return err
}

func (s *Service) Generate(ctx context.Context, req GenerateReq) (string, string, error) {
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return "", "invalid_name", nil
	}

	format := normalizeFormat(req.Format)
	if !isValidFormat(format) {
		return "", "invalid_format", nil
	}

	generationModel, ok := normalizeGenerationModel(req.GenerationModel)
	if !ok {
		return "", "invalid_generation_model", nil
	}

	if req.Voters <= 0 {
		return "", "invalid_voters", nil
	}

	if req.Voters > maxGeneratedVoters {
		return "", "too_many_voters", nil
	}

	if code := validateCandidates(req.Candidates); code != "" {
		return "", code, nil
	}

	candidates := make([]Candidate, 0, len(req.Candidates))
	cids := make([]string, 0, len(req.Candidates))
	for _, c := range req.Candidates {
		id := strings.TrimSpace(c.ID)
		nm := strings.TrimSpace(c.Name)
		candidates = append(candidates, Candidate{ID: id, Name: nm})
		cids = append(cids, id)
	}

	params := map[string]any{
		"generation_model": generationModel,
		"voters":           req.Voters,
	}
	switch format {
	case "approval":
		if req.ApprovalMaxChoices != nil {
			params["approval_max_choices"] = *req.ApprovalMaxChoices
		}
	case "ranking":
		if req.RankingTopK != nil {
			params["ranking_top_k"] = *req.RankingTopK
		}
	case "score":
		if req.ScoreMin != nil {
			params["score_min"] = *req.ScoreMin
		}
		if req.ScoreMax != nil {
			params["score_max"] = *req.ScoreMax
		}
		if req.ScoreStep != nil {
			params["score_step"] = *req.ScoreStep
		}
	}

	if code := validateDatasetParams(format, params, len(candidates), format == "score"); code != "" {
		return "", code, nil
	}

	seed := req.Seed
	if seed == nil {
		sb := make([]byte, 8)
		_, _ = rand.Read(sb)

		var u uint64
		for i := 0; i < 8; i++ {
			u = (u << 8) | uint64(sb[i])
		}

		v := int64(u & 0x7fffffffffffffff)
		if v == 0 {
			v = 1
		}

		seed = &v
	}

	dsDoc := DatasetDoc{
		Name:        name,
		Description: strings.TrimSpace(req.Description),
		Source:      "generate",
		Format:      format,
		Candidates:  candidates,
		CreatedAt:   time.Now().UTC(),
		Seed:        seed,
		Parameters:  params,
	}

	ins, err := s.db.Collection("datasets").InsertOne(ctx, dsDoc)
	if err != nil {
		return "", "", err
	}
	dsid := ins.InsertedID.(primitive.ObjectID)

	rng := newLCG(uint64(*seed))

	baseOrder := shuffle(rng, cids)
	oppositeOrder := reverseStrings(baseOrder)

	storeRawExport := req.Voters <= generatedRawExportBallotLimit

	var rawBallots []BallotDoc
	if storeRawExport {
		rawBallots = make([]BallotDoc, 0, req.Voters)
	}

	batch := make([]BallotDoc, 0, generatedBallotInsertBatchSize)

	var scoreMin, scoreMax, scoreStep, scoreSteps int
	var levels []int
	if format == "score" {
		scoreMin = params["score_min"].(int)
		scoreMax = params["score_max"].(int)
		scoreStep = params["score_step"].(int)
		scoreSteps = ((scoreMax - scoreMin) / scoreStep) + 1
		levels = scoreLevels(scoreMin, scoreMax, scoreStep)
	}

	for i := 0; i < req.Voters; i++ {
		b := BallotDoc{
			DatasetID: dsid,
			VoterRef:  "v" + itoa(i+1),
		}

		switch format {
		case "approval":
			q := len(cids)
			if req.ApprovalMaxChoices != nil && *req.ApprovalMaxChoices > 0 {
				q = *req.ApprovalMaxChoices
			}
			if q > len(cids) {
				q = len(cids)
			}
			if q < 1 {
				q = 1
			}

			k := 1 + int(rng.next()%uint64(q))

			if generationModel == "uniform" {
				b.Approval = pickSubset(rng, cids, k)
			} else {
				order := buildPreferenceOrder(rng, cids, generationModel, baseOrder, oppositeOrder)
				b.Approval = append([]string(nil), order[:k]...)
			}

		case "ranking":
			top := len(cids)
			if req.RankingTopK != nil && *req.RankingTopK > 0 && *req.RankingTopK < top {
				top = *req.RankingTopK
			}

			order := buildPreferenceOrder(rng, cids, generationModel, baseOrder, oppositeOrder)
			b.Ranking = append([]string(nil), order[:top]...)

		case "score":
			if generationModel == "uniform" {
				b.Scores = map[string]int{}
				for _, id := range cids {
					v := int(rng.next() % uint64(scoreSteps))
					b.Scores[id] = scoreMin + v*scoreStep
				}
			} else {
				order := buildPreferenceOrder(rng, cids, generationModel, baseOrder, oppositeOrder)
				b.Scores = scoresFromOrder(order, levels)
			}
		}

		if storeRawExport {
			rawBallots = append(rawBallots, b)
		}

		batch = append(batch, b)
		if len(batch) >= generatedBallotInsertBatchSize {
			if err := s.insertGeneratedBallotBatch(ctx, batch); err != nil {
				return "", "", err
			}
			batch = batch[:0]
		}
	}

	if err := s.insertGeneratedBallotBatch(ctx, batch); err != nil {
		return "", "", err
	}

	if storeRawExport {
		export := map[string]any{
			"dataset": map[string]any{
				"id":          dsid.Hex(),
				"name":        dsDoc.Name,
				"description": dsDoc.Description,
				"source":      dsDoc.Source,
				"format":      dsDoc.Format,
				"candidates":  dsDoc.Candidates,
				"created_at":  dsDoc.CreatedAt.UTC().Format(time.RFC3339),
				"seed":        dsDoc.Seed,
				"parameters":  dsDoc.Parameters,
			},
			"ballots": ballotsToJSON(rawBallots),
		}
		raw, _ := json.Marshal(export)

		_, err = s.db.Collection("datasets").UpdateOne(ctx,
			bson.M{"_id": dsid},
			bson.M{"$set": bson.M{
				"raw":          primitive.Binary{Subtype: 0x00, Data: raw},
				"raw_filename": "dataset.json",
				"raw_mime":     "application/json",
			}},
		)
		if err != nil {
			return "", "", err
		}
	} else {
		_, err = s.db.Collection("datasets").UpdateOne(ctx,
			bson.M{"_id": dsid},
			bson.M{"$set": bson.M{
				"parameters.raw_export_omitted":      true,
				"parameters.raw_export_ballot_limit": generatedRawExportBallotLimit,
				"parameters.raw_export_omit_reason":  "generated dataset is stored in dataset_ballots; raw JSON export was skipped to avoid MongoDB document size limit",
			}},
		)
		if err != nil {
			return "", "", err
		}
	}

	return dsid.Hex(), "", nil
}
