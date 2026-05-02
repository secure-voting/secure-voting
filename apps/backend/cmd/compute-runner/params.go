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
	case float32:
		return int32(t), true
	case int:
		return int32(t), true
	case int32:
		return t, true
	case int64:
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

func getBool(m map[string]any, key string) (bool, bool) {
	v, ok := m[key]
	if !ok {
		return false, false
	}
	switch t := v.(type) {
	case bool:
		return t, true
	case string:
		s := strings.ToLower(strings.TrimSpace(t))
		switch s {
		case "1", "true", "yes", "y", "on":
			return true, true
		case "0", "false", "no", "n", "off":
			return false, true
		default:
			return false, false
		}
	default:
		return false, false
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
	case "score", "scoring":
		return "score"
	default:
		return ""
	}
}

func grpcBallotFormatName(format string) string {
	switch normalizeBallotFormat(format) {
	case "ranking":
		return "ranking"
	case "approval":
		return "approval"
	case "score":
		return "scoring"
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
		"approval-3",
		"score",
		"threshold",
		"practical-condorcet",
		"q-paretian-strong-simple-majority",
		"q-paretian-strong-plurality",
		"q-paretian-strongest-simple-majority":
		return v
	case "strong-q-paretian-simple-majority":
		return "q-paretian-strong-simple-majority"
	case "strong-q-paretian-plurality":
		return "q-paretian-strong-plurality"
	case "strongest-q-paretian-simple-majority":
		return "q-paretian-strongest-simple-majority"
	case "anti-plurality":
		return "inverse-plurality"
	case "minimax", "minmax":
		return "minmax"
	case "condorcet-practical":
		return "practical-condorcet"
	default:
		return ""
	}
}

func resolveComputeTallyRule(raw string, approvalMaxChoices *int32) string {
	n := normalizeComputeTallyRule(raw)
	if n != "" {
		return n
	}

	v := strings.ToLower(strings.TrimSpace(raw))
	v = strings.ReplaceAll(v, "_", "-")

	if v == "approval" && approvalMaxChoices != nil {
		switch *approvalMaxChoices {
		case 2:
			return "approval-2"
		case 3:
			return "approval-3"
		}
	}

	return ""
}
