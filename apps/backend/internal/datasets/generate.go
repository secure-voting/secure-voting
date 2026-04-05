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

func (s *Service) Generate(ctx context.Context, req GenerateReq) (string, string, error) {
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return "", "invalid_name", nil
	}

	format := normalizeFormat(req.Format)
	if !isValidFormat(format) {
		return "", "invalid_format", nil
	}

	if req.Voters <= 0 {
		return "", "invalid_voters", nil
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

	params := map[string]any{}
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
		v := int64(0)
		for i := 0; i < 8; i++ {
			v = (v << 8) | int64(sb[i])
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

	ballots := make([]BallotDoc, 0, req.Voters)

	var scoreMin, scoreMax, scoreStep, scoreSteps int
	if format == "score" {
		scoreMin = params["score_min"].(int)
		scoreMax = params["score_max"].(int)
		scoreStep = params["score_step"].(int)
		scoreSteps = ((scoreMax - scoreMin) / scoreStep) + 1
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
			b.Approval = pickSubset(rng, cids, k)

		case "ranking":
			top := len(cids)
			if req.RankingTopK != nil && *req.RankingTopK > 0 && *req.RankingTopK < top {
				top = *req.RankingTopK
			}
			sh := shuffle(rng, cids)
			b.Ranking = sh[:top]

		case "score":
			b.Scores = map[string]int{}
			for _, id := range cids {
				v := int(rng.next() % uint64(scoreSteps))
				b.Scores[id] = scoreMin + v*scoreStep
			}
		}

		ballots = append(ballots, b)
	}

	if len(ballots) > 0 {
		_, err = s.db.Collection("dataset_ballots").InsertMany(ctx, toAny(ballots))
		if err != nil {
			return "", "", err
		}
	}

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
		"ballots": ballotsToJSON(ballots),
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

	return dsid.Hex(), "", nil
}
