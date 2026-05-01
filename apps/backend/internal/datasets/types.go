package datasets

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type Candidate struct {
	ID   string `json:"id" bson:"id"`
	Name string `json:"name" bson:"name"`
}

type Dataset struct {
	ID          string         `json:"id" bson:"-"`
	Name        string         `json:"name" bson:"name"`
	Description string         `json:"description,omitempty" bson:"description,omitempty"`
	Source      string         `json:"source" bson:"source"`
	Format      string         `json:"format" bson:"format"`
	Candidates  []Candidate    `json:"candidates" bson:"candidates"`
	CreatedAt   string         `json:"created_at" bson:"-"`
	Seed        *int64         `json:"seed,omitempty" bson:"seed,omitempty"`
	Parameters  map[string]any `json:"parameters,omitempty" bson:"-"`
}

type ListItem struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Source    string `json:"source"`
	Format    string `json:"format"`
	CreatedAt string `json:"created_at"`
}

type GenerateReq struct {
	Name        string      `json:"name"`
	Description string      `json:"description,omitempty"`
	Format      string      `json:"format"`
	Candidates  []Candidate `json:"candidates"`
	Voters      int         `json:"voters"`
	Seed        *int64      `json:"seed,omitempty"`

	GenerationModel string `json:"generation_model,omitempty"`

	ApprovalMaxChoices *int `json:"approval_max_choices,omitempty"`
	RankingTopK        *int `json:"ranking_top_k,omitempty"`
	ScoreMin           *int `json:"score_min,omitempty"`
	ScoreMax           *int `json:"score_max,omitempty"`
	ScoreStep          *int `json:"score_step,omitempty"`
}

type ExportElectionReq struct {
	ElectionID  string `json:"election_id"`
	Name        string `json:"name,omitempty"`
	Description string `json:"description,omitempty"`
	ActorUserID string `json:"-"`
	ActorRole   string `json:"-"`
}

type ImportMeta struct {
	Name        string
	Description string
	Format      string
}

type DatasetDoc struct {
	ID          primitive.ObjectID `bson:"_id,omitempty"`
	Name        string             `bson:"name"`
	Description string             `bson:"description,omitempty"`
	Source      string             `bson:"source"`
	Format      string             `bson:"format"`
	Candidates  []Candidate        `bson:"candidates"`
	CreatedAt   time.Time          `bson:"created_at"`
	Seed        *int64             `bson:"seed,omitempty"`

	Raw         primitive.Binary `bson:"raw,omitempty"`
	RawFilename string           `bson:"raw_filename,omitempty"`
	RawMime     string           `bson:"raw_mime,omitempty"`
	Parameters  map[string]any   `bson:"parameters,omitempty"`
}

type BallotDoc struct {
	ID        primitive.ObjectID `bson:"_id,omitempty"`
	DatasetID primitive.ObjectID `bson:"dataset_id"`
	Approval  []string           `bson:"approval,omitempty"`
	Ranking   []string           `bson:"ranking,omitempty"`
	Scores    map[string]int     `bson:"scores,omitempty"`
	VoterRef  string             `bson:"voter_ref,omitempty"`
}

type importFile struct {
	Dataset struct {
		Name        string         `json:"name"`
		Description string         `json:"description,omitempty"`
		Format      string         `json:"format"`
		Candidates  []Candidate    `json:"candidates"`
		Seed        *int64         `json:"seed,omitempty"`
		Parameters  map[string]any `json:"parameters,omitempty"`
	} `json:"dataset"`
	Ballots []struct {
		VoterRef string         `json:"voter_ref,omitempty"`
		Approval []string       `json:"approval,omitempty"`
		Ranking  []string       `json:"ranking,omitempty"`
		Scores   map[string]int `json:"scores,omitempty"`
	} `json:"ballots"`
}
