package draftkings

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/BrendanMoorehead/data-flow/internal/canonical"
	"github.com/BrendanMoorehead/data-flow/internal/odds"
	"github.com/BrendanMoorehead/data-flow/internal/provider"
)

const (
	leagueNBA          = "NBA"
	offerTypeMoneyline = "MONEYLINE"
)

var sideByRole = map[string]canonical.Side{
	"home": canonical.SideHome,
	"away": canonical.SideAway,
}

type Client struct {
	baseURL    string
	httpClient *http.Client
}

func New(baseURL string, httpClient *http.Client) *Client {
	return &Client{baseURL: baseURL, httpClient: httpClient}
}

func (c *Client) Source() canonical.SourceID {
	return canonical.SourceDraftKingsDirect
}

func (c *Client) Slices(ctx context.Context) ([]provider.SliceKey, error) {
	query := url.Values{"league": {leagueNBA}}
	var response eventListJSON
	if err := provider.GetJSON(ctx, c.httpClient, c.baseURL+"/api/events?"+query.Encode(), &response); err != nil {
		return nil, err
	}
	return sliceKeysFrom(response.Events), nil
}

func (c *Client) Poll(ctx context.Context, slice provider.SliceKey) (provider.Snapshot, error) {
	var response eventOffersJSON
	offersURL := c.baseURL + "/api/events/" + url.PathEscape(string(slice)) + "/offers"
	if err := provider.GetJSON(ctx, c.httpClient, offersURL, &response); err != nil {
		return provider.Snapshot{}, err
	}
	return snapshotFromEventOffers(response), nil
}

func sliceKeysFrom(events []eventSummaryJSON) []provider.SliceKey {
	keys := make([]provider.SliceKey, 0, len(events))
	for _, event := range events {
		keys = append(keys, provider.SliceKey(event.EventID))
	}
	return keys
}

func snapshotFromEventOffers(response eventOffersJSON) provider.Snapshot {
	var builder provider.SnapshotBuilder
	event := providerEventFrom(response.Event)
	for _, offer := range response.Offers {
		if offer.Type == offerTypeMoneyline {
			addMoneylineOutcomes(&builder, event, offer)
		}
	}
	return builder.Snapshot()
}

func addMoneylineOutcomes(builder *provider.SnapshotBuilder, event canonical.ProviderEvent, offer offerJSON) {
	for _, outcome := range offer.Outcomes {
		observation, err := moneylineObservation(event, offer.UpdatedAt, outcome)
		if err != nil {
			builder.Reject("event %s: %v", event.ProviderEventID, err)
			continue
		}
		builder.Add(observation)
	}
}

func moneylineObservation(event canonical.ProviderEvent, updatedAt time.Time, outcome outcomeJSON) (canonical.Observation, error) {
	side, known := sideByRole[outcome.Role]
	if !known {
		return canonical.Observation{}, fmt.Errorf("unknown outcome role %q", outcome.Role)
	}
	price, err := odds.ParseAmerican(outcome.OddsAmerican)
	if err != nil {
		return canonical.Observation{}, err
	}
	return canonical.Observation{
		Source:     canonical.SourceDraftKingsDirect,
		Book:       canonical.BookDraftKings,
		Event:      event,
		Market:     canonical.MarketMoneyline,
		Side:       side,
		Price:      price.Decimal(),
		RawPrice:   outcome.OddsAmerican,
		RawFormat:  canonical.FormatAmerican,
		ObservedAt: updatedAt,
	}, nil
}

func providerEventFrom(event eventJSON) canonical.ProviderEvent {
	return canonical.ProviderEvent{
		ProviderEventID: event.EventID,
		League:          event.League,
		HomeTeam:        event.HomeTeam,
		AwayTeam:        event.AwayTeam,
		StartsAt:        event.StartDate,
	}
}
