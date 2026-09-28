package fanduel

import (
	"context"
	"errors"
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
	sliceNBA              = "nba"
	marketTypeMoneyline   = "MONEYLINE"
	runnerStatusSuspended = "SUSPENDED"
)

var errMissingPrice = errors.New("runner has no price")

var leagueBySlice = map[provider.SliceKey]string{sliceNBA: "NBA"}

var sideByRunnerSide = map[string]canonical.Side{
	"HOME": canonical.SideHome,
	"AWAY": canonical.SideAway,
}

type Client struct {
	baseURL    string
	httpClient *http.Client
}

func New(baseURL string, httpClient *http.Client) *Client {
	return &Client{baseURL: baseURL, httpClient: httpClient}
}

func (c *Client) Source() canonical.SourceID {
	return canonical.SourceFanDuelDirect
}

func (c *Client) Slices(context.Context) ([]provider.SliceKey, error) {
	return []provider.SliceKey{sliceNBA}, nil
}

func (c *Client) Poll(ctx context.Context, slice provider.SliceKey) (provider.Snapshot, error) {
	league, known := leagueBySlice[slice]
	if !known {
		return provider.Snapshot{}, fmt.Errorf("unknown fanduel slice %q", slice)
	}
	query := url.Values{"market": {"moneyline"}}
	eventsURL := c.baseURL + "/sportsbook/v1/leagues/" + url.PathEscape(string(slice)) + "/events?" + query.Encode()
	var response eventsJSON
	if err := provider.GetJSON(ctx, c.httpClient, eventsURL, &response); err != nil {
		return provider.Snapshot{}, err
	}
	return snapshotFromEvents(league, response.Events), nil
}

func snapshotFromEvents(league string, events []eventJSON) provider.Snapshot {
	var builder provider.SnapshotBuilder
	for _, event := range events {
		addEvent(&builder, providerEventFrom(league, event), event.Markets)
	}
	return builder.Snapshot()
}

func addEvent(builder *provider.SnapshotBuilder, event canonical.ProviderEvent, markets []marketJSON) {
	for _, market := range markets {
		if market.MarketType != marketTypeMoneyline {
			continue
		}
		for _, runner := range market.Runners {
			observation, err := moneylineObservation(event, runner)
			if err != nil {
				builder.Reject(runner, "event %s: %v", event.ProviderEventID, err)
				continue
			}
			builder.Add(observation)
		}
	}
}

func moneylineObservation(event canonical.ProviderEvent, runner runnerJSON) (canonical.Observation, error) {
	side, known := sideByRunnerSide[runner.Side]
	if !known {
		return canonical.Observation{}, fmt.Errorf("unknown runner side %q", runner.Side)
	}
	if runner.AmericanOdds == nil {
		return canonical.Observation{}, fmt.Errorf("%w (%s side)", errMissingPrice, runner.Side)
	}
	price, err := odds.NewAmerican(*runner.AmericanOdds)
	if err != nil {
		return canonical.Observation{}, err
	}
	return canonical.Observation{
		Source:    canonical.SourceFanDuelDirect,
		Book:      canonical.BookFanDuel,
		Event:     event,
		Market:    canonical.MarketMoneyline,
		Side:      side,
		Status:    statusOf(runner),
		Price:     price.Decimal(),
		RawPrice:  strconv.Itoa(*runner.AmericanOdds),
		RawFormat: canonical.FormatAmerican,
	}, nil
}

func statusOf(runner runnerJSON) canonical.MarketStatus {
	if runner.Status == runnerStatusSuspended {
		return canonical.StatusOffBoard
	}
	return canonical.StatusOpen
}

func providerEventFrom(league string, event eventJSON) canonical.ProviderEvent {
	return canonical.ProviderEvent{
		ProviderEventID: event.ID,
		League:          league,
		HomeTeam:        event.HomeTeam.Nickname,
		AwayTeam:        event.AwayTeam.Nickname,
		StartsAt:        time.Unix(event.StartTime, 0).UTC(),
	}
}
