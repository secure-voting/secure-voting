package tally

func computeApproval(committeeSize int, candidates []string, ballots [][]string) Output {
	score := make(map[string]int, len(candidates))
	for _, cid := range candidates {
		score[cid] = 0
	}

	for _, b := range ballots {
		for _, cid := range b {
			if _, ok := score[cid]; ok {
				score[cid]++
			}
		}
	}

	winners := topK(score, committeeSize)
	return Output{
		Winners: winners,
		Metrics: map[string]any{
			"ballots": len(ballots),
		},
		Protocol: map[string]any{
			"scores": score,
		},
	}
}

func computePlurality(committeeSize int, candidates []string, ballots [][]string) Output {
	score := make(map[string]int, len(candidates))
	for _, cid := range candidates {
		score[cid] = 0
	}

	for _, b := range ballots {
		if len(b) == 0 {
			continue
		}
		first := b[0]
		if _, ok := score[first]; ok {
			score[first]++
		}
	}

	winners := topK(score, committeeSize)
	return Output{
		Winners: winners,
		Metrics: map[string]any{
			"ballots": len(ballots),
		},
		Protocol: map[string]any{
			"scores": score,
		},
	}
}

func computeBorda(committeeSize int, candidates []string, ballots [][]string) Output {
	n := len(candidates)
	score := make(map[string]int, n)
	for _, cid := range candidates {
		score[cid] = 0
	}

	for _, b := range ballots {
		for i, cid := range b {
			if _, ok := score[cid]; !ok {
				continue
			}
			points := (n - 1) - i
			if points < 0 {
				points = 0
			}
			score[cid] += points
		}
	}

	winners := topK(score, committeeSize)
	return Output{
		Winners: winners,
		Metrics: map[string]any{
			"ballots": len(ballots),
		},
		Protocol: map[string]any{
			"scores": score,
		},
	}
}
