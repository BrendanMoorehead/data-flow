package canonical

import (
	"time"

	"github.com/BrendanMoorehead/data-flow/internal/odds"
)

type SourceID string

const (
	SourceAggregator       SourceID = "aggregator"
	SourceDraftKingsDirect SourceID = "draftkings_direct"
)

type BookID string

const (
	BookDraftKings BookID = "draftkings"
	BookFanDuel    BookID = "fanduel"
)

type MarketType string

const MarketMoneyline MarketType = "moneyline"

type Side string

const (
	SideHome Side = "home"
	SideAway Side = "away"
)

type PriceFormat string

const (
	FormatAmerican PriceFormat = "american"
	FormatDecimal  PriceFormat = "decimal"
)

type TeamID string

type EventID int64

type Team struct {
	ID   TeamID
	Name string
}

type TeamAlias struct {
	Source SourceID
	Alias  string
	TeamID TeamID
}

type Matchup struct {
	Home TeamID
	Away TeamID
}

type Event struct {
	ID       EventID
	League   string
	HomeTeam string
	AwayTeam string
	StartsAt time.Time
}

// ProviderEvent is an event as one provider describes it, before it is matched to an Event.
type ProviderEvent struct {
	ProviderEventID string
	League          string
	HomeTeam        string
	AwayTeam        string
	StartsAt        time.Time
}

// Observation is one provider price already translated into canonical units, but not yet
// matched to a canonical event.
type Observation struct {
	Source     SourceID
	Book       BookID
	Event      ProviderEvent
	Market     MarketType
	Side       Side
	Price      odds.Decimal
	RawPrice   string
	RawFormat  PriceFormat
	ObservedAt time.Time
}

func (o Observation) KeyFor(eventID EventID) PriceKey {
	return PriceKey{EventID: eventID, Book: o.Book, Market: o.Market, Side: o.Side}
}

type PriceKey struct {
	EventID EventID
	Book    BookID
	Market  MarketType
	Side    Side
}

// SourcePrice is the most recent observation one source has reported for a price key.
type SourcePrice struct {
	Key             PriceKey
	Source          SourceID
	Price           odds.Decimal
	ObservedAt      time.Time
	LastConfirmedAt time.Time
}

// ResolvedPrice is the price the service serves for a price key after precedence rules.
type ResolvedPrice struct {
	Key             PriceKey
	Price           odds.Decimal
	Source          SourceID
	ObservedAt      time.Time
	LastConfirmedAt time.Time
}
