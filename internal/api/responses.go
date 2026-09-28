package api

import (
	"time"

	"github.com/BrendanMoorehead/data-flow/internal/canonical"
)

type eventResponse struct {
	ID       int64     `json:"id"`
	League   string    `json:"league"`
	HomeTeam string    `json:"home_team"`
	AwayTeam string    `json:"away_team"`
	StartsAt time.Time `json:"starts_at"`
}

type eventsResponse struct {
	Events []eventResponse `json:"events"`
}

type freshnessResponse struct {
	AgeSeconds        float64 `json:"age_seconds"`
	StaleAfterSeconds float64 `json:"stale_after_seconds"`
	IsStale           bool    `json:"is_stale"`
}

type confidenceResponse struct {
	Level   canonical.ConfidenceLevel    `json:"level"`
	Reasons []canonical.ConfidenceReason `json:"reasons"`
}

type priceResponse struct {
	Book            canonical.BookID       `json:"book"`
	Market          canonical.MarketType   `json:"market"`
	Side            canonical.Side         `json:"side"`
	Status          canonical.MarketStatus `json:"status"`
	PriceDecimal    float64                `json:"price_decimal"`
	PriceAmerican   int                    `json:"price_american"`
	Source          canonical.SourceID     `json:"source"`
	ObservedAt      time.Time              `json:"observed_at"`
	LastConfirmedAt time.Time              `json:"last_confirmed_at"`
	Freshness       freshnessResponse      `json:"freshness"`
	Confidence      confidenceResponse     `json:"confidence"`
}

type oddsResponse struct {
	EventID     int64           `json:"event_id"`
	GeneratedAt time.Time       `json:"generated_at"`
	Prices      []priceResponse `json:"prices"`
}

func eventsResponseFrom(events []canonical.Event) eventsResponse {
	response := eventsResponse{Events: make([]eventResponse, 0, len(events))}
	for _, event := range events {
		response.Events = append(response.Events, eventResponse{
			ID:       int64(event.ID),
			League:   event.League,
			HomeTeam: event.HomeTeam,
			AwayTeam: event.AwayTeam,
			StartsAt: event.StartsAt,
		})
	}
	return response
}

func (s *Server) oddsResponseFrom(eventID int64, prices []canonical.ResolvedPrice) oddsResponse {
	now := s.now()
	response := oddsResponse{EventID: eventID, GeneratedAt: now, Prices: make([]priceResponse, 0, len(prices))}
	for _, price := range prices {
		response.Prices = append(response.Prices, priceResponse{
			Book:            price.Key.Book,
			Market:          price.Key.Market,
			Side:            price.Key.Side,
			Status:          price.Status,
			PriceDecimal:    float64(price.Price),
			PriceAmerican:   int(price.Price.American()),
			Source:          price.Source,
			ObservedAt:      price.ObservedAt,
			LastConfirmedAt: price.LastConfirmedAt,
			Freshness:       s.freshnessOf(price, now),
			Confidence:      confidenceResponse{Level: price.Confidence.Level, Reasons: price.Confidence.Reasons},
		})
	}
	return response
}

func (s *Server) freshnessOf(price canonical.ResolvedPrice, now time.Time) freshnessResponse {
	config, _ := s.sources.Lookup(price.Source)
	return freshnessResponse{
		AgeSeconds:        now.Sub(price.LastConfirmedAt).Seconds(),
		StaleAfterSeconds: config.StaleAfter.Seconds(),
		IsStale:           !config.IsFresh(price.LastConfirmedAt, now),
	}
}
