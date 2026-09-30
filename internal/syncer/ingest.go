package syncer

import (
	"context"
	"errors"
	"time"

	"github.com/BrendanMoorehead/data-flow/internal/canonical"
	"github.com/BrendanMoorehead/data-flow/internal/identity"
	"github.com/BrendanMoorehead/data-flow/internal/provider"
	"github.com/BrendanMoorehead/data-flow/internal/sanity"
	"github.com/BrendanMoorehead/data-flow/internal/store"
)

type sliceRef struct {
	source canonical.SourceID
	slice  string
}

type ingestCounts struct {
	inserted         int
	confirmed        int
	offBoardInferred int
	quarantined      int
}

func (c *ingestCounts) add(result store.RecordResult) {
	switch result {
	case store.RecordInserted:
		c.inserted++
	case store.RecordConfirmed:
		c.confirmed++
	}
}

func (s *Syncer) ingest(ctx context.Context, ref sliceRef, snapshot provider.Snapshot) (ingestCounts, error) {
	var counts ingestCounts
	receivedAt := s.now()
	accepted, marginRejections := sanity.CheckMargins(snapshot.Observations)
	if err := s.quarantineAll(ctx, ref, append(snapshot.Rejections, marginRejections...), receivedAt, &counts); err != nil {
		return counts, err
	}
	seen, err := s.recordAccepted(ctx, ref, accepted, receivedAt, &counts)
	if err != nil {
		return counts, err
	}
	if snapshot.MissingMeansOffBoard {
		return counts, s.markMissingOffBoard(ctx, ref, seen, receivedAt, &counts)
	}
	return counts, nil
}

func (s *Syncer) recordAccepted(ctx context.Context, ref sliceRef, observations []canonical.Observation, receivedAt time.Time, counts *ingestCounts) (map[canonical.PriceKey]bool, error) {
	seen := make(map[canonical.PriceKey]bool, len(observations))
	linksByProviderEvent := make(map[string]canonical.EventLink)
	for _, observation := range observations {
		observation = withObservedAtFallback(observation, receivedAt)
		link, err := s.eventLinkFor(ctx, observation, linksByProviderEvent)
		if errors.Is(err, identity.ErrUnknownTeam) {
			rejection := provider.NewRejection(observation.Event, "%v", err)
			if err := s.quarantineAll(ctx, ref, []provider.Rejection{rejection}, receivedAt, counts); err != nil {
				return seen, err
			}
			continue
		}
		if err != nil {
			return seen, err
		}
		observation = orientedTo(link, observation)
		result, err := s.store.RecordObservation(ctx, store.ObservationRecord{
			EventID: link.EventID, Slice: ref.slice, Observation: observation, ReceivedAt: receivedAt,
		})
		if err != nil {
			return seen, err
		}
		counts.add(result)
		seen[observation.KeyFor(link.EventID)] = true
	}
	return seen, nil
}

func (s *Syncer) eventLinkFor(ctx context.Context, observation canonical.Observation, linksByProviderEvent map[string]canonical.EventLink) (canonical.EventLink, error) {
	if link, resolved := linksByProviderEvent[observation.Event.ProviderEventID]; resolved {
		return link, nil
	}
	link, err := s.events.ResolveEvent(ctx, observation.Source, observation.Event)
	if err == nil {
		linksByProviderEvent[observation.Event.ProviderEventID] = link
	}
	return link, err
}

func (s *Syncer) markMissingOffBoard(ctx context.Context, ref sliceRef, seen map[canonical.PriceKey]bool, receivedAt time.Time, counts *ingestCounts) error {
	previous, err := s.store.LatestForSourceSlice(ctx, ref.source, ref.slice)
	if err != nil {
		return err
	}
	for _, known := range previous {
		if seen[known.Latest.Key] {
			continue
		}
		if err := s.recordMissingAsOffBoard(ctx, ref, known, receivedAt, counts); err != nil {
			return err
		}
	}
	return nil
}

func (s *Syncer) recordMissingAsOffBoard(ctx context.Context, ref sliceRef, known store.SliceObservation, receivedAt time.Time, counts *ingestCounts) error {
	if known.Latest.Status == canonical.StatusOffBoard {
		counts.confirmed++
		return s.store.ConfirmObservation(ctx, known.ContentKey, receivedAt)
	}
	counts.offBoardInferred++
	_, err := s.store.RecordObservation(ctx, store.ObservationRecord{
		EventID:     known.Latest.Key.EventID,
		Slice:       ref.slice,
		Observation: inferredOffBoard(known.Latest, receivedAt),
		ReceivedAt:  receivedAt,
	})
	return err
}

func (s *Syncer) quarantineAll(ctx context.Context, ref sliceRef, rejections []provider.Rejection, at time.Time, counts *ingestCounts) error {
	for _, rejection := range rejections {
		s.logger.Warn("observation quarantined", "source", ref.source, "slice", ref.slice, "reason", rejection.Reason)
		record := store.QuarantineRecord{Source: ref.source, Slice: ref.slice, Reason: rejection.Reason, Raw: rejection.Raw, QuarantinedAt: at}
		if err := s.store.Quarantine(ctx, record); err != nil {
			return err
		}
		counts.quarantined++
	}
	return nil
}

func orientedTo(link canonical.EventLink, observation canonical.Observation) canonical.Observation {
	if link.SidesSwapped {
		observation.Side = observation.Side.Opposite()
	}
	return observation
}

func withObservedAtFallback(observation canonical.Observation, receivedAt time.Time) canonical.Observation {
	if !observation.HasSourceTimestamp {
		observation.ObservedAt = receivedAt
	}
	return observation
}

func inferredOffBoard(lastOpen canonical.SourcePrice, noticedAt time.Time) canonical.Observation {
	return canonical.Observation{
		Source:     lastOpen.Source,
		Book:       lastOpen.Key.Book,
		Market:     lastOpen.Key.Market,
		Side:       lastOpen.Key.Side,
		Status:     canonical.StatusOffBoard,
		Price:      lastOpen.Price,
		ObservedAt: noticedAt,
	}
}
