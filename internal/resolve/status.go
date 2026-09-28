package resolve

import (
	"slices"

	"github.com/BrendanMoorehead/data-flow/internal/canonical"
)

func mostRecentStatusSetter(candidates []Candidate) Candidate {
	return slices.MaxFunc(candidates, func(a, b Candidate) int {
		if byTime := a.ObservedAt.Compare(b.ObservedAt); byTime != 0 {
			return byTime
		}
		return offBoardFirst(a) - offBoardFirst(b)
	})
}

func offBoardFirst(candidate Candidate) int {
	if candidate.Status == canonical.StatusOffBoard {
		return 1
	}
	return 0
}

func openCandidates(candidates []Candidate) []Candidate {
	var open []Candidate
	for _, candidate := range candidates {
		if candidate.Status == canonical.StatusOpen {
			open = append(open, candidate)
		}
	}
	return open
}
