package syncer

import (
	"slices"
	"sync"
	"time"

	"github.com/BrendanMoorehead/data-flow/internal/provider"
)

type sliceState struct {
	nextAttemptAt       time.Time
	consecutiveFailures int
	inFlight            bool
}

type sliceSchedule struct {
	mu     sync.Mutex
	states map[provider.SliceKey]*sliceState
}

func newSliceSchedule() *sliceSchedule {
	return &sliceSchedule{states: make(map[provider.SliceKey]*sliceState)}
}

func (s *sliceSchedule) replaceSlices(keys []provider.SliceKey, now time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()

	current := make(map[provider.SliceKey]*sliceState, len(keys))
	for _, key := range keys {
		if existing, known := s.states[key]; known {
			current[key] = existing
			continue
		}
		current[key] = &sliceState{nextAttemptAt: now}
	}
	s.states = current
}

func (s *sliceSchedule) claimDue(now time.Time) []provider.SliceKey {
	s.mu.Lock()
	defer s.mu.Unlock()

	var due []provider.SliceKey
	for key, state := range s.states {
		if !state.inFlight && !state.nextAttemptAt.After(now) {
			state.inFlight = true
			due = append(due, key)
		}
	}
	slices.Sort(due)
	return due
}

func (s *sliceSchedule) finish(key provider.SliceKey, succeeded bool, now time.Time, delayAfter func(consecutiveFailures int) time.Duration) time.Time {
	s.mu.Lock()
	defer s.mu.Unlock()

	state, known := s.states[key]
	if !known {
		return now.Add(delayAfter(0))
	}
	state.inFlight = false
	state.consecutiveFailures = nextFailureCount(state.consecutiveFailures, succeeded)
	state.nextAttemptAt = now.Add(delayAfter(state.consecutiveFailures))
	return state.nextAttemptAt
}

func nextFailureCount(current int, succeeded bool) int {
	if succeeded {
		return 0
	}
	return current + 1
}
