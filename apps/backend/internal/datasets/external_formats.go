package datasets

import (
	"bufio"
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

func parseImportFile(b []byte, filename, mime string) (importFile, bool) {
	var parsed importFile

	lowerMime := strings.ToLower(strings.TrimSpace(mime))
	ext := strings.ToLower(strings.TrimSpace(filepath.Ext(filename)))

	if strings.Contains(lowerMime, "json") || ext == ".json" {
		if err := json.Unmarshal(b, &parsed); err == nil && strings.TrimSpace(parsed.Dataset.Format) != "" {
			return parsed, true
		}
	}

	if ext == ".soc" || ext == ".soi" {
		if pf, err := parsePrefLibOrdinal(b, filename); err == nil {
			return pf, true
		}
	}

	if ext == ".pb" {
		if pf, err := parsePabulibPB(b, filename); err == nil {
			return pf, true
		}
	}

	return importFile{}, false
}

func parsePrefLibOrdinal(b []byte, filename string) (importFile, error) {
	var parsed importFile
	parsed.Dataset.Name = strings.TrimSpace(strings.TrimSuffix(filepath.Base(filename), filepath.Ext(filename)))
	parsed.Dataset.Format = "ranking"

	altNames := map[string]string{}
	sc := bufio.NewScanner(bytes.NewReader(b))

	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}

		if strings.HasPrefix(line, "#") {
			header := strings.TrimSpace(strings.TrimPrefix(line, "#"))
			if strings.HasPrefix(header, "TITLE:") && parsed.Dataset.Name == "" {
				parsed.Dataset.Name = strings.TrimSpace(strings.TrimPrefix(header, "TITLE:"))
			}

			const prefix = "ALTERNATIVE NAME "
			if strings.HasPrefix(header, prefix) {
				rest := strings.TrimSpace(strings.TrimPrefix(header, prefix))
				parts := strings.SplitN(rest, ":", 2)
				if len(parts) == 2 {
					id := strings.TrimSpace(parts[0])
					name := strings.TrimSpace(parts[1])
					if id != "" && name != "" {
						altNames[id] = name
					}
				}
			}
			continue
		}

		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}

		countStr := strings.TrimSpace(parts[0])
		orderStr := strings.TrimSpace(parts[1])
		count, err := strconv.Atoi(countStr)
		if err != nil || count <= 0 {
			continue
		}

		ranking := parsePrefLibRanking(orderStr)
		if len(ranking) == 0 {
			continue
		}

		for i := 0; i < count; i++ {
			parsed.Ballots = append(parsed.Ballots, struct {
				VoterRef string         `json:"voter_ref,omitempty"`
				Approval []string       `json:"approval,omitempty"`
				Ranking  []string       `json:"ranking,omitempty"`
				Scores   map[string]int `json:"scores,omitempty"`
			}{
				VoterRef: fmt.Sprintf("v%d", len(parsed.Ballots)+1),
				Ranking:  ranking,
			})
		}
	}

	if err := sc.Err(); err != nil {
		return importFile{}, err
	}

	ids := make([]string, 0, len(altNames))
	for id := range altNames {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	for _, id := range ids {
		parsed.Dataset.Candidates = append(parsed.Dataset.Candidates, Candidate{
			ID:   id,
			Name: altNames[id],
		})
	}

	if len(parsed.Dataset.Candidates) == 0 || len(parsed.Ballots) == 0 {
		return importFile{}, fmt.Errorf("invalid preflib file")
	}

	return parsed, nil
}

func parsePrefLibRanking(s string) []string {
	if s == "" {
		return nil
	}

	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	seen := map[string]struct{}{}

	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if strings.Contains(part, "{") || strings.Contains(part, "}") {
			return nil
		}
		if _, ok := seen[part]; ok {
			continue
		}
		seen[part] = struct{}{}
		out = append(out, part)
	}

	return out
}

