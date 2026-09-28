package sim

import (
	"fmt"
	"math"
	"net/http"
	"strconv"
	"time"

	"github.com/BrendanMoorehead/data-flow/internal/httpserve"
)

const (
	aggregatorSportNBA        = "basketball_nba"
	aggregatorPageSize        = 4
	aggregatorMarketMoneyline = "h2h"
	aggregatorEventIDOffset   = 1000
)

type aggregatorPage struct {
	Page       int              `json:"page"`
	TotalPages int              `json:"total_pages"`
	Data       []aggregatorGame `json:"data"`
}

type aggregatorGame struct {
	ID           string                `json:"id"`
	Sport        string                `json:"sport"`
	CommenceTime time.Time             `json:"commence_time"`
	HomeTeam     string                `json:"home_team"`
	AwayTeam     string                `json:"away_team"`
	Bookmakers   []aggregatorBookmaker `json:"bookmakers"`
}

type aggregatorBookmaker struct {
	Key        string             `json:"key"`
	LastUpdate time.Time          `json:"last_update"`
	Markets    []aggregatorMarket `json:"markets"`
}

type aggregatorMarket struct {
	Key      string              `json:"key"`
	Outcomes []aggregatorOutcome `json:"outcomes"`
}

type aggregatorOutcome struct {
	Name  string  `json:"name"`
	Price float64 `json:"price"`
}

func (s *Simulator) AggregatorHandler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /v1/odds", s.serveAggregatorOdds)
	return s.withAggregatorProblems(mux)
}

func (s *Simulator) serveAggregatorOdds(rw http.ResponseWriter, r *http.Request) {
	page, err := pageNumber(r)
	if err != nil {
		httpserve.WriteError(rw, http.StatusBadRequest, err.Error())
		return
	}
	var games []aggregatorGame
	if r.URL.Query().Get("sport") == aggregatorSportNBA {
		games = aggregatorGamesFrom(s.world.snapshot(), s.aggregatorPricesAsOf())
	}
	pageGames, totalPages := paginate(games, page, aggregatorPageSize)
	httpserve.WriteJSON(rw, http.StatusOK, aggregatorPage{Page: page, TotalPages: totalPages, Data: pageGames})
}

func (s *Simulator) aggregatorPricesAsOf() time.Time {
	if s.chaos.isActive(ScenarioAggregatorLag) {
		return s.now().Add(-aggregatorLagDelay)
	}
	return s.now()
}

func aggregatorGamesFrom(views []gameView, pricesAsOf time.Time) []aggregatorGame {
	games := make([]aggregatorGame, 0, len(views))
	for _, view := range views {
		games = append(games, aggregatorGameFrom(view, pricesAsOf))
	}
	return games
}

func aggregatorGameFrom(view gameView, pricesAsOf time.Time) aggregatorGame {
	bookmakers := make([]aggregatorBookmaker, 0, len(books))
	for _, book := range books {
		bookmakers = append(bookmakers, aggregatorBookmakerFrom(view.quoteAsOf(book, pricesAsOf), view, book))
	}
	return aggregatorGame{
		ID:           fmt.Sprintf("agg-%d", aggregatorEventIDOffset+view.number),
		Sport:        aggregatorSportNBA,
		CommenceTime: view.startsAt,
		HomeTeam:     view.home.fullName,
		AwayTeam:     view.away.fullName,
		Bookmakers:   bookmakers,
	}
}

func aggregatorBookmakerFrom(quote moneylineQuote, view gameView, book string) aggregatorBookmaker {
	return aggregatorBookmaker{
		Key:        book,
		LastUpdate: quote.updatedAt,
		Markets: []aggregatorMarket{{
			Key: aggregatorMarketMoneyline,
			Outcomes: []aggregatorOutcome{
				{Name: view.home.fullName, Price: roundToCents(float64(quote.home.Decimal()))},
				{Name: view.away.fullName, Price: roundToCents(float64(quote.away.Decimal()))},
			},
		}},
	}
}

func pageNumber(r *http.Request) (int, error) {
	text := r.URL.Query().Get("page")
	if text == "" {
		return 1, nil
	}
	page, err := strconv.Atoi(text)
	if err != nil || page < 1 {
		return 0, fmt.Errorf("page must be a positive integer, got %q", text)
	}
	return page, nil
}

func paginate[T any](items []T, page, pageSize int) ([]T, int) {
	totalPages := (len(items) + pageSize - 1) / pageSize
	start := min((page-1)*pageSize, len(items))
	end := min(start+pageSize, len(items))
	return items[start:end], totalPages
}

func roundToCents(value float64) float64 {
	return math.Round(value*100) / 100
}
