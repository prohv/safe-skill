package scanner

import (
	"sort"

	"safeskill/internal/types"
)

func Aggregate(results []Result) ([]types.Signal, int) {
	seen := make(map[string]types.Signal)
	for _, r := range results {
		for _, s := range r.Signals {
			key := s.Rule + ":" + s.Message
			seen[key] = s
		}
	}

	signals := make([]types.Signal, 0, len(seen))
	var score int
	for _, s := range seen {
		signals = append(signals, s)
		score += s.Severity
	}

	sort.Slice(signals, func(i, j int) bool {
		return signals[i].Severity > signals[j].Severity
	})

	return signals, score
}
