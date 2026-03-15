package main

type ballotDoc struct {
	VoterRef string   `bson:"voter_ref"`
	Ranking  []string `bson:"ranking"`
}
