package main

type ballotDoc struct {
	VoterRef string         `bson:"voter_ref"`
	Approval []string       `bson:"approval"`
	Ranking  []string       `bson:"ranking"`
	Scores   map[string]any `bson:"scores"`
}
