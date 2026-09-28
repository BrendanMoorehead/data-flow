package resolve

import (
	"context"
	"log/slog"
	"time"

	"github.com/BrendanMoorehead/data-flow/internal/canonical"
	"github.com/BrendanMoorehead/data-flow/internal/source"
)

type Store interface {
	LatestSourcePrices(ctx context.Context) ([]canonical.SourcePrice, error)
	UpsertResolvedPrices(ctx context.Context, prices []canonical.ResolvedPrice, resolvedAt time.Time) error
}

type Rebuilder struct {
	store   Store
	sources source.Registry
	logger  *slog.Logger
	now     func() time.Time
}

func NewRebuilder(store Store, sources source.Registry, logger *slog.Logger, now func() time.Time) *Rebuilder {
	return &Rebuilder{store: store, sources: sources, logger: logger, now: now}
}

func (r *Rebuilder) Run(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		if err := r.Rebuild(ctx); err != nil {
			r.logger.Error("rebuild resolved prices", "error", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (r *Rebuilder) Rebuild(ctx context.Context) error {
	latest, err := r.store.LatestSourcePrices(ctx)
	if err != nil {
		return err
	}
	now := r.now()
	resolved := resolveAll(r.candidatesByKey(latest), now)
	return r.store.UpsertResolvedPrices(ctx, resolved, now)
}

func (r *Rebuilder) candidatesByKey(latest []canonical.SourcePrice) map[canonical.PriceKey][]Candidate {
	candidates := make(map[canonical.PriceKey][]Candidate)
	for _, price := range latest {
		config, configured := r.sources.Lookup(price.Source)
		if !configured {
			continue
		}
		candidates[price.Key] = append(candidates[price.Key], Candidate{SourcePrice: price, Config: config})
	}
	return candidates
}

func resolveAll(candidatesByKey map[canonical.PriceKey][]Candidate, now time.Time) []canonical.ResolvedPrice {
	resolved := make([]canonical.ResolvedPrice, 0, len(candidatesByKey))
	for _, candidates := range candidatesByKey {
		if price, found := Resolve(candidates, now); found {
			resolved = append(resolved, price)
		}
	}
	return resolved
}
