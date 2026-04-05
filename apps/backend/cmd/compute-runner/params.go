package main

import (
	"encoding/json"
	"strconv"
	"strings"
)

func getString(m map[string]any, key string) string {
	v, ok := m[key]
	if !ok {
		return ""
	}
	s, ok := v.(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(s)
}

func getInt32(m map[string]any, key string) (int32, bool) {
	v, ok := m[key]
	if !ok {
		return 0, false
	}
	switch t := v.(type) {
	case float64:
		return int32(t), true
	case string:
		n, err := strconv.Atoi(strings.TrimSpace(t))
		if err != nil {
			return 0, false
		}
		return int32(n), true
	default:
		return 0, false
	}
}

func parseParams(raw json.RawMessage) map[string]any {
	if len(raw) == 0 || string(raw) == "null" {
		return map[string]any{}
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		return map[string]any{}
	}
	if m == nil {
		return map[string]any{}
	}
	return m
}

func normalizeBallotFormat(s string) string {
	v := strings.ToLower(strings.TrimSpace(s))
	v = strings.ReplaceAll(v, "_", "-")
	switch v {
	case "ranking":
		return "ranking"
	case "approval":
		return "approval"
	case "score":
		return "score"
	default:
		return ""
	}
}

func normalizeComputeTallyRule(s string) string {
	v := strings.ToLower(strings.TrimSpace(s))
	v = strings.ReplaceAll(v, "_", "-")

	switch v {
	case "plurality",
		"borda",
		"black",
		"copeland-i",
		"copeland-ii",
		"copeland-iii",
		"simpson",
		"hare",
		"nanson",
		"coombs",
		"inverse-borda",
		"inverse-plurality",
		"approval-2",
		"approval-3":
		return v
	case "anti-plurality":
		return "inverse-plurality"
	case "minimax", "minmax":
		return "minmax"
	default:
		return ""
	}
}
