package backoff

import (
	"testing"
	"time"
)

const (
	testBase    = 2 * time.Second
	testCeiling = time.Minute
)

func TestDelayDoublesWithEachConsecutiveFailure(t *testing.T) {
	cases := map[int]time.Duration{1: 2 * time.Second, 2: 4 * time.Second, 3: 8 * time.Second}
	for failures, wantFullDelay := range cases {
		if got := Delay(testBase, testCeiling, failures, 1); got != wantFullDelay {
			t.Errorf("Delay after %d failures = %v, want %v", failures, got, wantFullDelay)
		}
	}
}

func TestDelayNeverExceedsCeiling(t *testing.T) {
	if got := Delay(testBase, testCeiling, 50, 1); got != testCeiling {
		t.Errorf("Delay after 50 failures = %v, want ceiling %v", got, testCeiling)
	}
}

func TestDelayJitterStaysWithinUpperHalf(t *testing.T) {
	fullDelay := 8 * time.Second
	shortest := Delay(testBase, testCeiling, 3, 0)
	longest := Delay(testBase, testCeiling, 3, 1)

	if shortest != fullDelay/2 || longest != fullDelay {
		t.Errorf("jitter range = [%v, %v], want [%v, %v]", shortest, longest, fullDelay/2, fullDelay)
	}
}
