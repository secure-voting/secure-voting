package elections

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"path/filepath"
	"strings"
)

func importCandidates(filename string, content []byte) ([]CandidateNormalized, string, error) {
	name := strings.TrimSpace(filename)
	if name == "" {
		return nil, "invalid_file", nil
	}

	data := stripUTF8BOM(content)
	if len(bytes.TrimSpace(data)) == 0 {
		return nil, "invalid_file", nil
	}

	switch strings.ToLower(filepath.Ext(name)) {
	case ".csv":
		items, code, err := parseCandidateCSV(data)
		if err != nil || code != "" {
			return nil, code, err
		}

		normalized, err := normalizeCandidatesFromObjects(items)
		if err != nil {
			return nil, candidateNormalizationCode(err), nil
		}
		return normalized, "", nil

	case ".json":
		return parseCandidateJSON(data)

	default:
		return nil, "unsupported_file_format", nil
	}
}

func parseCandidateJSON(data []byte) ([]CandidateNormalized, string, error) {
	var wrapped struct {
		Items []CandidateInput `json:"items"`
	}
	if err := json.Unmarshal(data, &wrapped); err == nil && len(wrapped.Items) > 0 {
		normalized, err := normalizeCandidatesFromObjects(wrapped.Items)
		if err != nil {
			return nil, candidateNormalizationCode(err), nil
		}
		return normalized, "", nil
	}

	var objects []CandidateInput
	if err := json.Unmarshal(data, &objects); err == nil && len(objects) > 0 {
		normalized, err := normalizeCandidatesFromObjects(objects)
		if err != nil {
			return nil, candidateNormalizationCode(err), nil
		}
		return normalized, "", nil
	}

	var names []string
	if err := json.Unmarshal(data, &names); err == nil && len(names) > 0 {
		normalized, err := normalizeCandidatesFromNames(names)
		if err != nil {
			return nil, candidateNormalizationCode(err), nil
		}
		return normalized, "", nil
	}

	return nil, "invalid_file", nil
}

func parseCandidateCSV(data []byte) ([]CandidateInput, string, error) {
	r := csv.NewReader(bytes.NewReader(data))
	r.FieldsPerRecord = -1
	r.TrimLeadingSpace = true

	rows, err := r.ReadAll()
	if err != nil {
		return nil, "invalid_file", nil
	}
	if len(rows) == 0 {
		return nil, "invalid_candidates", nil
	}

	startRow := 0
	nameIdx := 0
	descIdx := -1

	if idx, ok := candidateCSVColumnIndex(rows[0], "name", "candidate", "candidate_name", "fio", "full_name", "имя", "фио"); ok {
		startRow = 1
		nameIdx = idx
		if idx, ok := candidateCSVColumnIndex(rows[0], "description", "desc", "bio", "описание"); ok {
			descIdx = idx
		}
	}

	items := make([]CandidateInput, 0, len(rows))
	for i := startRow; i < len(rows); i++ {
		row := rows[i]
		if isCandidateCSVRowEmpty(row) {
			continue
		}

		if nameIdx >= len(row) {
			return nil, "invalid_candidates", nil
		}

		item := CandidateInput{
			Name: row[nameIdx],
		}

		if descIdx >= 0 {
			if descIdx < len(row) {
				desc := strings.TrimSpace(row[descIdx])
				if desc != "" {
					item.Description = &desc
				}
			}
		} else if startRow == 0 && len(row) > 1 {
			desc := strings.TrimSpace(row[1])
			if desc != "" {
				item.Description = &desc
			}
		}

		items = append(items, item)
	}

	if len(items) == 0 {
		return nil, "invalid_candidates", nil
	}

	return items, "", nil
}

func candidateCSVColumnIndex(header []string, names ...string) (int, bool) {
	want := make(map[string]struct{}, len(names))
	for _, name := range names {
		want[normalizeCandidateCSVHeader(name)] = struct{}{}
	}

	for i, col := range header {
		if _, ok := want[normalizeCandidateCSVHeader(col)]; ok {
			return i, true
		}
	}

	return -1, false
}

func normalizeCandidateCSVHeader(v string) string {
	s := strings.ToLower(strings.TrimSpace(v))
	s = strings.ReplaceAll(s, "-", "_")
	s = strings.ReplaceAll(s, " ", "_")
	return s
}

func isCandidateCSVRowEmpty(row []string) bool {
	for _, col := range row {
		if strings.TrimSpace(col) != "" {
			return false
		}
	}
	return true
}

func stripUTF8BOM(data []byte) []byte {
	return bytes.TrimPrefix(data, []byte{0xEF, 0xBB, 0xBF})
}
