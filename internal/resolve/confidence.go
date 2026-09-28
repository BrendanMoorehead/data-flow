package resolve

import (
	"math"

	"github.com/BrendanMoorehead/data-flow/internal/canonical"
	"github.com/BrendanMoorehead/data-flow/internal/source"
)

const (
	agreementTolerance     = 0.015
	matchPrecisionDecimals = 2
)

var rankByLevel = map[canonical.ConfidenceLevel]int{
	canonical.ConfidenceLow:      0,
	canonical.ConfidenceMedium:   1,
	canonical.ConfidenceHigh:     2,
	canonical.ConfidenceVerified: 3,
}

type confidenceInputs struct {
	served     Candidate
	pool       []Candidate
	candidates []Candidate
	allStale   bool
}

func assessConfidence(inputs confidenceInputs) canonical.Confidence {
	confidence := levelFromSources(inputs)
	confidence.Reasons = append(directReasons(inputs), confidence.Reasons...)
	if !inputs.served.HasSourceTimestamp {
		confidence = capAtMedium(confidence)
		confidence.Reasons = append(confidence.Reasons, canonical.ReasonNoSourceTimestamp)
	}
	return confidence
}

func levelFromSources(inputs confidenceInputs) canonical.Confidence {
	if inputs.allStale {
		return confidenceOf(canonical.ConfidenceLow, canonical.ReasonAllSourcesStale)
	}
	second, found := secondSource(inputs.served, inputs.pool)
	if !found {
		return confidenceOf(canonical.ConfidenceMedium, canonical.ReasonNoSecondSource)
	}
	return compareSources(inputs.served, second)
}

func compareSources(served, second Candidate) canonical.Confidence {
	if served.Status != canonical.StatusOpen || second.Status != canonical.StatusOpen {
		if served.Status == second.Status {
			return confidenceOf(canonical.ConfidenceVerified, canonical.ReasonSourcesMatch)
		}
		return confidenceOf(canonical.ConfidenceLow, canonical.ReasonSourcesDisagree)
	}
	switch {
	case pricesMatch(served, second):
		return confidenceOf(canonical.ConfidenceVerified, canonical.ReasonSourcesMatch)
	case pricesAgree(served, second):
		return confidenceOf(canonical.ConfidenceHigh, canonical.ReasonSourcesAgree)
	default:
		return confidenceOf(canonical.ConfidenceLow, canonical.ReasonSourcesDisagree)
	}
}

func secondSource(served Candidate, pool []Candidate) (Candidate, bool) {
	var others []Candidate
	for _, candidate := range pool {
		if candidate.Source != served.Source {
			others = append(others, candidate)
		}
	}
	if len(others) == 0 {
		return Candidate{}, false
	}
	return highestPriority(others), true
}

func directReasons(inputs confidenceInputs) []canonical.ConfidenceReason {
	if inputs.served.Config.Kind == source.KindDirect && !inputs.allStale {
		return []canonical.ConfidenceReason{canonical.ReasonDirectFresh}
	}
	for _, candidate := range inputs.candidates {
		if candidate.Config.Kind == source.KindDirect {
			return []canonical.ConfidenceReason{canonical.ReasonDirectStale}
		}
	}
	return nil
}

func pricesMatch(a, b Candidate) bool {
	return roundToDecimals(float64(a.Price), matchPrecisionDecimals) == roundToDecimals(float64(b.Price), matchPrecisionDecimals)
}

func pricesAgree(a, b Candidate) bool {
	return math.Abs(a.Price.ImpliedProbability()-b.Price.ImpliedProbability()) <= agreementTolerance
}

func capAtMedium(confidence canonical.Confidence) canonical.Confidence {
	if rankByLevel[confidence.Level] > rankByLevel[canonical.ConfidenceMedium] {
		confidence.Level = canonical.ConfidenceMedium
	}
	return confidence
}

func confidenceOf(level canonical.ConfidenceLevel, reason canonical.ConfidenceReason) canonical.Confidence {
	return canonical.Confidence{Level: level, Reasons: []canonical.ConfidenceReason{reason}}
}

func roundToDecimals(value float64, decimals int) float64 {
	scale := math.Pow(10, float64(decimals))
	return math.Round(value*scale) / scale
}
