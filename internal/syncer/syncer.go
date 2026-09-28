package syncer

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/BrendanMoorehead/data-flow/internal/canonical"
	"github.com/BrendanMoorehead/data-flow/internal/identity"
	"github.com/BrendanMoorehead/data-flow/internal/provider"
	"github.com/BrendanMoorehead/data-flow/internal/source"
	"github.com/BrendanMoorehead/data-flow/internal/store"
)

type Syncer struct {
	providers []provider.Provider
	sources   source.Registry
	events    *identity.Resolver
	store     *store.Store
	logger    *slog.Logger
	now       func() time.Time
}

type Dependencies struct {
	Providers []provider.Provider
	Sources   source.Registry
	Events    *identity.Resolver
	Store     *store.Store
	Logger    *slog.Logger
	Now       func() time.Time
}

func New(deps Dependencies) *Syncer {
	return &Syncer{
		providers: deps.Providers,
		sources:   deps.Sources,
		events:    deps.Events,
		store:     deps.Store,
		logger:    deps.Logger,
		now:       deps.Now,
	}
}

func (s *Syncer) Run(ctx context.Context) {
	var running sync.WaitGroup
	for _, p := range s.providers {
		runner, configured := s.runnerFor(p)
		if !configured {
			continue
		}
		startTask(&running, func() { runner.run(ctx) })
	}
	running.Wait()
}

func (s *Syncer) PollProviderOnce(ctx context.Context, p provider.Provider) {
	runner, configured := s.runnerFor(p)
	if !configured {
		return
	}
	runner.refreshCatalog(ctx, 0)
	runner.pollDueSlicesOnce(ctx)
}

func (s *Syncer) runnerFor(p provider.Provider) (*providerRunner, bool) {
	config, configured := s.sources.Lookup(p.Source())
	if !configured {
		s.logger.Error("provider has no source config; not polling", "source", p.Source())
		return nil, false
	}
	return newProviderRunner(s, p, config), true
}

func (s *Syncer) pollSlice(ctx context.Context, p provider.Provider, slice provider.SliceKey) error {
	startedAt := s.now()
	snapshot, err := p.Poll(ctx, slice)
	if err != nil {
		return err
	}
	counts, err := s.ingest(ctx, sliceRef{source: p.Source(), slice: string(slice)}, snapshot)
	if err != nil {
		return err
	}
	s.logPoll(p.Source(), slice, counts, s.now().Sub(startedAt))
	return nil
}

func (s *Syncer) recordAttempt(ctx context.Context, sourceID canonical.SourceID, slice string, attemptedAt, nextAttemptAt time.Time, pollErr error) {
	if pollErr != nil {
		s.logger.Warn("poll failed", "source", sourceID, "slice", slice, "error", pollErr, "next_attempt_at", nextAttemptAt)
	}
	attempt := store.PollAttempt{Source: sourceID, Slice: slice, At: attemptedAt, NextAttemptAt: nextAttemptAt, Err: pollErr}
	if err := s.store.RecordPollAttempt(ctx, attempt); err != nil {
		s.logger.Error("record poll attempt", "source", sourceID, "slice", slice, "error", err)
	}
}

func (s *Syncer) logPoll(sourceID canonical.SourceID, slice provider.SliceKey, counts ingestCounts, duration time.Duration) {
	s.logger.Info("poll succeeded",
		"source", sourceID,
		"slice", slice,
		"inserted", counts.inserted,
		"confirmed", counts.confirmed,
		"off_board_inferred", counts.offBoardInferred,
		"quarantined", counts.quarantined,
		"duration_ms", duration.Milliseconds())
}
