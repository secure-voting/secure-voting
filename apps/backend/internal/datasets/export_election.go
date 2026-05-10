package datasets

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type electionDatasetSource struct {
	ID                 string
	Title              string
	Description        *string
	Status             string
	CreatedBy          string
	BallotFormat       string
	TallyRule          string
	ShowAggregates     bool
	ApprovalMaxChoices *int
	RankingTopK        *int
	ScoreMin           *int
	ScoreMax           *int
	ScoreStep          *int
	ScoreAllowSkip     bool
	CommitteeSize      *int
	QuotaType          *string
}

func (s *Service) ExportElection(ctx context.Context, req ExportElectionReq) (string, string, error) {
	if s.pg == nil {
		return "", "postgres_unavailable", nil
	}

	electionID := strings.TrimSpace(req.ElectionID)
	if _, err := uuid.Parse(electionID); err != nil {
		return "", "invalid_election_id", nil
	}

	actorUserID := strings.TrimSpace(req.ActorUserID)
	actorRole := strings.TrimSpace(req.ActorRole)
	if actorUserID == "" {
		return "", "unauthorized", nil
	}

	var src electionDatasetSource
	err := s.pg.QueryRow(ctx, `
		SELECT
			id::text,
			title,
			description,
			status,
			created_by::text,
			ballot_format,
			tally_rule,
			show_aggregates,
			approval_max_choices,
			ranking_top_k,
			score_min,
			score_max,
			score_step,
			score_allow_skip,
			committee_size,
			quota_type
		FROM elections
		WHERE id = $1::uuid
	`, electionID).Scan(
		&src.ID,
		&src.Title,
		&src.Description,
		&src.Status,
		&src.CreatedBy,
		&src.BallotFormat,
		&src.TallyRule,
		&src.ShowAggregates,
		&src.ApprovalMaxChoices,
		&src.RankingTopK,
		&src.ScoreMin,
		&src.ScoreMax,
		&src.ScoreStep,
		&src.ScoreAllowSkip,
		&src.CommitteeSize,
		&src.QuotaType,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", "not_found", nil
		}
		return "", "", err
	}

	if code := checkElectionDatasetAccess(src, actorUserID, actorRole); code != "" {
		return "", code, nil
	}

	candidates, code, err := s.loadElectionDatasetCandidates(ctx, electionID)
	if err != nil {
		return "", "", err
	}
	if code != "" {
		return "", code, nil
	}

	ballots, code, err := s.loadElectionDatasetBallots(ctx, electionID, src.BallotFormat)
	if err != nil {
		return "", "", err
	}
	if code != "" {
		return "", code, nil
	}

	name := strings.TrimSpace(req.Name)
	if name == "" {
		name = fmt.Sprintf("Dataset from election: %s", src.Title)
	}

	description := strings.TrimSpace(req.Description)
	if description == "" {
		description = fmt.Sprintf("Anonymized dataset exported from election %s", src.ID)
	}

	parameters := map[string]any{
		"source_election_id": src.ID,
		"source_election":    src.Title,
		"source_status":      src.Status,
		"tally_rule":         src.TallyRule,
		"ballot_format":      src.BallotFormat,
		"exported_at":        time.Now().UTC().Format(time.RFC3339),
		"anonymized":         true,
		"accepted_ballots":   len(ballots),
		"score_allow_skip":   src.ScoreAllowSkip,
		"show_aggregates":    src.ShowAggregates,
	}

	if src.ApprovalMaxChoices != nil {
		parameters["approval_max_choices"] = *src.ApprovalMaxChoices
	}
	if src.RankingTopK != nil {
		parameters["ranking_top_k"] = *src.RankingTopK
	}
	if src.ScoreMin != nil {
		parameters["score_min"] = *src.ScoreMin
	}
	if src.ScoreMax != nil {
		parameters["score_max"] = *src.ScoreMax
	}
	if src.ScoreStep != nil {
		parameters["score_step"] = *src.ScoreStep
	}
	if src.CommitteeSize != nil {
		parameters["committee_size"] = *src.CommitteeSize
	}
	if src.QuotaType != nil && strings.TrimSpace(*src.QuotaType) != "" {
		parameters["quota_type"] = strings.TrimSpace(*src.QuotaType)
	}

	rawFile := importFile{}
	rawFile.Dataset.Name = name
	rawFile.Dataset.Description = description
	rawFile.Dataset.Format = src.BallotFormat
	rawFile.Dataset.Candidates = candidates
	rawFile.Dataset.Parameters = parameters

	for _, ballot := range ballots {
		rawFile.Ballots = append(rawFile.Ballots, struct {
			VoterRef string         `json:"voter_ref,omitempty"`
			Approval []string       `json:"approval,omitempty"`
			Ranking  []string       `json:"ranking,omitempty"`
			Scores   map[string]int `json:"scores,omitempty"`
		}{
			VoterRef: ballot.VoterRef,
			Approval: ballot.Approval,
			Ranking:  ballot.Ranking,
			Scores:   ballot.Scores,
		})
	}

	rawBytes, err := json.MarshalIndent(rawFile, "", "  ")
	if err != nil {
		return "", "", err
	}

	doc := DatasetDoc{
		Name:        name,
		Description: description,
		Source:      "election",
		Format:      src.BallotFormat,
		Candidates:  candidates,
		CreatedAt:   time.Now().UTC(),
		Parameters:  parameters,
		Raw: primitive.Binary{
			Subtype: 0x00,
			Data:    rawBytes,
		},
		RawFilename: fmt.Sprintf("election-%s-dataset.json", src.ID),
		RawMime:     "application/json",
	}

	res, err := s.db.Collection("datasets").InsertOne(ctx, doc)
	if err != nil {
		return "", "", err
	}

	datasetID := res.InsertedID.(primitive.ObjectID)

	docs := make([]BallotDoc, 0, len(ballots))
	for _, ballot := range ballots {
		ballot.DatasetID = datasetID
		docs = append(docs, ballot)
	}

	if len(docs) > 0 {
		if _, err := s.db.Collection("dataset_ballots").InsertMany(ctx, toAny(docs)); err != nil {
			return "", "", err
		}
	}

	return datasetID.Hex(), "", nil
}

