package syncer

import (
	"slices"
	"testing"
	"time"

	"github.com/BrendanMoorehead/data-flow/internal/provider"
)

var scheduleNow = time.Date(2026, 9, 28, 18, 0, 0, 0, time.UTC)

func delayOfOneSecondPerFailure(consecutiveFailures int) time.Duration {
	return time.Duration(consecutiveFailures+1) * time.Second
}

func TestClaimDueSkipsSlicesAlreadyInFlight(t *testing.T) {
	schedule := newSliceSchedule()
	schedule.replaceSlices([]provider.SliceKey{"a", "b"}, scheduleNow)

	first := schedule.claimDue(scheduleNow)
	second := schedule.claimDue(scheduleNow)

	if !slices.Equal(first, []provider.SliceKey{"a", "b"}) || len(second) != 0 {
		t.Errorf("claims = %v then %v, want [a b] then nothing", first, second)
	}
}

func TestFinishAfterFailureDelaysAndCountsFailures(t *testing.T) {
	schedule := newSliceSchedule()
	schedule.replaceSlices([]provider.SliceKey{"a"}, scheduleNow)
	schedule.claimDue(scheduleNow)

	nextAttemptAt := schedule.finish("a", false, scheduleNow, delayOfOneSecondPerFailure)

	if want := scheduleNow.Add(2 * time.Second); !nextAttemptAt.Equal(want) {
		t.Errorf("next attempt = %v, want %v", nextAttemptAt, want)
	}
	if due := schedule.claimDue(scheduleNow.Add(time.Second)); len(due) != 0 {
		t.Errorf("slice was due again before its backoff ended: %v", due)
	}
}

func TestFinishAfterSuccessResetsFailures(t *testing.T) {
	schedule := newSliceSchedule()
	schedule.replaceSlices([]provider.SliceKey{"a"}, scheduleNow)
	for _, succeeded := range []bool{false, false} {
		schedule.claimDue(scheduleNow.Add(time.Hour))
		schedule.finish("a", succeeded, scheduleNow, delayOfOneSecondPerFailure)
	}
	schedule.claimDue(scheduleNow.Add(time.Hour))

	nextAttemptAt := schedule.finish("a", true, scheduleNow, delayOfOneSecondPerFailure)

	if want := scheduleNow.Add(time.Second); !nextAttemptAt.Equal(want) {
		t.Errorf("next attempt after recovery = %v, want %v", nextAttemptAt, want)
	}
}

func TestReplaceSlicesKeepsBackoffOfKnownSlices(t *testing.T) {
	schedule := newSliceSchedule()
	schedule.replaceSlices([]provider.SliceKey{"a"}, scheduleNow)
	schedule.claimDue(scheduleNow)
	schedule.finish("a", false, scheduleNow, delayOfOneSecondPerFailure)

	schedule.replaceSlices([]provider.SliceKey{"a", "b"}, scheduleNow)

	if due := schedule.claimDue(scheduleNow); !slices.Equal(due, []provider.SliceKey{"b"}) {
		t.Errorf("due after catalog refresh = %v, want only the new slice b", due)
	}
}
