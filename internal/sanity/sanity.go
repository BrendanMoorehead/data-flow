package sanity

import (
	"github.com/BrendanMoorehead/data-flow/internal/canonical"
	"github.com/BrendanMoorehead/data-flow/internal/provider"
)

const (
	minBookMargin = 1.00
	maxBookMargin = 1.15
)

type marketKey struct {
	source          canonical.SourceID
	book            canonical.BookID
	providerEventID string
	market          canonical.MarketType
}

func keyOf(observation canonical.Observation) marketKey {
	return marketKey{
		source:          observation.Source,
		book:            observation.Book,
		providerEventID: observation.Event.ProviderEventID,
		market:          observation.Market,
	}
}

func CheckMargins(observations []canonical.Observation) ([]canonical.Observation, []provider.Rejection) {
	implausible := implausibleMarkets(groupByMarket(observations))
	var accepted []canonical.Observation
	var rejected []provider.Rejection
	for _, observation := range observations {
		if margin, isImplausible := implausible[keyOf(observation)]; isImplausible {
			rejected = append(rejected, provider.NewRejection(observation,
				"event %s %s: book margin %.3f outside %.2f-%.2f",
				observation.Event.ProviderEventID, observation.Book, margin, minBookMargin, maxBookMargin))
			continue
		}
		accepted = append(accepted, observation)
	}
	return accepted, rejected
}

func groupByMarket(observations []canonical.Observation) map[marketKey][]canonical.Observation {
	grouped := make(map[marketKey][]canonical.Observation)
	for _, observation := range observations {
		grouped[keyOf(observation)] = append(grouped[keyOf(observation)], observation)
	}
	return grouped
}

func implausibleMarkets(markets map[marketKey][]canonical.Observation) map[marketKey]float64 {
	implausible := make(map[marketKey]float64)
	for key, sides := range markets {
		margin, bothSidesOpen := openMarketMargin(sides)
		if bothSidesOpen && (margin < minBookMargin || margin > maxBookMargin) {
			implausible[key] = margin
		}
	}
	return implausible
}

func openMarketMargin(observations []canonical.Observation) (float64, bool) {
	probabilityBySide := make(map[canonical.Side]float64)
	for _, observation := range observations {
		if observation.Status == canonical.StatusOpen {
			probabilityBySide[observation.Side] = observation.Price.ImpliedProbability()
		}
	}
	if len(probabilityBySide) < 2 {
		return 0, false
	}
	margin := 0.0
	for _, probability := range probabilityBySide {
		margin += probability
	}
	return margin, true
}
