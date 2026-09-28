package syncer_test

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/BrendanMoorehead/data-flow/internal/canonical"
	"github.com/BrendanMoorehead/data-flow/internal/identity"
	"github.com/BrendanMoorehead/data-flow/internal/provider"
	"github.com/BrendanMoorehead/data-flow/internal/provider/aggregator"
	"github.com/BrendanMoorehead/data-flow/internal/provider/draftkings"
	"github.com/BrendanMoorehead/data-flow/internal/resolve"
	"github.com/BrendanMoorehead/data-flow/internal/sim"
	"github.com/BrendanMoorehead/data-flow/internal/source"
	"github.com/BrendanMoorehead/data-flow/internal/store"
	"github.com/BrendanMoorehead/data-flow/internal/syncer"
)

const simulatedGameCount = 6

var testSources = source.NewRegistry(
	source.Config{ID: canonical.SourceDraftKingsDirect, Kind: source.KindDirect, PollInterval: time.Second, StaleAfter: time.Minute},
	source.Config{ID: canonical.SourceAggregator, Kind: source.KindAggregator, PollInterval: time.Second, StaleAfter: time.Minute},
)

type pipeline struct {
	store     *store.Store
	syncer    *syncer.Syncer
	rebuilder *resolve.Rebuilder
	providers []provider.Provider
}

func newPipeline(t *testing.T) pipeline {
	t.Helper()
	ctx := context.Background()
	simulator := sim.New(sim.Options{Seed: 7, BackgroundProblems: false, Now: time.Now})
	aggregatorServer := httptest.NewServer(simulator.AggregatorHandler())
	draftKingsServer := httptest.NewServer(simulator.DraftKingsHandler())
	t.Cleanup(aggregatorServer.Close)
	t.Cleanup(draftKingsServer.Close)

	testStore, err := store.Open(ctx, filepath.Join(t.TempDir(), "pipeline.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { testStore.Close() })
	if err := testStore.SeedTeams(ctx, identity.SeedTeams(), identity.SeedAliases()); err != nil {
		t.Fatalf("seed teams: %v", err)
	}

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	providers := []provider.Provider{
		aggregator.New(aggregatorServer.URL, http.DefaultClient),
		draftkings.New(draftKingsServer.URL, http.DefaultClient),
	}
	return pipeline{
		store: testStore,
		syncer: syncer.New(syncer.Dependencies{
			Providers: providers,
			Sources:   testSources,
			Events:    identity.NewResolver(testStore),
			Store:     testStore,
			Logger:    logger,
			Now:       time.Now,
		}),
		rebuilder: resolve.NewRebuilder(testStore, testSources, logger, time.Now),
		providers: providers,
	}
}

func (p pipeline) pollEveryProvider(t *testing.T) {
	t.Helper()
	for _, provider := range p.providers {
		p.syncer.PollProviderOnce(context.Background(), provider)
	}
	if err := p.rebuilder.Rebuild(context.Background()); err != nil {
		t.Fatalf("rebuild: %v", err)
	}
}

func TestProvidersWithDifferentIdentifiersMergeIntoOneEventPerGame(t *testing.T) {
	testPipeline := newPipeline(t)

	testPipeline.pollEveryProvider(t)

	events, err := testPipeline.store.ListEvents(context.Background())
	if err != nil {
		t.Fatalf("list events: %v", err)
	}
	if len(events) != simulatedGameCount {
		t.Errorf("got %d canonical events, want %d (one per game across both providers)", len(events), simulatedGameCount)
	}
}

func TestDirectFeedWinsForItsBookAndAggregatorFillsTheOther(t *testing.T) {
	testPipeline := newPipeline(t)

	testPipeline.pollEveryProvider(t)

	events, _ := testPipeline.store.ListEvents(context.Background())
	prices, err := testPipeline.store.ResolvedPricesForEvent(context.Background(), events[0].ID)
	if err != nil {
		t.Fatalf("resolved prices: %v", err)
	}
	wantSourceByBook := map[canonical.BookID]canonical.SourceID{
		canonical.BookDraftKings: canonical.SourceDraftKingsDirect,
		canonical.BookFanDuel:    canonical.SourceAggregator,
	}
	for _, price := range prices {
		if price.Source != wantSourceByBook[price.Key.Book] {
			t.Errorf("%s %s came from %s, want %s", price.Key.Book, price.Key.Side, price.Source, wantSourceByBook[price.Key.Book])
		}
	}
	if len(prices) != 4 {
		t.Errorf("got %d resolved prices, want 4 (two books, two sides)", len(prices))
	}
}

func TestRepeatedPollsDoNotDuplicateObservations(t *testing.T) {
	testPipeline := newPipeline(t)

	testPipeline.pollEveryProvider(t)
	firstPass, _ := testPipeline.store.LatestSourcePrices(context.Background())
	testPipeline.pollEveryProvider(t)
	secondPass, _ := testPipeline.store.LatestSourcePrices(context.Background())

	if len(firstPass) != len(secondPass) {
		t.Errorf("latest prices went from %d to %d after an identical poll", len(firstPass), len(secondPass))
	}
}
