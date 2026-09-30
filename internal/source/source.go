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
	ID                     canonical.SourceID
	Kind                   Kind
	PollInterval           time.Duration
	StaleAfter             time.Duration
	RequestTimeout         time.Duration
	CatalogRefreshInterval time.Duration
	Workers                int
	RequestsPerSecond      float64
	RequestBurst           int
	// PriceDecimals is how many decimal places the source rounds its prices to.
	// Zero means the source sends exact prices, such as American odds.
	PriceDecimals int
}

func (c Config) IsFresh(lastConfirmedAt, now time.Time) bool {
	return now.Sub(lastConfirmedAt) <= c.StaleAfter
}

func (c Config) IsCatalogFresh(lastRefreshedAt, now time.Time) bool {
	return now.Sub(lastRefreshedAt) <= c.CatalogRefreshInterval+c.StaleAfter
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
