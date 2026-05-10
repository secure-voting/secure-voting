package elections

import "secure-voting/apps/backend/internal/computeclient"

type ruleMatrix map[string]computeclient.TallyRuleInfo

func buildRuleMatrix(rules []computeclient.TallyRuleInfo) ruleMatrix {
	m := make(ruleMatrix, len(rules))
	for _, r := range rules {
		id := normalizeRuleName(r.ID)
		if id == "" {
			continue
		}
		m[id] = r
	}
	return m
}

func (m ruleMatrix) get(rule string) (computeclient.TallyRuleInfo, bool) {
	r, ok := m[normalizeRuleName(rule)]
	return r, ok
}
