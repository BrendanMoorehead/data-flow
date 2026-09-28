package resolve

import (
	"testing"
	"time"

	"github.com/BrendanMoorehead/data-flow/internal/canonical"
	"github.com/BrendanMoorehead/data-flow/internal/source"
)

var (
	testNow          = time.Date(2026, 9, 28, 18, 0, 0, 0, time.UTC)
	directConfig     = source.Config{ID: canonical.SourceDraftKingsDirect, Kind: source.KindDirect, StaleAfter: 10 * time.Second}
	aggregatorConfig = source.Config{ID: canonical.SourceAggregator, Kind: source.KindAggregator, StaleAfter: 30 * time.Second}
)

func candidate(config source.Config, confirmedAgo time.Duration) Candidate {
	return Candidate{
		SourcePrice: canonical.SourcePrice{Source: config.ID, LastConfirmedAt: testNow.Add(-confirmedAgo)},
		Config:      config,
	}
}

func TestSelectPreferredChoosesFreshDirectOverFreshAggregator(t *testing.T) {
	direct := candidate(directConfig, 5*time.Second)
	aggregator := candidate(aggregatorConfig, 1*time.Second)

	preferred, _ := SelectPreferred([]Candidate{aggregator, direct}, testNow)

	if preferred.Source != canonical.SourceDraftKingsDirect {
		t.Errorf("preferred source = %s, want the fresh direct feed", preferred.Source)
	}
}

func TestSelectPreferredFallsBackToAggregatorWhenDirectIsStale(t *testing.T) {
	direct := candidate(directConfig, 20*time.Second)
	aggregator := candidate(aggregatorConfig, 5*time.Second)

	preferred, _ := SelectPreferred([]Candidate{direct, aggregator}, testNow)

	if preferred.Source != canonical.SourceAggregator {
		t.Errorf("preferred source = %s, want the fresh aggregator", preferred.Source)
	}
}

func TestSelectPreferredUsesMostRecentlyConfirmedWhenAllAreStale(t *testing.T) {
	direct := candidate(directConfig, 5*time.Minute)
	aggregator := candidate(aggregatorConfig, 2*time.Minute)

	preferred, _ := SelectPreferred([]Candidate{direct, aggregator}, testNow)

	if preferred.Source != canonical.SourceAggregator {
		t.Errorf("preferred source = %s, want the most recently confirmed source", preferred.Source)
	}
}

func TestSelectPreferredReportsNothingWithoutCandidates(t *testing.T) {
	if _, found := SelectPreferred(nil, testNow); found {
		t.Error("SelectPreferred(nil) found a candidate")
	}
}
