package aggregator

import (
	"testing"
	"time"

	"github.com/BrendanMoorehead/data-flow/internal/canonical"
	"github.com/BrendanMoorehead/data-flow/internal/odds"
)

func TestSnapshotFromGamesMapsOutcomesToCanonicalSides(t *testing.T) {
	updatedAt := time.Date(2026, 9, 28, 18, 0, 0, 0, time.UTC)
	game := gameJSON{
		ID:       "agg-1001",
		Sport:    sportNBA,
		HomeTeam: "Los Angeles Lakers",
		AwayTeam: "Boston Celtics",
		Bookmakers: []bookmakerJSON{{
			Key:        "DraftKings",
			LastUpdate: updatedAt,
			Markets: []marketJSON{{
				Key: marketKeyMoneyline,
				Outcomes: []outcomeJSON{
					{Name: "Boston Celtics", Price: 1.91},
					{Name: "Los Angeles Lakers", Price: 2.00},
				},
			}},
		}},
	}

	snapshot := snapshotFromGames([]gameJSON{game})

	if len(snapshot.Rejections) != 0 {
		t.Fatalf("unexpected rejections: %v", snapshot.Rejections)
	}
	wantPriceBySide := map[canonical.Side]odds.Decimal{canonical.SideAway: 1.91, canonical.SideHome: 2.00}
	for _, observation := range snapshot.Observations {
		if observation.Price != wantPriceBySide[observation.Side] {
			t.Errorf("%s price = %v, want %v", observation.Side, observation.Price, wantPriceBySide[observation.Side])
		}
		if !observation.ObservedAt.Equal(updatedAt) {
			t.Errorf("observed at = %v, want bookmaker last_update %v", observation.ObservedAt, updatedAt)
		}
	}
	if len(snapshot.Observations) != 2 {
		t.Fatalf("got %d observations, want 2", len(snapshot.Observations))
	}
}

func TestSnapshotFromGamesRejectsUnknownSportsbook(t *testing.T) {
	game := gameJSON{
		ID:         "agg-1001",
		Sport:      sportNBA,
		HomeTeam:   "Los Angeles Lakers",
		AwayTeam:   "Boston Celtics",
		Bookmakers: []bookmakerJSON{{Key: "Unknown Book"}},
	}

	snapshot := snapshotFromGames([]gameJSON{game})

	if len(snapshot.Rejections) != 1 {
		t.Fatalf("got %d rejections, want 1", len(snapshot.Rejections))
	}
}
