package api

import (
	"testing"
	"time"

	"github.com/BrendanMoorehead/data-flow/internal/provider"
	"github.com/BrendanMoorehead/data-flow/internal/source"
	"github.com/BrendanMoorehead/data-flow/internal/store"
)

var (
	statusNow    = time.Date(2026, 9, 28, 18, 0, 0, 0, time.UTC)
	statusConfig = source.Config{StaleAfter: 30 * time.Second, CatalogRefreshInterval: time.Minute}
)

func healthSucceededAgo(slice string, ago time.Duration, consecutiveFailures int) store.SliceHealth {
	lastSuccessAt := statusNow.Add(-ago)
	return store.SliceHealth{Slice: slice, LastSuccessAt: &lastSuccessAt, ConsecutiveFailures: consecutiveFailures}
}

func TestSliceStateEscalatesWithConsecutiveFailures(t *testing.T) {
	cases := map[int]healthState{0: stateHealthy, 1: stateRetrying, 2: stateRetrying, 3: stateFailing}
	for failures, want := range cases {
		if got := sliceState(statusConfig, healthSucceededAgo("game", time.Second, failures), statusNow); got != want {
			t.Errorf("state after %d failures = %s, want %s", failures, got, want)
		}
	}
}

func TestCatalogIsJudgedAgainstItsRefreshInterval(t *testing.T) {
	catalogRefreshedAWhileAgo := healthSucceededAgo(provider.CatalogSlice, 45*time.Second, 0)
	gameUpdatedAWhileAgo := healthSucceededAgo("game", 45*time.Second, 0)

	if got := sliceState(statusConfig, catalogRefreshedAWhileAgo, statusNow); got != stateHealthy {
		t.Errorf("catalog refreshed 45s ago with a 1m interval = %s, want healthy", got)
	}
	if got := sliceState(statusConfig, gameUpdatedAWhileAgo, statusNow); got != stateStale {
		t.Errorf("game slice last updated 45s ago with 30s staleness = %s, want stale", got)
	}
}
