package resolve

import (
	"slices"
	"time"

	"github.com/BrendanMoorehead/data-flow/internal/canonical"
	"github.com/BrendanMoorehead/data-flow/internal/source"
)

var priorityByKind = map[source.Kind]int{
	source.KindDirect:     0,
	source.KindAggregator: 1,
}

type Candidate struct {
	canonical.SourcePrice
	Config source.Config
}

func (c Candidate) IsFresh(now time.Time) bool {
	return c.Config.IsFresh(c.LastConfirmedAt, now)
}

func SelectPreferred(candidates []Candidate, now time.Time) (Candidate, bool) {
	if len(candidates) == 0 {
		return Candidate{}, false
	}
	fresh := freshCandidates(candidates, now)
	if len(fresh) > 0 {
		return highestPriority(fresh), true
	}
	return mostRecentlyConfirmed(candidates), true
}

func freshCandidates(candidates []Candidate, now time.Time) []Candidate {
	var fresh []Candidate
	for _, candidate := range candidates {
		if candidate.IsFresh(now) {
			fresh = append(fresh, candidate)
		}
	}
	return fresh
}

func highestPriority(candidates []Candidate) Candidate {
	return slices.MinFunc(candidates, func(a, b Candidate) int {
		if priorityDifference := priorityByKind[a.Config.Kind] - priorityByKind[b.Config.Kind]; priorityDifference != 0 {
			return priorityDifference
		}
		return b.LastConfirmedAt.Compare(a.LastConfirmedAt)
	})
}

func mostRecentlyConfirmed(candidates []Candidate) Candidate {
	return slices.MaxFunc(candidates, func(a, b Candidate) int {
		return a.LastConfirmedAt.Compare(b.LastConfirmedAt)
	})
}
