package tally

import "sort"

func topK(score map[string]int, k int) []string {
	type pair struct {
		ID    string
		Score int
	}

	items := make([]pair, 0, len(score))
	for id, sc := range score {
		items = append(items, pair{ID: id, Score: sc})
	}

	sort.Slice(items, func(i, j int) bool {
		if items[i].Score != items[j].Score {
			return items[i].Score > items[j].Score
		}
		return items[i].ID < items[j].ID
	})

	if k <= 0 {
		k = 1
	}
	if k > len(items) {
		k = len(items)
	}

	out := make([]string, 0, k)
	for i := 0; i < k; i++ {
		out = append(out, items[i].ID)
	}
	return out
}
