package resolve

import (
	"time"

	"github.com/BrendanMoorehead/data-flow/internal/canonical"
)

func Resolve(candidates []Candidate, now time.Time) (canonical.ResolvedPrice, bool) {
	if len(candidates) == 0 {
		return canonical.ResolvedPrice{}, false
	}
	pool := freshCandidates(candidates, now)
	allStale := len(pool) == 0
	if allStale {
		pool = candidates
	}
	served := servedCandidate(pool, now)
	confidence := assessConfidence(confidenceInputs{served: served, pool: pool, candidates: candidates, allStale: allStale})
	return resolvedPriceFrom(served, confidence), true
}

func servedCandidate(pool []Candidate, now time.Time) Candidate {
	statusSetter := mostRecentStatusSetter(pool)
	if statusSetter.Status == canonical.StatusOffBoard {
		return statusSetter
	}
	preferred, _ := SelectPreferred(openCandidates(pool), now)
	return preferred
}

func resolvedPriceFrom(served Candidate, confidence canonical.Confidence) canonical.ResolvedPrice {
	return canonical.ResolvedPrice{
		Key:             served.Key,
		Status:          served.Status,
		Price:           served.Price,
		Source:          served.Source,
		ObservedAt:      served.ObservedAt,
		LastConfirmedAt: served.LastConfirmedAt,
		Confidence:      confidence,
	}
}
