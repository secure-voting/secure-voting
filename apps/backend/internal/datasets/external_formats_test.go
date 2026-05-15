package datasets

import (
	"strings"
	"testing"

	"golang.org/x/text/encoding/charmap"
)

func TestParsePrefLibOrdinal_SOC(t *testing.T) {
	src := []byte(`# FILE NAME: sample.soc
# TITLE: Sample PrefLib
# NUMBER ALTERNATIVES: 3
# NUMBER VOTERS: 3
# NUMBER UNIQUE ORDERS: 2
# ALTERNATIVE NAME 1: Alice
# ALTERNATIVE NAME 2: Bob
# ALTERNATIVE NAME 3: Carol
2: 1,2,3
1: 2,1,3
`)
	parsed, err := parsePrefLibOrdinal(src, "sample.soc")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if parsed.Dataset.Format != "ranking" {
		t.Fatalf("unexpected format: %q", parsed.Dataset.Format)
	}
	if len(parsed.Dataset.Candidates) != 3 {
		t.Fatalf("unexpected candidates count: %d", len(parsed.Dataset.Candidates))
	}
	if len(parsed.Ballots) != 3 {
		t.Fatalf("unexpected ballots count: %d", len(parsed.Ballots))
	}
	if got := parsed.Ballots[0].Ranking; len(got) != 3 || got[0] != "1" || got[1] != "2" || got[2] != "3" {
		t.Fatalf("unexpected ranking: %+v", got)
	}
}

func TestParsePabulibPB_Approval(t *testing.T) {
	src := []byte(`META
key; value
description; PB approval sample
vote_type; approval
PROJECTS
project_id; cost
1; 100
2; 200
3; 300
VOTES
voter_id; vote
1; 1,2
2; 2,3
`)
	parsed, err := parsePabulibPB(src, "sample.pb")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if parsed.Dataset.Format != "approval" {
		t.Fatalf("unexpected format: %q", parsed.Dataset.Format)
	}
	if len(parsed.Dataset.Candidates) != 3 {
		t.Fatalf("unexpected candidates count: %d", len(parsed.Dataset.Candidates))
	}
	if len(parsed.Ballots) != 2 {
		t.Fatalf("unexpected ballots count: %d", len(parsed.Ballots))
	}
	if got := parsed.Ballots[0].Approval; len(got) != 2 || got[0] != "1" || got[1] != "2" {
		t.Fatalf("unexpected approval ballot: %+v", got)
	}
}

func TestParsePabulibPB_Ordinal(t *testing.T) {
	src := []byte(`META
key; value
description; PB ordinal sample
vote_type; ordinal
PROJECTS
project_id; name
1; Alpha
2; Beta
3; Gamma
VOTES
voter_id; vote
1; 2,1,3
`)
	parsed, err := parsePabulibPB(src, "sample.pb")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if parsed.Dataset.Format != "ranking" {
		t.Fatalf("unexpected format: %q", parsed.Dataset.Format)
	}
	if len(parsed.Ballots) != 1 {
		t.Fatalf("unexpected ballots count: %d", len(parsed.Ballots))
	}
	if got := parsed.Ballots[0].Ranking; len(got) != 3 || got[0] != "2" || got[1] != "1" || got[2] != "3" {
		t.Fatalf("unexpected ranking ballot: %+v", got)
	}
}

func TestParsePabulibPB_Scoring(t *testing.T) {
	src := []byte(`META
key; value
description; PB scoring sample
vote_type; scoring
PROJECTS
project_id; name
1; Alpha
2; Beta
3; Gamma
VOTES
voter_id; vote
1; 1=5,2=3,3=1
`)
	parsed, err := parsePabulibPB(src, "sample.pb")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if parsed.Dataset.Format != "score" {
		t.Fatalf("unexpected format: %q", parsed.Dataset.Format)
	}
	if len(parsed.Ballots) != 1 {
		t.Fatalf("unexpected ballots count: %d", len(parsed.Ballots))
	}
	if parsed.Ballots[0].Scores["1"] != 5 || parsed.Ballots[0].Scores["2"] != 3 || parsed.Ballots[0].Scores["3"] != 1 {
		t.Fatalf("unexpected scores: %+v", parsed.Ballots[0].Scores)
	}
}

