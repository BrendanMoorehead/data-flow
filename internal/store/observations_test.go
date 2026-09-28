package store

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/BrendanMoorehead/data-flow/internal/canonical"
	"github.com/BrendanMoorehead/data-flow/internal/odds"
)

var (
	testObservedAt = time.Date(2026, 9, 28, 18, 0, 0, 0, time.UTC)
	testMatchup    = canonical.Matchup{Home: "nba-lal", Away: "nba-bos"}
)

func openTestStore(t *testing.T) *Store {
	t.Helper()
	ctx := context.Background()
	testStore, err := Open(ctx, filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { testStore.Close() })

	teams := []canonical.Team{{ID: testMatchup.Home, Name: "Los Angeles Lakers"}, {ID: testMatchup.Away, Name: "Boston Celtics"}}
	if err := testStore.SeedTeams(ctx, teams, nil); err != nil {
		t.Fatalf("seed teams: %v", err)
	}
	return testStore
}

func createTestEvent(t *testing.T, testStore *Store) canonical.EventID {
	t.Helper()
	eventID, err := testStore.CreateEvent(context.Background(), testMatchup, "NBA", testObservedAt)
	if err != nil {
		t.Fatalf("create event: %v", err)
	}
	return eventID
}

func homeMoneyline(price odds.American, observedAt time.Time) canonical.Observation {
	return canonical.Observation{
		Source:     canonical.SourceDraftKingsDirect,
		Book:       canonical.BookDraftKings,
		Market:     canonical.MarketMoneyline,
		Side:       canonical.SideHome,
		Price:      price.Decimal(),
		RawPrice:   price.String(),
		RawFormat:  canonical.FormatAmerican,
		ObservedAt: observedAt,
	}
}

func TestRecordObservationTwiceStoresOnceAndMovesConfirmation(t *testing.T) {
	ctx := context.Background()
	testStore := openTestStore(t)
	eventID := createTestEvent(t, testStore)
	observation := homeMoneyline(-110, testObservedAt)
	firstReceipt := testObservedAt.Add(time.Second)
	secondReceipt := testObservedAt.Add(5 * time.Second)

	firstResult, err := testStore.RecordObservation(ctx, eventID, observation, firstReceipt)
	if err != nil {
		t.Fatalf("first record: %v", err)
	}
	secondResult, err := testStore.RecordObservation(ctx, eventID, observation, secondReceipt)
	if err != nil {
		t.Fatalf("second record: %v", err)
	}

	if firstResult != RecordInserted || secondResult != RecordConfirmed {
		t.Errorf("results = %v, %v; want inserted then confirmed", firstResult, secondResult)
	}
	latest := latestPricesOrFail(t, testStore)
	if len(latest) != 1 {
		t.Fatalf("got %d latest prices, want 1", len(latest))
	}
	if !latest[0].LastConfirmedAt.Equal(secondReceipt) {
		t.Errorf("last confirmed at = %v, want %v", latest[0].LastConfirmedAt, secondReceipt)
	}
}

func TestLatestSourcePricesIgnoresOutOfOrderObservation(t *testing.T) {
	ctx := context.Background()
	testStore := openTestStore(t)
	eventID := createTestEvent(t, testStore)
	newer := homeMoneyline(-120, testObservedAt.Add(10*time.Second))
	older := homeMoneyline(-105, testObservedAt)

	for _, observation := range []canonical.Observation{newer, older} {
		if _, err := testStore.RecordObservation(ctx, eventID, observation, testObservedAt.Add(time.Minute)); err != nil {
			t.Fatalf("record: %v", err)
		}
	}

	latest := latestPricesOrFail(t, testStore)
	if len(latest) != 1 || latest[0].Price.American() != -120 {
		t.Errorf("latest prices = %+v, want only the newer -120 observation", latest)
	}
}

func latestPricesOrFail(t *testing.T, testStore *Store) []canonical.SourcePrice {
	t.Helper()
	latest, err := testStore.LatestSourcePrices(context.Background())
	if err != nil {
		t.Fatalf("latest source prices: %v", err)
	}
	return latest
}
