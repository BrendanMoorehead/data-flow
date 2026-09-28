package aggregator

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/BrendanMoorehead/data-flow/internal/canonical"
	"github.com/BrendanMoorehead/data-flow/internal/odds"
	"github.com/BrendanMoorehead/data-flow/internal/provider"
)

const (
	sportNBA           = "basketball_nba"
	marketKeyMoneyline = "h2h"
)

var leagueBySport = map[string]string{sportNBA: "NBA"}

var bookBySportsbookName = map[string]canonical.BookID{
	"DraftKings": canonical.BookDraftKings,
	"FanDuel":    canonical.BookFanDuel,
}

type Client struct {
	baseURL    string
	httpClient *http.Client
}

func New(baseURL string, httpClient *http.Client) *Client {
	return &Client{baseURL: baseURL, httpClient: httpClient}
}

func (c *Client) Source() canonical.SourceID {
	return canonical.SourceAggregator
}

func (c *Client) Slices(context.Context) ([]provider.SliceKey, error) {
	return []provider.SliceKey{sportNBA}, nil
}

func (c *Client) Poll(ctx context.Context, slice provider.SliceKey) (provider.Snapshot, error) {
	games, err := c.fetchAllPages(ctx, string(slice))
	if err != nil {
		return provider.Snapshot{}, err
	}
	return snapshotFromGames(games), nil
}

func (c *Client) fetchAllPages(ctx context.Context, sport string) ([]gameJSON, error) {
	var games []gameJSON
	for page := 1; ; page++ {
		response, err := c.fetchPage(ctx, sport, page)
		if err != nil {
			return nil, err
		}
		games = append(games, response.Data...)
		if page >= response.TotalPages {
			return games, nil
		}
	}
}

func (c *Client) fetchPage(ctx context.Context, sport string, page int) (pageJSON, error) {
	query := url.Values{"sport": {sport}, "page": {strconv.Itoa(page)}}
	var response pageJSON
	err := provider.GetJSON(ctx, c.httpClient, c.baseURL+"/v1/odds?"+query.Encode(), &response)
	return response, err
}

type quoteContext struct {
	gameID     string
	event      canonical.ProviderEvent
	book       canonical.BookID
	observedAt time.Time
}

func snapshotFromGames(games []gameJSON) provider.Snapshot {
	var builder provider.SnapshotBuilder
	for _, game := range games {
		addGame(&builder, game)
	}
	return builder.Snapshot()
}

func addGame(builder *provider.SnapshotBuilder, game gameJSON) {
	event, err := providerEventFrom(game)
	if err != nil {
		builder.Reject("game %s: %v", game.ID, err)
		return
	}
	for _, bookmaker := range game.Bookmakers {
		addBookmaker(builder, game.ID, event, bookmaker)
	}
}

func addBookmaker(builder *provider.SnapshotBuilder, gameID string, event canonical.ProviderEvent, bookmaker bookmakerJSON) {
	book, known := bookBySportsbookName[bookmaker.Key]
	if !known {
		builder.Reject("game %s: unknown sportsbook %q", gameID, bookmaker.Key)
		return
	}
	quote := quoteContext{gameID: gameID, event: event, book: book, observedAt: bookmaker.LastUpdate}
	for _, market := range bookmaker.Markets {
		if market.Key == marketKeyMoneyline {
			addMoneylineOutcomes(builder, quote, market.Outcomes)
		}
	}
}

func addMoneylineOutcomes(builder *provider.SnapshotBuilder, quote quoteContext, outcomes []outcomeJSON) {
	for _, outcome := range outcomes {
		observation, err := moneylineObservation(quote, outcome)
		if err != nil {
			builder.Reject("game %s, %s: %v", quote.gameID, quote.book, err)
			continue
		}
		builder.Add(observation)
	}
}

func moneylineObservation(quote quoteContext, outcome outcomeJSON) (canonical.Observation, error) {
	side, err := sideForTeam(quote.event, outcome.Name)
	if err != nil {
		return canonical.Observation{}, err
	}
	price, err := odds.NewDecimal(outcome.Price)
	if err != nil {
		return canonical.Observation{}, err
	}
	return canonical.Observation{
		Source:     canonical.SourceAggregator,
		Book:       quote.book,
		Event:      quote.event,
		Market:     canonical.MarketMoneyline,
		Side:       side,
		Price:      price,
		RawPrice:   strconv.FormatFloat(outcome.Price, 'f', -1, 64),
		RawFormat:  canonical.FormatDecimal,
		ObservedAt: quote.observedAt,
	}, nil
}

func providerEventFrom(game gameJSON) (canonical.ProviderEvent, error) {
	league, known := leagueBySport[game.Sport]
	if !known {
		return canonical.ProviderEvent{}, fmt.Errorf("unknown sport %q", game.Sport)
	}
	return canonical.ProviderEvent{
		ProviderEventID: game.ID,
		League:          league,
		HomeTeam:        game.HomeTeam,
		AwayTeam:        game.AwayTeam,
		StartsAt:        game.CommenceTime,
	}, nil
}

func sideForTeam(event canonical.ProviderEvent, teamName string) (canonical.Side, error) {
	switch teamName {
	case event.HomeTeam:
		return canonical.SideHome, nil
	case event.AwayTeam:
		return canonical.SideAway, nil
	default:
		return "", fmt.Errorf("outcome %q matches neither team", teamName)
	}
}