func TestParseImportFile_PrefLibByExtension(t *testing.T) {
	src := []byte(`# ALTERNATIVE NAME 1: Alice
# ALTERNATIVE NAME 2: Bob
1: 1,2
`)
	parsed, ok := parseImportFile(src, "sample.soc", "text/plain", "")
	if !ok {
		t.Fatalf("expected parse ok")
	}
	if parsed.Dataset.Format != "ranking" {
		t.Fatalf("unexpected format: %q", parsed.Dataset.Format)
	}
}

func TestParseImportFile_PabulibByExtension(t *testing.T) {
	src := []byte(`META
key; value
vote_type; approval
PROJECTS
project_id; cost
1; 100
2; 100
VOTES
voter_id; vote
1; 1,2
`)
	parsed, ok := parseImportFile(src, "sample.pb", "text/plain", "")
	if !ok {
		t.Fatalf("expected parse ok")
	}
	if parsed.Dataset.Format != "approval" {
		t.Fatalf("unexpected format: %q", parsed.Dataset.Format)
	}
}

func TestParsePrefLibOrdinal_SOI(t *testing.T) {
	src := []byte(`# FILE NAME: sample.soi
# TITLE: Sample SOI
# NUMBER ALTERNATIVES: 3
# NUMBER VOTERS: 2
# NUMBER UNIQUE ORDERS: 2
# ALTERNATIVE NAME 1: Alice
# ALTERNATIVE NAME 2: Bob
# ALTERNATIVE NAME 3: Carol
1: 1,3,2
1: 3,2,1
`)
	parsed, ok := parseImportFile(src, "sample.soi", "text/plain", "")
	if !ok {
		t.Fatalf("expected parse ok")
	}
	if parsed.Dataset.Format != "ranking" {
		t.Fatalf("unexpected format: %q", parsed.Dataset.Format)
	}
	if len(parsed.Dataset.Candidates) != 3 {
		t.Fatalf("unexpected candidates count: %d", len(parsed.Dataset.Candidates))
	}
	if len(parsed.Ballots) != 2 {
		t.Fatalf("unexpected ballots count: %d", len(parsed.Ballots))
	}
	if got := parsed.Ballots[0].Ranking; len(got) != 3 || got[0] != "1" || got[1] != "3" || got[2] != "2" {
		t.Fatalf("unexpected first ranking: %+v", got)
	}
}

func TestParsePrefLibOrdinal_TiedRankingRejected(t *testing.T) {
	src := []byte(`# FILE NAME: tied.soc
# TITLE: Tied ranking
# ALTERNATIVE NAME 1: Alice
# ALTERNATIVE NAME 2: Bob
# ALTERNATIVE NAME 3: Carol
1: {1,2},3
`)
	_, err := parsePrefLibOrdinal(src, "tied.soc")
	if err == nil {
		t.Fatalf("expected error for tied ranking")
	}

	_, ok := parseImportFile(src, "tied.soc", "text/plain", "")
	if ok {
		t.Fatalf("expected parseImportFile to reject tied ranking")
	}
}

func TestParsePabulibPB_AppAlias(t *testing.T) {
	src := []byte(`META
key; value
description; PB app sample
vote_type; app
PROJECTS
project_id; name
1; Alpha
2; Beta
VOTES
voter_id; vote
1; 1,2
`)
	parsed, err := parsePabulibPB(src, "app.pb")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if parsed.Dataset.Format != "approval" {
		t.Fatalf("unexpected format: %q", parsed.Dataset.Format)
	}
	if got := parsed.Ballots[0].Approval; len(got) != 2 || got[0] != "1" || got[1] != "2" {
		t.Fatalf("unexpected approval ballot: %+v", got)
	}
}

func TestParsePabulibPB_AppDotAlias(t *testing.T) {
	src := []byte(`META
key; value
description; PB app dot sample
vote_type; app.
PROJECTS
project_id; name
1; Alpha
2; Beta
VOTES
voter_id; vote
1; 2
`)
	parsed, err := parsePabulibPB(src, "app-dot.pb")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if parsed.Dataset.Format != "approval" {
		t.Fatalf("unexpected format: %q", parsed.Dataset.Format)
	}
	if got := parsed.Ballots[0].Approval; len(got) != 1 || got[0] != "2" {
		t.Fatalf("unexpected approval ballot: %+v", got)
	}
}

