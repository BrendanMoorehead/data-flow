package syncer

import (
	"context"
	"testing"
	"time"

	"github.com/BrendanMoorehead/data-flow/internal/canonical"
	"github.com/BrendanMoorehead/data-flow/internal/identity"
	"github.com/BrendanMoorehead/data-flow/internal/odds"
	"github.com/BrendanMoorehead/data-flow/internal/provider"
)

var aggregatorSlice = sliceRef{source: canonical.SourceAggregator, slice: "basketball_nba"}

func lakersCelticsMoneyline(side canonical.Side, price odds.American, observedAt time.Time) canonical.Observation {
	return canonical.Observation{
		Source: canonical.SourceAggregator,
		Book:   canonical.BookFanDuel,
		Event: canonical.ProviderEvent{
			ProviderEventID: "agg-1001",
			League:          "NBA",
			HomeTeam:        "Los Angeles Lakers",
			AwayTeam:        "Boston Celtics",
			StartsAt:        time.Date(2026, 9, 28, 23, 0, 0, 0, time.UTC),
		},
		Market:             canonical.MarketMoneyline,
		Side:               side,
		Status:             canonical.StatusOpen,
		Price:              price.Decimal(),
		ObservedAt:         observedAt,
		HasSourceTimestamp: true,
	}
}

func completeSnapshot(observations ...canonical.Observation) provider.Snapshot {
	return provider.Snapshot{Observations: observations, MissingMeansOffBoard: true}
}

func newSeededSyncer(t *testing.T) *Syncer {
	t.Helper()
	testSyncer := newTestSyncer(t)
	seed, err := identity.LoadDefaultSeed()
	if err != nil {
		t.Fatalf("load seed: %v", err)
	}
	if err := testSyncer.store.SeedTeams(context.Background(), seed.Teams, seed.Aliases); err != nil {
		t.Fatalf("seed teams: %v", err)
	}
	return testSyncer
}

func latestStatusBySide(t *testing.T, testSyncer *Syncer) map[canonical.Side]canonical.MarketStatus {
	t.Helper()
	latest, err := testSyncer.store.LatestSourcePrices(context.Background())
	if err != nil {
		t.Fatalf("latest source prices: %v", err)
	}
	statuses := make(map[canonical.Side]canonical.MarketStatus)
	for _, price := range latest {
		statuses[price.Key.Side] = price.Status
	}
	return statuses
}

func TestMarketMissingFromCompleteSnapshotGoesOffTheBoard(t *testing.T) {
	ctx := context.Background()
	testSyncer := newSeededSyncer(t)
	observedAt := time.Now().Add(-time.Minute)
	testSyncer.ingest(ctx, aggregatorSlice, completeSnapshot(
		lakersCelticsMoneyline(canonical.SideHome, -110, observedAt),
		lakersCelticsMoneyline(canonical.SideAway, -110, observedAt),
	))

	counts, err := testSyncer.ingest(ctx, aggregatorSlice, completeSnapshot(
		lakersCelticsMoneyline(canonical.SideAway, -110, observedAt),
	))

	if err != nil {
		t.Fatalf("ingest: %v", err)
	}
	statuses := latestStatusBySide(t, testSyncer)
	if statuses[canonical.SideHome] != canonical.StatusOffBoard || statuses[canonical.SideAway] != canonical.StatusOpen {
		t.Errorf("statuses = %v, want the missing home side off the board", statuses)
	}
	if counts.offBoardInferred != 1 {
		t.Errorf("inferred %d off-the-board markets, want 1", counts.offBoardInferred)
	}
}

func TestStillMissingMarketIsConfirmedNotReinferred(t *testing.T) {
	ctx := context.Background()
	testSyncer := newSeededSyncer(t)
	observedAt := time.Now().Add(-time.Minute)
	away := lakersCelticsMoneyline(canonical.SideAway, -110, observedAt)
	testSyncer.ingest(ctx, aggregatorSlice, completeSnapshot(lakersCelticsMoneyline(canonical.SideHome, -110, observedAt), away))
	testSyncer.ingest(ctx, aggregatorSlice, completeSnapshot(away))

	counts, _ := testSyncer.ingest(ctx, aggregatorSlice, completeSnapshot(away))

	if counts.offBoardInferred != 0 {
		t.Errorf("a market already off the board was inferred again %d times; want it only confirmed", counts.offBoardInferred)
	}
}

func TestIncompleteSnapshotsNeverInferOffBoard(t *testing.T) {
	ctx := context.Background()
	testSyncer := newSeededSyncer(t)
	observedAt := time.Now().Add(-time.Minute)
	testSyncer.ingest(ctx, aggregatorSlice, completeSnapshot(
		lakersCelticsMoneyline(canonical.SideHome, -110, observedAt),
		lakersCelticsMoneyline(canonical.SideAway, -110, observedAt),
	))

	testSyncer.ingest(ctx, aggregatorSlice, provider.Snapshot{})

	if statuses := latestStatusBySide(t, testSyncer); statuses[canonical.SideHome] != canonical.StatusOpen {
		t.Errorf("home side = %s after a snapshot that doesn't claim completeness, want open", statuses[canonical.SideHome])
	}
}

func TestProviderWithHomeAndAwayReversedJoinsTheSameGameWithSidesFlipped(t *testing.T) {
	ctx := context.Background()
	testSyncer := newSeededSyncer(t)
	observedAt := time.Now().Add(-time.Minute)
	testSyncer.ingest(ctx, aggregatorSlice, completeSnapshot(lakersCelticsMoneyline(canonical.SideHome, -150, observedAt)))
	reversed := lakersCelticsMoneyline(canonical.SideAway, -150, observedAt)
	reversed.Source = canonical.SourceDraftKingsDirect
	reversed.Event = canonical.ProviderEvent{
		ProviderEventID: "dk-5501",
		League:          "NBA",
		HomeTeam:        "BOS Celtics",
		AwayTeam:        "LA Lakers",
		StartsAt:        reversed.Event.StartsAt,
	}

	testSyncer.ingest(ctx, sliceRef{source: canonical.SourceDraftKingsDirect, slice: "dk-5501"}, provider.Snapshot{Observations: []canonical.Observation{reversed}})

	events, _ := testSyncer.store.ListEvents(ctx)
	if len(events) != 1 {
		t.Fatalf("got %d events, want the reversed listing to join the existing game", len(events))
	}
	latest, _ := testSyncer.store.LatestSourcePrices(ctx)
	for _, price := range latest {
		if price.Source == canonical.SourceDraftKingsDirect && price.Key.Side != canonical.SideHome {
			t.Errorf("draftkings' Lakers price landed on %s, want home (the Lakers are home in our event)", price.Key.Side)
		}
	}
}
