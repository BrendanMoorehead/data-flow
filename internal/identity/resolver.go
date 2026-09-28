package identity

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/BrendanMoorehead/data-flow/internal/canonical"
)

const eventStartMatchWindow = 30 * time.Minute

var ErrUnknownTeam = errors.New("unknown team alias")

type Store interface {
	TeamIDForAlias(ctx context.Context, source canonical.SourceID, alias string) (canonical.TeamID, bool, error)
	EventIDForSourceRef(ctx context.Context, source canonical.SourceID, providerEventID string) (canonical.EventID, bool, error)
	FindEventByMatchup(ctx context.Context, matchup canonical.Matchup, startsFrom, startsTo time.Time) (canonical.EventID, bool, error)
	CreateEvent(ctx context.Context, matchup canonical.Matchup, league string, startsAt time.Time) (canonical.EventID, error)
	LinkSourceRef(ctx context.Context, source canonical.SourceID, providerEventID string, eventID canonical.EventID) error
}

type Resolver struct {
	store Store
	mu    sync.Mutex
}

func NewResolver(store Store) *Resolver {
	return &Resolver{store: store}
}

func (r *Resolver) ResolveEvent(ctx context.Context, source canonical.SourceID, event canonical.ProviderEvent) (canonical.EventID, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	eventID, linked, err := r.store.EventIDForSourceRef(ctx, source, event.ProviderEventID)
	if err != nil || linked {
		return eventID, err
	}
	eventID, err = r.matchOrCreateEvent(ctx, source, event)
	if err != nil {
		return 0, err
	}
	return eventID, r.store.LinkSourceRef(ctx, source, event.ProviderEventID, eventID)
}

func (r *Resolver) matchOrCreateEvent(ctx context.Context, source canonical.SourceID, event canonical.ProviderEvent) (canonical.EventID, error) {
	matchup, err := r.resolveMatchup(ctx, source, event)
	if err != nil {
		return 0, err
	}
	startsFrom := event.StartsAt.Add(-eventStartMatchWindow)
	startsTo := event.StartsAt.Add(eventStartMatchWindow)
	eventID, found, err := r.store.FindEventByMatchup(ctx, matchup, startsFrom, startsTo)
	if err != nil || found {
		return eventID, err
	}
	return r.store.CreateEvent(ctx, matchup, event.League, event.StartsAt)
}

func (r *Resolver) resolveMatchup(ctx context.Context, source canonical.SourceID, event canonical.ProviderEvent) (canonical.Matchup, error) {
	home, err := r.resolveTeam(ctx, source, event.HomeTeam)
	if err != nil {
		return canonical.Matchup{}, err
	}
	away, err := r.resolveTeam(ctx, source, event.AwayTeam)
	if err != nil {
		return canonical.Matchup{}, err
	}
	return canonical.Matchup{Home: home, Away: away}, nil
}

func (r *Resolver) resolveTeam(ctx context.Context, source canonical.SourceID, alias string) (canonical.TeamID, error) {
	teamID, found, err := r.store.TeamIDForAlias(ctx, source, alias)
	if err != nil {
		return "", err
	}
	if !found {
		return "", fmt.Errorf("%w: %s calls a team %q", ErrUnknownTeam, source, alias)
	}
	return teamID, nil
}