func TestParsePabulibPB_ScoreAliasAndColonScores(t *testing.T) {
	src := []byte(`META
key; value
description; PB score sample
vote_type; score
PROJECTS
project_id; name
1; Alpha
2; Beta
3; Gamma
VOTES
voter_id; vote
1; 1:5,2:4,3:0
`)
	parsed, err := parsePabulibPB(src, "score.pb")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if parsed.Dataset.Format != "score" {
		t.Fatalf("unexpected format: %q", parsed.Dataset.Format)
	}
	if parsed.Ballots[0].Scores["1"] != 5 || parsed.Ballots[0].Scores["2"] != 4 || parsed.Ballots[0].Scores["3"] != 0 {
		t.Fatalf("unexpected scores: %+v", parsed.Ballots[0].Scores)
	}
}

func TestParsePabulibPB_UnsupportedVoteTypeRejected(t *testing.T) {
	src := []byte(`META
key; value
description; unsupported
vote_type; cumulative
PROJECTS
project_id; name
1; Alpha
VOTES
voter_id; vote
1; 1
`)
	_, err := parsePabulibPB(src, "unsupported.pb")
	if err == nil {
		t.Fatalf("expected unsupported vote_type error")
	}

	_, ok := parseImportFile(src, "unsupported.pb", "text/plain", "")
	if ok {
		t.Fatalf("expected parseImportFile to reject unsupported vote_type")
	}
}

func TestParseImportFile_CSVApproval(t *testing.T) {
	src := []byte(`record_type,id,name,voter_ref,approval,ranking,scores
candidate,c1,Alice,,,,
candidate,c2,Bob,,,,
candidate,c3,Carol,,,,
ballot,,,v1,c1|c2,,
ballot,,,v2,c2|c3,,
`)

	parsed, ok := parseImportFile(src, "approval.csv", "text/csv", "approval")
	if !ok {
		t.Fatalf("expected parse ok")
	}
	if parsed.Dataset.Format != "approval" {
		t.Fatalf("unexpected format: %q", parsed.Dataset.Format)
	}
	if len(parsed.Dataset.Candidates) != 3 {
		t.Fatalf("unexpected candidates count: %d", len(parsed.Dataset.Candidates))
	}
	if len(parsed.Ballots) != 2 {
		t.Fatalf("unexpected ballots count: %d", len(parsed.Ballots))
	}
	if got := parsed.Ballots[0].Approval; len(got) != 2 || got[0] != "c1" || got[1] != "c2" {
		t.Fatalf("unexpected approval ballot: %+v", got)
	}
}

func TestParseImportFile_CSVRanking(t *testing.T) {
	src := []byte(`record_type,id,name,voter_ref,approval,ranking,scores
candidate,c1,Alice,,,,
candidate,c2,Bob,,,,
candidate,c3,Carol,,,,
ballot,,,v1,,c2|c1|c3,
ballot,,,v2,,c3|c2|c1,
`)

	parsed, ok := parseImportFile(src, "ranking.csv", "text/csv", "ranking")
	if !ok {
		t.Fatalf("expected parse ok")
	}
	if parsed.Dataset.Format != "ranking" {
		t.Fatalf("unexpected format: %q", parsed.Dataset.Format)
	}
	if len(parsed.Dataset.Candidates) != 3 {
		t.Fatalf("unexpected candidates count: %d", len(parsed.Dataset.Candidates))
	}
	if got := parsed.Ballots[0].Ranking; len(got) != 3 || got[0] != "c2" || got[1] != "c1" || got[2] != "c3" {
		t.Fatalf("unexpected ranking ballot: %+v", got)
	}
}

func TestParseImportFile_CSVScore(t *testing.T) {
	src := []byte(`record_type,id,name,voter_ref,approval,ranking,scores
candidate,c1,Alice,,,,
candidate,c2,Bob,,,,
candidate,c3,Carol,,,,
ballot,,,v1,,,c1=5|c2=3|c3=1
ballot,,,v2,,,c1=2|c2=4|c3=5
`)

	parsed, ok := parseImportFile(src, "score.csv", "text/csv", "score")
	if !ok {
		t.Fatalf("expected parse ok")
	}
	if parsed.Dataset.Format != "score" {
		t.Fatalf("unexpected format: %q", parsed.Dataset.Format)
	}
	if len(parsed.Ballots) != 2 {
		t.Fatalf("unexpected ballots count: %d", len(parsed.Ballots))
	}
	if parsed.Ballots[0].Scores["c1"] != 5 || parsed.Ballots[0].Scores["c2"] != 3 || parsed.Ballots[0].Scores["c3"] != 1 {
		t.Fatalf("unexpected scores: %+v", parsed.Ballots[0].Scores)
	}
}

