package api

import (
	"time"

	"github.com/BrendanMoorehead/data-flow/internal/canonical"
	"github.com/BrendanMoorehead/data-flow/internal/source"
	"github.com/BrendanMoorehead/data-flow/internal/store"
)

type healthState string

const (
	stateHealthy healthState = "healthy"
	stateStale   healthState = "stale"
	stateFailing healthState = "failing"
	stateUnknown healthState = "unknown"
)

var severityByState = map[healthState]int{
	stateHealthy: 0,
	stateUnknown: 1,
	stateStale:   2,
	stateFailing: 3,
}

type sliceStatusResponse struct {
	Slice               string      `json:"slice"`
	State               healthState `json:"state"`
	LastAttemptAt       time.Time   `json:"last_attempt_at"`
	LastSuccessAt       *time.Time  `json:"last_success_at"`
	LastError           string      `json:"last_error,omitempty"`
	LastErrorAt         *time.Time  `json:"last_error_at,omitempty"`
	ConsecutiveFailures int         `json:"consecutive_failures"`
}

type sourceStatusResponse struct {
	Source            canonical.SourceID    `json:"source"`
	Kind              source.Kind           `json:"kind"`
	State             healthState           `json:"state"`
	StaleAfterSeconds float64               `json:"stale_after_seconds"`
	Slices            []sliceStatusResponse `json:"slices"`
}

type statusResponse struct {
	GeneratedAt time.Time              `json:"generated_at"`
	Sources     []sourceStatusResponse `json:"sources"`
}

func statusResponseFrom(sources source.Registry, healths []store.SliceHealth, now time.Time) statusResponse {
	healthsBySource := groupBySource(healths)
	response := statusResponse{GeneratedAt: now}
	for _, config := range sources.SortedByID() {
		response.Sources = append(response.Sources, sourceStatusFrom(config, healthsBySource[config.ID], now))
	}
	return response
}

func sourceStatusFrom(config source.Config, healths []store.SliceHealth, now time.Time) sourceStatusResponse {
	slices := make([]sliceStatusResponse, 0, len(healths))
	for _, health := range healths {
		slices = append(slices, sliceStatusFrom(config, health, now))
	}
	return sourceStatusResponse{
		Source:            config.ID,
		Kind:              config.Kind,
		State:             worstState(slices),
		StaleAfterSeconds: config.StaleAfter.Seconds(),
		Slices:            slices,
	}
}

func sliceStatusFrom(config source.Config, health store.SliceHealth, now time.Time) sliceStatusResponse {
	return sliceStatusResponse{
		Slice:               health.Slice,
		State:               sliceState(config, health, now),
		LastAttemptAt:       health.LastAttemptAt,
		LastSuccessAt:       health.LastSuccessAt,
		LastError:           health.LastError,
		LastErrorAt:         health.LastErrorAt,
		ConsecutiveFailures: health.ConsecutiveFailures,
	}
}

func sliceState(config source.Config, health store.SliceHealth, now time.Time) healthState {
	if health.ConsecutiveFailures > 0 {
		return stateFailing
	}
	if health.LastSuccessAt == nil || !config.IsFresh(*health.LastSuccessAt, now) {
		return stateStale
	}
	return stateHealthy
}

func worstState(slices []sliceStatusResponse) healthState {
	if len(slices) == 0 {
		return stateUnknown
	}
	worst := stateHealthy
	for _, slice := range slices {
		if severityByState[slice.State] > severityByState[worst] {
			worst = slice.State
		}
	}
	return worst
}

func groupBySource(healths []store.SliceHealth) map[canonical.SourceID][]store.SliceHealth {
	grouped := make(map[canonical.SourceID][]store.SliceHealth)
	for _, health := range healths {
		grouped[health.Source] = append(grouped[health.Source], health)
	}
	return grouped
}
