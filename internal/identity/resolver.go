package identity

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/BrendanMoorehead/data-flow/internal/canonical"
)

const eventStartMatchWindow = 30 * time.Minute

var ErrUnknownTeam = errors.New("unknown team alias")

type Store interface {
	TeamIDForAlias(ctx context.Context, source canonical.SourceID, alias string) (canonical.TeamID, bool, error)
	EventLinkForSourceRef(ctx context.Context, source canonical.SourceID, providerEventID string) (canonical.EventLink, bool, error)
	FindEventByTeams(ctx context.Context, matchup canonical.Matchup, startsFrom, startsTo time.Time) (canonical.EventLink, bool, error)
	CreateEvent(ctx context.Context, matchup canonical.Matchup, league string, startsAt time.Time) (canonical.EventID, error)
	LinkSourceRef(ctx context.Context, source canonical.SourceID, providerEventID string, link canonical.EventLink) error
}

type Resolver struct {
	store  Store
	logger *slog.Logger
	mu     sync.Mutex
}

func NewResolver(store Store, logger *slog.Logger) *Resolver {
	return &Resolver{store: store, logger: logger}
}

func (r *Resolver) ResolveEvent(ctx context.Context, source canonical.SourceID, event canonical.ProviderEvent) (canonical.EventLink, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	link, linked, err := r.store.EventLinkForSourceRef(ctx, source, event.ProviderEventID)
	if err != nil || linked {
		return link, err
	}
	link, err = r.matchOrCreateEvent(ctx, source, event)
	if err != nil {
		return canonical.EventLink{}, err
	}
	if link.SidesSwapped {
		r.logger.Warn("provider lists home and away the other way round; flipping its sides",
			"source", source, "provider_event_id", event.ProviderEventID, "event_id", link.EventID)
	}
	return link, r.store.LinkSourceRef(ctx, source, event.ProviderEventID, link)
}

func (r *Resolver) matchOrCreateEvent(ctx context.Context, source canonical.SourceID, event canonical.ProviderEvent) (canonical.EventLink, error) {
	matchup, err := r.resolveMatchup(ctx, source, event)
	if err != nil {
		return canonical.EventLink{}, err
	}
	startsFrom := event.StartsAt.Add(-eventStartMatchWindow)
	startsTo := event.StartsAt.Add(eventStartMatchWindow)
	link, found, err := r.store.FindEventByTeams(ctx, matchup, startsFrom, startsTo)
	if err != nil || found {
		return link, err
	}
	eventID, err := r.store.CreateEvent(ctx, matchup, event.League, event.StartsAt)
	return canonical.EventLink{EventID: eventID}, err
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
