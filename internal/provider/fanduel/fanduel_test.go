package fanduel

import (
	"testing"
	"time"

	"github.com/BrendanMoorehead/data-flow/internal/canonical"
)

func americanOdds(value int) *int {
	return &value
}

func lakersCelticsEvent(runners ...runnerJSON) eventJSON {
	return eventJSON{
		ID:        "fd-9001",
		StartTime: time.Date(2026, 9, 28, 23, 0, 0, 0, time.UTC).Unix(),
		HomeTeam:  teamJSON{Nickname: "Lakers"},
		AwayTeam:  teamJSON{Nickname: "Celtics"},
		Markets:   []marketJSON{{MarketType: marketTypeMoneyline, Runners: runners}},
	}
}

func TestSnapshotMapsRunnersWithoutSourceTimestamps(t *testing.T) {
	event := lakersCelticsEvent(
		runnerJSON{Side: "HOME", Status: "ACTIVE", AmericanOdds: americanOdds(-110)},
		runnerJSON{Side: "AWAY", Status: "ACTIVE", AmericanOdds: americanOdds(-110)},
	)

	snapshot := snapshotFromEvents("NBA", []eventJSON{event})

	if len(snapshot.Observations) != 2 || len(snapshot.Rejections) != 0 {
		t.Fatalf("got %d observations and %d rejections, want 2 and 0", len(snapshot.Observations), len(snapshot.Rejections))
	}
	observation := snapshot.Observations[0]
	if observation.HasSourceTimestamp || !observation.ObservedAt.IsZero() {
		t.Error("fanduel observation claims a source timestamp it never sent")
	}
	if observation.Event.StartsAt.Hour() != 23 {
		t.Errorf("start time = %v, want 23:00 UTC from epoch seconds", observation.Event.StartsAt)
	}
}

func TestSuspendedRunnerTakesOnlyThatSideOffTheBoard(t *testing.T) {
	event := lakersCelticsEvent(
		runnerJSON{Side: "HOME", Status: runnerStatusSuspended, AmericanOdds: americanOdds(-110)},
		runnerJSON{Side: "AWAY", Status: "ACTIVE", AmericanOdds: americanOdds(-110)},
	)

	snapshot := snapshotFromEvents("NBA", []eventJSON{event})

	statusBySide := map[canonical.Side]canonical.MarketStatus{}
	for _, observation := range snapshot.Observations {
		statusBySide[observation.Side] = observation.Status
	}
	if statusBySide[canonical.SideHome] != canonical.StatusOffBoard || statusBySide[canonical.SideAway] != canonical.StatusOpen {
		t.Errorf("statuses = %v, want home off the board and away open", statusBySide)
	}
}

func TestRunnerWithoutPriceIsRejected(t *testing.T) {
	event := lakersCelticsEvent(
		runnerJSON{Side: "HOME", Status: "ACTIVE"},
		runnerJSON{Side: "AWAY", Status: "ACTIVE", AmericanOdds: americanOdds(120)},
	)

	snapshot := snapshotFromEvents("NBA", []eventJSON{event})

	if len(snapshot.Rejections) != 1 || len(snapshot.Observations) != 1 {
		t.Errorf("got %d rejections and %d observations, want 1 and 1", len(snapshot.Rejections), len(snapshot.Observations))
	}
}
