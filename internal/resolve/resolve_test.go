package resolve

import (
	"slices"
	"testing"
	"time"

	"github.com/BrendanMoorehead/data-flow/internal/canonical"
	"github.com/BrendanMoorehead/data-flow/internal/odds"
	"github.com/BrendanMoorehead/data-flow/internal/source"
)

var fanDuelDirectConfig = source.Config{ID: canonical.SourceFanDuelDirect, Kind: source.KindDirect, StaleAfter: 10 * time.Second}

type candidateSpec struct {
	config       source.Config
	status       canonical.MarketStatus
	price        odds.Decimal
	observedAgo  time.Duration
	confirmedAgo time.Duration
	noTimestamp  bool
}

func buildCandidate(spec candidateSpec) Candidate {
	return Candidate{
		SourcePrice: canonical.SourcePrice{
			Source:             spec.config.ID,
			Status:             spec.status,
			Price:              spec.price,
			ObservedAt:         testNow.Add(-spec.observedAgo),
			LastConfirmedAt:    testNow.Add(-spec.confirmedAgo),
			HasSourceTimestamp: !spec.noTimestamp,
		},
		Config: spec.config,
	}
}

func open(config source.Config, price odds.Decimal) candidateSpec {
	return candidateSpec{config: config, status: canonical.StatusOpen, price: price, observedAgo: 2 * time.Second, confirmedAgo: time.Second}
}

func resolveOrFail(t *testing.T, specs ...candidateSpec) canonical.ResolvedPrice {
	t.Helper()
	var candidates []Candidate
	for _, spec := range specs {
		candidates = append(candidates, buildCandidate(spec))
	}
	resolved, found := Resolve(candidates, testNow)
	if !found {
		t.Fatal("Resolve found nothing")
	}
	return resolved
}

func TestConfidenceIsVerifiedWhenSourcesMatchAtTwoDecimals(t *testing.T) {
	resolved := resolveOrFail(t, open(directConfig, odds.American(-110).Decimal()), open(aggregatorConfig, 1.91))

	if resolved.Confidence.Level != canonical.ConfidenceVerified {
		t.Errorf("level = %s, want verified for -110 against 1.91", resolved.Confidence.Level)
	}
}

func TestConfidenceIsHighWhenSourcesAreClose(t *testing.T) {
	resolved := resolveOrFail(t, open(directConfig, odds.American(-110).Decimal()), open(aggregatorConfig, 1.93))

	if resolved.Confidence.Level != canonical.ConfidenceHigh {
		t.Errorf("level = %s, want high for -110 against 1.93", resolved.Confidence.Level)
	}
}

func TestConfidenceIsLowWhenSourcesDisagree(t *testing.T) {
	resolved := resolveOrFail(t, open(directConfig, odds.American(-110).Decimal()), open(aggregatorConfig, 2.40))

	if resolved.Confidence.Level != canonical.ConfidenceLow || !slices.Contains(resolved.Confidence.Reasons, canonical.ReasonSourcesDisagree) {
		t.Errorf("confidence = %+v, want low with sources_disagree", resolved.Confidence)
	}
}

func TestConfidenceIsMediumWithOnlyOneFreshSource(t *testing.T) {
	stale := open(aggregatorConfig, 1.91)
	stale.confirmedAgo = time.Hour

	resolved := resolveOrFail(t, open(directConfig, 1.91), stale)

	if resolved.Confidence.Level != canonical.ConfidenceMedium {
		t.Errorf("level = %s, want medium when the second source is stale", resolved.Confidence.Level)
	}
}

func TestConfidenceIsLowWhenEverySourceIsStale(t *testing.T) {
	direct := open(directConfig, 1.91)
	direct.confirmedAgo = time.Hour

	resolved := resolveOrFail(t, direct)

	if resolved.Confidence.Level != canonical.ConfidenceLow || !slices.Contains(resolved.Confidence.Reasons, canonical.ReasonAllSourcesStale) {
		t.Errorf("confidence = %+v, want low with all_sources_stale", resolved.Confidence)
	}
}

func TestMissingSourceTimestampCapsConfidenceAtMedium(t *testing.T) {
	fanDuel := open(fanDuelDirectConfig, odds.American(-110).Decimal())
	fanDuel.noTimestamp = true

	resolved := resolveOrFail(t, fanDuel, open(aggregatorConfig, 1.91))

	if resolved.Confidence.Level != canonical.ConfidenceMedium || !slices.Contains(resolved.Confidence.Reasons, canonical.ReasonNoSourceTimestamp) {
		t.Errorf("confidence = %+v, want medium with no_source_timestamp", resolved.Confidence)
	}
}

func TestMostRecentFreshObservationSetsStatus(t *testing.T) {
	directOffBoard := candidateSpec{config: directConfig, status: canonical.StatusOffBoard, price: 1.91, observedAgo: time.Second, confirmedAgo: time.Second}
	aggregatorOpenEarlier := open(aggregatorConfig, 1.91)
	aggregatorOpenEarlier.observedAgo = 5 * time.Second

	resolved := resolveOrFail(t, directOffBoard, aggregatorOpenEarlier)

	if resolved.Status != canonical.StatusOffBoard || resolved.Price != 1.91 {
		t.Errorf("resolved = %s at %v, want off the board with the last shown price 1.91", resolved.Status, resolved.Price)
	}
}

func TestNewerOpenObservationPutsMarketBackOnTheBoard(t *testing.T) {
	aggregatorOffBoard := candidateSpec{config: aggregatorConfig, status: canonical.StatusOffBoard, price: 1.91, observedAgo: 10 * time.Second, confirmedAgo: time.Second}

	resolved := resolveOrFail(t, aggregatorOffBoard, open(directConfig, 1.95))

	if resolved.Status != canonical.StatusOpen || resolved.Source != canonical.SourceDraftKingsDirect {
		t.Errorf("resolved = %s from %s, want open from the newer direct observation", resolved.Status, resolved.Source)
	}
}

func TestSimultaneousObservationsFavorOffBoard(t *testing.T) {
	directOpen := open(directConfig, 1.91)
	aggregatorOffBoard := open(aggregatorConfig, 1.91)
	aggregatorOffBoard.status = canonical.StatusOffBoard

	resolved := resolveOrFail(t, directOpen, aggregatorOffBoard)

	if resolved.Status != canonical.StatusOffBoard {
		t.Errorf("status on a timestamp tie = %s, want off_board", resolved.Status)
	}
}
