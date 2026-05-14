package datasets

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
)

func isTabularDatasetFile(filename, mime string) bool {
	ext := strings.ToLower(strings.TrimSpace(filepath.Ext(filename)))
	if ext == ".csv" || ext == ".txt" {
		return true
	}

	lowerMime := strings.ToLower(strings.TrimSpace(mime))
	return strings.Contains(lowerMime, "text/csv")
}

func parseTabularDataset(b []byte, filename, formatHint string) (importFile, error) {
	records, err := readTabularRecords(b, filename)
	if err != nil {
		return importFile{}, err
	}
	if len(records) < 2 {
		return importFile{}, fmt.Errorf("tabular dataset must contain header and data rows")
	}

	header := normalizeTabularHeader(records[0])
	if _, ok := header["record_type"]; !ok {
		return importFile{}, fmt.Errorf("missing record_type column")
	}

	var parsed importFile
	parsed.Dataset.Name = strings.TrimSpace(strings.TrimSuffix(filepath.Base(filename), filepath.Ext(filename)))

	format := normalizeFormat(formatHint)

	for _, rec := range records[1:] {
		if isEmptyTabularRecord(rec) {
			continue
		}
		if len(rec) > 0 && strings.HasPrefix(strings.TrimSpace(rec[0]), "#") {
			continue
		}

		row := tabularRecordToMap(header, rec)
		recordType := strings.ToLower(strings.TrimSpace(row["record_type"]))

		switch recordType {
		case "meta", "dataset":
			if format == "" {
				format = normalizeFormat(row["format"])
			}
			if name := strings.TrimSpace(row["name"]); name != "" {
				parsed.Dataset.Name = name
			}
			if desc := strings.TrimSpace(row["description"]); desc != "" {
				parsed.Dataset.Description = desc
			}

		case "candidate", "candidates":
			id := strings.TrimSpace(row["id"])
			name := strings.TrimSpace(row["name"])
			if id == "" || name == "" {
				return importFile{}, fmt.Errorf("candidate row must contain id and name")
			}
			parsed.Dataset.Candidates = append(parsed.Dataset.Candidates, Candidate{
				ID:   id,
				Name: name,
			})

		case "ballot", "ballots":
			ballot := struct {
				VoterRef string         `json:"voter_ref,omitempty"`
				Approval []string       `json:"approval,omitempty"`
				Ranking  []string       `json:"ranking,omitempty"`
				Scores   map[string]int `json:"scores,omitempty"`
			}{
				VoterRef: strings.TrimSpace(row["voter_ref"]),
			}
			if ballot.VoterRef == "" {
				ballot.VoterRef = "v" + itoa(len(parsed.Ballots)+1)
			}

			switch format {
			case "approval":
				ballot.Approval = parseDelimitedIDs(row["approval"])
				if len(ballot.Approval) == 0 {
					return importFile{}, fmt.Errorf("approval ballot row must contain approval values")
				}
			case "ranking":
				ballot.Ranking = parseDelimitedIDs(row["ranking"])
				if len(ballot.Ranking) == 0 {
					return importFile{}, fmt.Errorf("ranking ballot row must contain ranking values")
				}
			case "score":
				ballot.Scores = parseTabularScores(row["scores"])
				if len(ballot.Scores) == 0 {
					return importFile{}, fmt.Errorf("score ballot row must contain scores values")
				}
			default:
				return importFile{}, fmt.Errorf("unsupported or missing dataset format")
			}

			parsed.Ballots = append(parsed.Ballots, ballot)

		case "":
			continue

		default:
			return importFile{}, fmt.Errorf("unsupported record_type %q", recordType)
		}
	}

	if !isValidFormat(format) {
		return importFile{}, fmt.Errorf("invalid dataset format")
	}
	parsed.Dataset.Format = format

	if len(parsed.Dataset.Candidates) == 0 {
		return importFile{}, fmt.Errorf("tabular dataset must contain candidate rows")
	}
	if len(parsed.Ballots) == 0 {
		return importFile{}, fmt.Errorf("tabular dataset must contain ballot rows")
	}

	return parsed, nil
}

func readTabularRecords(b []byte, filename string) ([][]string, error) {
	delimiter := detectTabularDelimiter(b, filename)

	if delimiter == '\t' {
		return readTabDelimitedRecords(b), nil
	}

	reader := csv.NewReader(bytes.NewReader(b))
	reader.Comma = delimiter
	reader.TrimLeadingSpace = true
	reader.FieldsPerRecord = -1

	records, err := reader.ReadAll()
	if err != nil {
		return nil, err
	}
	return records, nil
}

func readTabDelimitedRecords(b []byte) [][]string {
	text := strings.ReplaceAll(string(b), "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")

	lines := strings.Split(text, "\n")
	records := make([][]string, 0, len(lines))

	for _, line := range lines {
		if line == "" {
			continue
		}

		record := strings.Split(line, "\t")
		for i := range record {
			record[i] = strings.TrimSpace(record[i])
		}

		records = append(records, record)
	}

	return records
}

func detectTabularDelimiter(b []byte, filename string) rune {
	firstLine := firstNonEmptyLine(string(b))
	ext := strings.ToLower(strings.TrimSpace(filepath.Ext(filename)))

	commaCount := strings.Count(firstLine, ",")
	semiCount := strings.Count(firstLine, ";")
	tabCount := strings.Count(firstLine, "\t")

	if tabCount >= commaCount && tabCount >= semiCount && tabCount > 0 {
		return '\t'
	}
	if semiCount >= commaCount && semiCount > 0 {
		return ';'
	}
	if commaCount > 0 {
		return ','
	}
	if ext == ".txt" {
		return '\t'
	}
	return ','
}

func firstNonEmptyLine(s string) string {
	lines := strings.Split(s, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			return line
		}
	}
	return ""
}

func normalizeTabularHeader(record []string) map[string]int {
	out := make(map[string]int, len(record))
	for idx, value := range record {
		key := strings.ToLower(strings.TrimSpace(strings.TrimPrefix(value, "\ufeff")))
		if key != "" {
			out[key] = idx
		}
	}
	return out
}

func tabularRecordToMap(header map[string]int, record []string) map[string]string {
	out := make(map[string]string, len(header))
	for key, idx := range header {
		if idx >= 0 && idx < len(record) {
			out[key] = strings.TrimSpace(record[idx])
		}
	}
	return out
}

func isEmptyTabularRecord(record []string) bool {
	for _, value := range record {
		if strings.TrimSpace(value) != "" {
			return false
		}
	}
	return true
}

func parseTabularScores(s string) map[string]int {
	out := map[string]int{}

	parts := strings.FieldsFunc(s, func(r rune) bool {
		return r == '|' || r == ',' || r == '>'
	})

	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		var key string
		var value string

		switch {
		case strings.Contains(part, "="):
			items := strings.SplitN(part, "=", 2)
			key = strings.TrimSpace(items[0])
			value = strings.TrimSpace(items[1])
		case strings.Contains(part, ":"):
			items := strings.SplitN(part, ":", 2)
			key = strings.TrimSpace(items[0])
			value = strings.TrimSpace(items[1])
		default:
			continue
		}

		score, err := strconv.Atoi(value)
		if err != nil {
			continue
		}
		if key != "" {
			out[key] = score
		}
	}

	return out
}
