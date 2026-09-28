package sim

import (
	"cmp"
	"errors"
	"fmt"
	"math/rand/v2"
	"slices"
	"sync"
	"time"
)

type Scenario string

const (
	ScenarioDraftKingsOutage    Scenario = "draftkings-outage"
	ScenarioDraftKingsSlow      Scenario = "draftkings-slow"
	ScenarioAggregatorRateLimit Scenario = "aggregator-rate-limit"
	ScenarioAggregatorLag       Scenario = "aggregator-lag"
)

const (
	draftKingsBackgroundErrorRate  = 0.05
	draftKingsOutOfOrderRate       = 0.05
	draftKingsDuplicateOutcomeRate = 0.03
	draftKingsSlowResponseDelay    = 3 * time.Second
	aggregatorRateLimitWindow      = 5 * time.Second
	aggregatorLagDelay             = 20 * time.Second
)

var scenarioDescriptions = map[Scenario]string{
	ScenarioDraftKingsOutage:    "every DraftKings request returns 503",
	ScenarioDraftKingsSlow:      fmt.Sprintf("DraftKings responses take %v, longer than the sync timeout", draftKingsSlowResponseDelay),
	ScenarioAggregatorRateLimit: fmt.Sprintf("the aggregator allows one request per %v and answers the rest with 429 and Retry-After", aggregatorRateLimitWindow),
	ScenarioAggregatorLag:       fmt.Sprintf("the aggregator serves prices from %v ago", aggregatorLagDelay),
}

var ErrUnknownScenario = errors.New("unknown scenario")

type scenarioStatus struct {
	Name        Scenario   `json:"name"`
	Description string     `json:"description"`
	Active      bool       `json:"active"`
	ActiveUntil *time.Time `json:"active_until,omitempty"`
}

type chaos struct {
	mu                      sync.Mutex
	random                  *rand.Rand
	backgroundProblems      bool
	activeUntil             map[Scenario]time.Time
	aggregatorLastAllowedAt time.Time
	now                     func() time.Time
}

func newChaos(seed uint64, backgroundProblems bool, now func() time.Time) *chaos {
	return &chaos{
		random:             rand.New(rand.NewPCG(seed, seed+1)),
		backgroundProblems: backgroundProblems,
		activeUntil:        make(map[Scenario]time.Time),
		now:                now,
	}
}

func (c *chaos) activate(scenario Scenario, duration time.Duration) (time.Time, error) {
	if _, known := scenarioDescriptions[scenario]; !known {
		return time.Time{}, fmt.Errorf("%w: %s", ErrUnknownScenario, scenario)
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	until := c.now().Add(duration)
	c.activeUntil[scenario] = until
	return until, nil
}

func (c *chaos) clear(scenario Scenario) error {
	if _, known := scenarioDescriptions[scenario]; !known {
		return fmt.Errorf("%w: %s", ErrUnknownScenario, scenario)
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.activeUntil, scenario)
	return nil
}

func (c *chaos) isActive(scenario Scenario) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now().Before(c.activeUntil[scenario])
}

func (c *chaos) backgroundChance(probability float64) bool {
	if !c.backgroundProblems {
		return false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.random.Float64() < probability
}

func (c *chaos) aggregatorRetryAfter() (time.Duration, bool) {
	if !c.isActive(ScenarioAggregatorRateLimit) {
		return 0, false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	now := c.now()
	if sinceLastAllowed := now.Sub(c.aggregatorLastAllowedAt); sinceLastAllowed < aggregatorRateLimitWindow {
		return aggregatorRateLimitWindow - sinceLastAllowed, true
	}
	c.aggregatorLastAllowedAt = now
	return 0, false
}

func (c *chaos) statuses() []scenarioStatus {
	c.mu.Lock()
	defer c.mu.Unlock()
	now := c.now()
	statuses := make([]scenarioStatus, 0, len(scenarioDescriptions))
	for scenario, description := range scenarioDescriptions {
		status := scenarioStatus{Name: scenario, Description: description}
		if until := c.activeUntil[scenario]; now.Before(until) {
			status.Active = true
			status.ActiveUntil = &until
		}
		statuses = append(statuses, status)
	}
	slices.SortFunc(statuses, func(a, b scenarioStatus) int { return cmp.Compare(a.Name, b.Name) })
	return statuses
}
