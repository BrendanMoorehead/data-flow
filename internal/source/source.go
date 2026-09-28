package source

import (
	"cmp"
	"slices"
	"time"

	"github.com/BrendanMoorehead/data-flow/internal/canonical"
)

type Kind string

const (
	KindDirect     Kind = "direct"
	KindAggregator Kind = "aggregator"
)

type Config struct {
	ID           canonical.SourceID
	Kind         Kind
	PollInterval time.Duration
	StaleAfter   time.Duration
}

func (c Config) IsFresh(lastConfirmedAt, now time.Time) bool {
	return now.Sub(lastConfirmedAt) <= c.StaleAfter
}

type Registry map[canonical.SourceID]Config

func NewRegistry(configs ...Config) Registry {
	registry := make(Registry, len(configs))
	for _, config := range configs {
		registry[config.ID] = config
	}
	return registry
}

func (r Registry) Lookup(id canonical.SourceID) (Config, bool) {
	config, found := r[id]
	return config, found
}

func (r Registry) SortedByID() []Config {
	configs := make([]Config, 0, len(r))
	for _, config := range r {
		configs = append(configs, config)
	}
	slices.SortFunc(configs, func(a, b Config) int { return cmp.Compare(a.ID, b.ID) })
	return configs
}
