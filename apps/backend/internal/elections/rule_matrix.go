package elections

import "secure-voting/apps/backend/internal/computeclient"

type ruleMatrix map[string]computeclient.TallyRuleInfo

func buildRuleMatrix(rules []computeclient.TallyRuleInfo) ruleMatrix {
	m := make(ruleMatrix, len(rules))
	for _, r := range rules {
		m[r.ID] = r
	}
	return m
}

func (m ruleMatrix) get(rule string) (computeclient.TallyRuleInfo, bool) {
	r, ok := m[rule]
	return r, ok
}