package datasets

import "errors"

func ballotsToJSON(in []BallotDoc) []map[string]any {
	out := make([]map[string]any, 0, len(in))
	for _, b := range in {
		m := map[string]any{
			"voter_ref": b.VoterRef,
		}
		if len(b.Approval) > 0 {
			m["approval"] = b.Approval
		}
		if len(b.Ranking) > 0 {
			m["ranking"] = b.Ranking
		}
		if len(b.Scores) > 0 {
			m["scores"] = b.Scores
		}
		out = append(out, m)
	}
	return out
}
func toAny[T any](in []T) []any {
	out := make([]any, 0, len(in))
	for i := range in {
		out = append(out, in[i])
	}
	return out
}

type lcg struct{ s uint64 }

func (r *lcg) next() uint64 {
	r.s = r.s*1664525 + 1013904223
	return r.s
}
func shuffle(r *lcg, in []string) []string {
	out := make([]string, len(in))
	copy(out, in)
	for i := len(out) - 1; i > 0; i-- {
		j := int(r.next() % uint64(i+1))
		out[i], out[j] = out[j], out[i]
	}
	return out
}
func pickSubset(r *lcg, in []string, k int) []string {
	if k <= 0 {
		return []string{}
	}
	if k >= len(in) {
		out := make([]string, len(in))
		copy(out, in)
		return out
	}
	sh := shuffle(r, in)
	return sh[:k]
}
func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	neg := false
	if i < 0 {
		neg = true
		i = -i
	}
	var b [32]byte
	pos := len(b)
	for i > 0 {
		pos--
		b[pos] = byte('0' + (i % 10))
		i /= 10
	}
	if neg {
		pos--
		b[pos] = '-'
	}
	return string(b[pos:])
}

var _ = errors.Is