func checkElectionDatasetAccess(src electionDatasetSource, actorUserID string, actorRole string) string {
	switch actorRole {
	case "admin":
		if src.CreatedBy != actorUserID {
			return "forbidden"
		}
		if src.Status != "closed" && src.Status != "results_ready" && src.Status != "published" {
			return "election_not_ready"
		}
		return ""
	case "researcher":
		if src.Status != "published" {
			return "election_not_published"
		}
		if !src.ShowAggregates {
			return "aggregates_disabled"
		}
		return ""
	default:
		return "forbidden"
	}
}

func (s *Service) loadElectionDatasetCandidates(ctx context.Context, electionID string) ([]Candidate, string, error) {
	rows, err := s.pg.Query(ctx, `
		SELECT id::text, name
		FROM candidates
		WHERE election_id = $1::uuid
		ORDER BY name
	`, electionID)
	if err != nil {
		return nil, "", err
	}
	defer rows.Close()

	candidates := make([]Candidate, 0)
	for rows.Next() {
		var item Candidate
		if err := rows.Scan(&item.ID, &item.Name); err != nil {
			return nil, "", err
		}
		candidates = append(candidates, item)
	}

	if err := rows.Err(); err != nil {
		return nil, "", err
	}

	if code := validateCandidates(candidates); code != "" {
		return nil, code, nil
	}

	return candidates, "", nil
}

func (s *Service) loadElectionDatasetBallots(ctx context.Context, electionID string, format string) ([]BallotDoc, string, error) {
	rows, err := s.pg.Query(ctx, `
		SELECT approval_set, ranking, scores
		FROM ballots
		WHERE election_id = $1::uuid
		  AND status = 'accepted'
		ORDER BY submitted_at, id
	`, electionID)
	if err != nil {
		return nil, "", err
	}
	defer rows.Close()

	ballots := make([]BallotDoc, 0)
	index := 0

	for rows.Next() {
		var approvalRaw []byte
		var rankingRaw []byte
		var scoresRaw []byte

		if err := rows.Scan(&approvalRaw, &rankingRaw, &scoresRaw); err != nil {
			return nil, "", err
		}

		index++
		ballot := BallotDoc{
			VoterRef: fmt.Sprintf("v%d", index),
		}

		switch format {
		case "approval":
			approval, err := decodeStringArrayJSON(approvalRaw)
			if err != nil {
				return nil, "invalid_ballot_payload", nil
			}
			if len(approval) == 0 {
				continue
			}
			ballot.Approval = approval
		case "ranking":
			ranking, err := decodeStringArrayJSON(rankingRaw)
			if err != nil {
				return nil, "invalid_ballot_payload", nil
			}
			if len(ranking) == 0 {
				continue
			}
			ballot.Ranking = ranking
		case "score":
			scores, err := decodeScoresJSON(scoresRaw)
			if err != nil {
				return nil, "invalid_ballot_payload", nil
			}
			if len(scores) == 0 {
				continue
			}
			ballot.Scores = scores
		default:
			return nil, "invalid_format", nil
		}

		ballots = append(ballots, ballot)
	}

	if err := rows.Err(); err != nil {
		return nil, "", err
	}

	if len(ballots) == 0 {
		return nil, "no_accepted_ballots", nil
	}

	return ballots, "", nil
}

func decodeStringArrayJSON(raw []byte) ([]string, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return nil, nil
	}

	var items []string
	if err := json.Unmarshal(raw, &items); err != nil {
		return nil, err
	}

	out := make([]string, 0, len(items))
	seen := make(map[string]struct{}, len(items))
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		if _, ok := seen[item]; ok {
			continue
		}
		seen[item] = struct{}{}
		out = append(out, item)
	}

	return out, nil
}

func decodeScoresJSON(raw []byte) (map[string]int, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return nil, nil
	}

	var items map[string]int
	if err := json.Unmarshal(raw, &items); err != nil {
		return nil, err
	}

	out := make(map[string]int, len(items))
	for key, value := range items {
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		out[key] = value
	}

	return out, nil
}
