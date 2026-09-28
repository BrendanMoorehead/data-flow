package canonical

import (
	"time"

	"github.com/BrendanMoorehead/data-flow/internal/odds"
)

type SourceID string

const (
	SourceAggregator       SourceID = "aggregator"
	SourceDraftKingsDirect SourceID = "draftkings_direct"
	SourceFanDuelDirect    SourceID = "fanduel_direct"
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

type MarketStatus string

const (
	StatusOpen     MarketStatus = "open"
	StatusOffBoard MarketStatus = "off_board"
)

type ConfidenceLevel string

const (
	ConfidenceVerified ConfidenceLevel = "verified"
	ConfidenceHigh     ConfidenceLevel = "high"
	ConfidenceMedium   ConfidenceLevel = "medium"
	ConfidenceLow      ConfidenceLevel = "low"
)

type ConfidenceReason string

const (
	ReasonDirectFresh       ConfidenceReason = "direct_fresh"
	ReasonDirectStale       ConfidenceReason = "direct_stale"
	ReasonSourcesMatch      ConfidenceReason = "sources_match"
	ReasonSourcesAgree      ConfidenceReason = "sources_agree"
	ReasonSourcesDisagree   ConfidenceReason = "sources_disagree"
	ReasonNoSecondSource    ConfidenceReason = "no_second_source"
	ReasonNoSourceTimestamp ConfidenceReason = "no_source_timestamp"
	ReasonAllSourcesStale   ConfidenceReason = "all_sources_stale"
)

type Confidence struct {
	Level   ConfidenceLevel
	Reasons []ConfidenceReason
}

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
// matched to a canonical event. When the market is off the board, Price is the last price
// the source showed.
type Observation struct {
	Source             SourceID
	Book               BookID
	Event              ProviderEvent
	Market             MarketType
	Side               Side
	Status             MarketStatus
	Price              odds.Decimal
	RawPrice           string
	RawFormat          PriceFormat
	ObservedAt         time.Time
	HasSourceTimestamp bool
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
	Key                PriceKey
	Source             SourceID
	Status             MarketStatus
	Price              odds.Decimal
	ObservedAt         time.Time
	LastConfirmedAt    time.Time
	HasSourceTimestamp bool
}

// ResolvedPrice is the price the service serves for a price key after precedence rules.
type ResolvedPrice struct {
	Key             PriceKey
	Status          MarketStatus
	Price           odds.Decimal
	Source          SourceID
	ObservedAt      time.Time
	LastConfirmedAt time.Time
	Confidence      Confidence
}