func TestParseImportFile_TXTRanking(t *testing.T) {
	line := func(cols ...string) string {
		return strings.Join(cols, "\t")
	}

	src := []byte(strings.Join([]string{
		line("record_type", "id", "name", "voter_ref", "approval", "ranking", "scores"),
		line("candidate", "c1", "Alice", "", "", "", ""),
		line("candidate", "c2", "Bob", "", "", "", ""),
		line("candidate", "c3", "Carol", "", "", "", ""),
		line("ballot", "", "", "v1", "", "c1|c3|c2", ""),
	}, "\n") + "\n")

	parsed, ok := parseImportFile(src, "ranking.txt", "text/plain", "ranking")
	if !ok {
		t.Fatalf("expected parse ok")
	}
	if parsed.Dataset.Format != "ranking" {
		t.Fatalf("unexpected format: %q", parsed.Dataset.Format)
	}
	if len(parsed.Dataset.Candidates) != 3 {
		t.Fatalf("unexpected candidates count: %d", len(parsed.Dataset.Candidates))
	}
	if len(parsed.Ballots) != 1 {
		t.Fatalf("unexpected ballots count: %d", len(parsed.Ballots))
	}
	if got := parsed.Ballots[0].Ranking; len(got) != 3 || got[0] != "c1" || got[1] != "c3" || got[2] != "c2" {
		t.Fatalf("unexpected ranking ballot: %+v", got)
	}
}

func TestParseImportFile_CSVInvalidRejected(t *testing.T) {
	src := []byte(`id,name
c1,Alice
`)

	_, ok := parseImportFile(src, "bad.csv", "text/csv", "ranking")
	if ok {
		t.Fatalf("expected invalid csv dataset to be rejected")
	}
}

func TestParseImportFile_CSVWindows1251CandidateNames(t *testing.T) {
	srcUTF8 := []byte(`record_type;id;name;voter_ref;approval;ranking;scores
candidate;c1;Алиса;;;;
candidate;c2;Борис;;;;
candidate;c3;Виктория;;;;
ballot;;;v1;;c1|c2|c3;
`)

	src1251, err := charmap.Windows1251.NewEncoder().Bytes(srcUTF8)
	if err != nil {
		t.Fatalf("encode windows-1251: %v", err)
	}

	parsed, ok := parseImportFile(src1251, "ranking.csv", "text/csv", "ranking")
	if !ok {
		t.Fatalf("expected parse ok")
	}
	if len(parsed.Dataset.Candidates) != 3 {
		t.Fatalf("unexpected candidates count: %d", len(parsed.Dataset.Candidates))
	}
	if parsed.Dataset.Candidates[0].Name != "Алиса" {
		t.Fatalf("unexpected candidate name: %q", parsed.Dataset.Candidates[0].Name)
	}
	if parsed.Dataset.Candidates[1].Name != "Борис" {
		t.Fatalf("unexpected candidate name: %q", parsed.Dataset.Candidates[1].Name)
	}
	if got := parsed.Ballots[0].Ranking; len(got) != 3 || got[0] != "c1" || got[1] != "c2" || got[2] != "c3" {
		t.Fatalf("unexpected ranking ballot: %+v", got)
	}
}

func TestParseImportFile_CSVRepairsMojibakeCandidateNames(t *testing.T) {
	src := []byte(`record_type;id;name;voter_ref;approval;ranking;scores
candidate;c1;РђР»РёСЃР°;;;;
candidate;c2;Р‘РѕСЂРёСЃ;;;;
candidate;c3;Р’РёРєС‚РѕСЂРёСЏ;;;;
ballot;;;v1;;c1|c2|c3;
`)

	parsed, ok := parseImportFile(src, "ranking.csv", "text/csv", "ranking")
	if !ok {
		t.Fatalf("expected parse ok")
	}
	if len(parsed.Dataset.Candidates) != 3 {
		t.Fatalf("unexpected candidates count: %d", len(parsed.Dataset.Candidates))
	}
	if parsed.Dataset.Candidates[0].Name != "Алиса" {
		t.Fatalf("unexpected candidate name: %q", parsed.Dataset.Candidates[0].Name)
	}
	if parsed.Dataset.Candidates[1].Name != "Борис" {
		t.Fatalf("unexpected candidate name: %q", parsed.Dataset.Candidates[1].Name)
	}
	if parsed.Dataset.Candidates[2].Name != "Виктория" {
		t.Fatalf("unexpected candidate name: %q", parsed.Dataset.Candidates[2].Name)
	}
}
