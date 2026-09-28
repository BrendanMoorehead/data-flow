package syncer

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"

	"github.com/BrendanMoorehead/data-flow/internal/canonical"
	"github.com/BrendanMoorehead/data-flow/internal/identity"
	"github.com/BrendanMoorehead/data-flow/internal/provider"
	"github.com/BrendanMoorehead/data-flow/internal/source"
	"github.com/BrendanMoorehead/data-flow/internal/store"
)

const catalogSlice = "catalog"

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
		running.Add(1)
		go func() {
			defer running.Done()
			s.runProvider(ctx, p)
		}()
	}
	running.Wait()
}

func (s *Syncer) runProvider(ctx context.Context, p provider.Provider) {
	config, configured := s.sources.Lookup(p.Source())
	if !configured {
		s.logger.Error("provider has no source config; not polling", "source", p.Source())
		return
	}
	ticker := time.NewTicker(config.PollInterval)
	defer ticker.Stop()
	for {
		s.PollProvider(ctx, p)
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (s *Syncer) PollProvider(ctx context.Context, p provider.Provider) {
	slices, err := p.Slices(ctx)
	if err != nil {
		s.recordFailure(ctx, p.Source(), catalogSlice, err)
		return
	}
	s.recordSuccess(ctx, p.Source(), catalogSlice)
	for _, slice := range slices {
		s.pollSlice(ctx, p, slice)
	}
}

func (s *Syncer) pollSlice(ctx context.Context, p provider.Provider, slice provider.SliceKey) {
	startedAt := s.now()
	snapshot, err := p.Poll(ctx, slice)
	if err != nil {
		s.recordFailure(ctx, p.Source(), string(slice), err)
		return
	}
	counts, err := s.ingest(ctx, p.Source(), snapshot)
	if err != nil {
		s.recordFailure(ctx, p.Source(), string(slice), err)
		return
	}
	s.recordSuccess(ctx, p.Source(), string(slice))
	s.logPoll(p.Source(), slice, counts, s.now().Sub(startedAt))
}

type ingestCounts struct {
	inserted  int
	confirmed int
	rejected  int
}

func (s *Syncer) ingest(ctx context.Context, sourceID canonical.SourceID, snapshot provider.Snapshot) (ingestCounts, error) {
	counts := ingestCounts{rejected: len(snapshot.Rejections)}
	s.logRejections(sourceID, snapshot.Rejections)

	receivedAt := s.now()
	for _, observation := range snapshot.Observations {
		result, err := s.recordObservation(ctx, observation, receivedAt)
		if errors.Is(err, identity.ErrUnknownTeam) {
			counts.rejected++
			s.logger.Warn("observation rejected", "source", sourceID, "reason", err)
			continue
		}
		if err != nil {
			return counts, err
		}
		counts.add(result)
	}
	return counts, nil
}

func (s *Syncer) recordObservation(ctx context.Context, observation canonical.Observation, receivedAt time.Time) (store.RecordResult, error) {
	eventID, err := s.events.ResolveEvent(ctx, observation.Source, observation.Event)
	if err != nil {
		return 0, err
	}
	return s.store.RecordObservation(ctx, eventID, observation, receivedAt)
}

func (c *ingestCounts) add(result store.RecordResult) {
	switch result {
	case store.RecordInserted:
		c.inserted++
	case store.RecordConfirmed:
		c.confirmed++
	}
}

func (s *Syncer) recordSuccess(ctx context.Context, sourceID canonical.SourceID, slice string) {
	if err := s.store.RecordPollSuccess(ctx, sourceID, slice, s.now()); err != nil {
		s.logger.Error("record poll success", "source", sourceID, "slice", slice, "error", err)
	}
}

func (s *Syncer) recordFailure(ctx context.Context, sourceID canonical.SourceID, slice string, pollErr error) {
	s.logger.Warn("poll failed", "source", sourceID, "slice", slice, "error", pollErr)
	if err := s.store.RecordPollFailure(ctx, sourceID, slice, s.now(), pollErr); err != nil {
		s.logger.Error("record poll failure", "source", sourceID, "slice", slice, "error", err)
	}
}

func (s *Syncer) logPoll(sourceID canonical.SourceID, slice provider.SliceKey, counts ingestCounts, duration time.Duration) {
	s.logger.Info("poll succeeded",
		"source", sourceID,
		"slice", slice,
		"inserted", counts.inserted,
		"confirmed", counts.confirmed,
		"rejected", counts.rejected,
		"duration_ms", duration.Milliseconds())
}

func (s *Syncer) logRejections(sourceID canonical.SourceID, rejections []provider.Rejection) {
	for _, rejection := range rejections {
		s.logger.Warn("observation rejected", "source", sourceID, "reason", rejection.Reason)
	}
}