func parsePabulibPB(b []byte, filename string) (importFile, error) {
	var parsed importFile
	parsed.Dataset.Name = strings.TrimSpace(strings.TrimSuffix(filepath.Base(filename), filepath.Ext(filename)))

	sc := bufio.NewScanner(bytes.NewReader(b))
	section := ""
	meta := map[string]string{}
	projectIDs := []string{}
	projectNames := map[string]string{}
	voteType := ""

	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}

		upper := strings.ToUpper(line)
		if upper == "META" || upper == "PROJECTS" || upper == "VOTES" {
			section = upper
			continue
		}

		switch section {
		case "META":
			if strings.EqualFold(line, "key; value") {
				continue
			}
			parts := splitSemi(line)
			if len(parts) < 2 {
				continue
			}
			key := strings.ToLower(strings.TrimSpace(parts[0]))
			value := strings.TrimSpace(parts[1])
			meta[key] = value
			if key == "vote_type" {
				voteType = strings.ToLower(value)
			}
			if key == "description" && parsed.Dataset.Name == "" {
				parsed.Dataset.Name = value
			}

		case "PROJECTS":
			if strings.HasPrefix(strings.ToLower(line), "project_id;") {
				continue
			}
			parts := splitSemi(line)
			if len(parts) < 1 {
				continue
			}
			id := strings.TrimSpace(parts[0])
			if id == "" {
				continue
			}
			name := id
			if len(parts) >= 2 && strings.TrimSpace(parts[1]) != "" {
				name = strings.TrimSpace(parts[1])
			}
			if _, ok := projectNames[id]; !ok {
				projectIDs = append(projectIDs, id)
			}
			projectNames[id] = name

		case "VOTES":
			if strings.Contains(strings.ToLower(line), "vote") && strings.Contains(line, ";") {
				continue
			}
			parts := splitSemi(line)
			if len(parts) == 0 {
				continue
			}
			voteRaw := strings.TrimSpace(parts[len(parts)-1])
			if voteRaw == "" {
				continue
			}

			ballot := struct {
				VoterRef string         `json:"voter_ref,omitempty"`
				Approval []string       `json:"approval,omitempty"`
				Ranking  []string       `json:"ranking,omitempty"`
				Scores   map[string]int `json:"scores,omitempty"`
			}{
				VoterRef: fmt.Sprintf("v%d", len(parsed.Ballots)+1),
			}

			switch voteType {
			case "approval", "app", "app.":
				ballot.Approval = parseDelimitedIDs(voteRaw)
				if len(ballot.Approval) == 0 {
					continue
				}
			case "ordinal":
				ballot.Ranking = parseDelimitedIDs(voteRaw)
				if len(ballot.Ranking) == 0 {
					continue
				}
			case "scoring", "score":
				scores := parsePabulibScores(voteRaw)
				if len(scores) == 0 {
					continue
				}
				ballot.Scores = scores
			default:
				continue
			}

			parsed.Ballots = append(parsed.Ballots, ballot)
		}
	}

	if err := sc.Err(); err != nil {
		return importFile{}, err
	}

	for _, id := range projectIDs {
		parsed.Dataset.Candidates = append(parsed.Dataset.Candidates, Candidate{
			ID:   id,
			Name: projectNames[id],
		})
	}

	switch voteType {
	case "approval", "app", "app.":
		parsed.Dataset.Format = "approval"
	case "ordinal":
		parsed.Dataset.Format = "ranking"
	case "scoring", "score":
		parsed.Dataset.Format = "score"
		if parsed.Dataset.Parameters == nil {
			parsed.Dataset.Parameters = map[string]any{
				"score_min":  0,
				"score_max":  5,
				"score_step": 1,
			}
		}
	default:
		return importFile{}, fmt.Errorf("unsupported pabulib vote_type")
	}

	if len(parsed.Dataset.Candidates) == 0 || len(parsed.Ballots) == 0 {
		return importFile{}, fmt.Errorf("invalid pabulib file")
	}

	return parsed, nil
}

func splitSemi(s string) []string {
	r := csv.NewReader(strings.NewReader(s))
	r.Comma = ';'
	r.TrimLeadingSpace = true
	r.FieldsPerRecord = -1
	rec, err := r.Read()
	if err != nil {
		return nil
	}
	return rec
}

func parseDelimitedIDs(s string) []string {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	parts := strings.FieldsFunc(s, func(r rune) bool {
		return r == ',' || r == '|' || r == '>'
	})
	out := make([]string, 0, len(parts))
	seen := map[string]struct{}{}
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if _, ok := seen[part]; ok {
			continue
		}
		seen[part] = struct{}{}
		out = append(out, part)
	}
	return out
}

func parsePabulibScores(s string) map[string]int {
	out := map[string]int{}
	items := strings.Split(s, ",")
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}

		var key, val string
		switch {
		case strings.Contains(item, "="):
			parts := strings.SplitN(item, "=", 2)
			key, val = strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1])
		case strings.Contains(item, ":"):
			parts := strings.SplitN(item, ":", 2)
			key, val = strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1])
		default:
			continue
		}

		n, err := strconv.Atoi(val)
		if err != nil {
			continue
		}
		if key != "" {
			out[key] = n
		}
	}
	return out
}
