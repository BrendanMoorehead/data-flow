package sim

import (
	"fmt"
	"net/http"

	"github.com/BrendanMoorehead/data-flow/internal/httpserve"
)

const (
	fanDuelLeagueNBA        = "nba"
	fanDuelMarketMoneyline  = "MONEYLINE"
	fanDuelRunnerActive     = "ACTIVE"
	fanDuelRunnerSuspended  = "SUSPENDED"
	fanDuelEventIDOffset    = 9000
	fanDuelMissingPriceRate = 0.03
)

type fanDuelEvents struct {
	Events []fanDuelEvent `json:"events"`
}

type fanDuelEvent struct {
	ID        string          `json:"id"`
	StartTime int64           `json:"startTime"`
	HomeTeam  fanDuelTeam     `json:"homeTeam"`
	AwayTeam  fanDuelTeam     `json:"awayTeam"`
	Markets   []fanDuelMarket `json:"markets"`
}

type fanDuelTeam struct {
	Nickname string `json:"nickname"`
}

type fanDuelMarket struct {
	MarketType string          `json:"marketType"`
	Runners    []fanDuelRunner `json:"runners"`
}

type fanDuelRunner struct {
	Side         string `json:"side"`
	Status       string `json:"status"`
	AmericanOdds *int   `json:"americanOdds,omitempty"`
}

func (s *Simulator) FanDuelHandler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /sportsbook/v1/leagues/{league}/events", s.serveFanDuelEvents)
	return mux
}

func (s *Simulator) serveFanDuelEvents(rw http.ResponseWriter, r *http.Request) {
	if r.PathValue("league") != fanDuelLeagueNBA {
		httpserve.WriteError(rw, http.StatusNotFound, "no league "+r.PathValue("league"))
		return
	}
	views := s.world.snapshot()
	events := make([]fanDuelEvent, 0, len(views))
	for _, view := range views {
		events = append(events, s.fanDuelEventFrom(view))
	}
	httpserve.WriteJSON(rw, http.StatusOK, fanDuelEvents{Events: events})
}

func (s *Simulator) fanDuelEventFrom(view gameView) fanDuelEvent {
	quote := view.currentQuote(bookFanDuel)
	return fanDuelEvent{
		ID:        fmt.Sprintf("fd-%d", fanDuelEventIDOffset+view.number),
		StartTime: view.startsAt.Unix(),
		HomeTeam:  fanDuelTeam{Nickname: view.home.nickname},
		AwayTeam:  fanDuelTeam{Nickname: view.away.nickname},
		Markets: []fanDuelMarket{{
			MarketType: fanDuelMarketMoneyline,
			Runners: []fanDuelRunner{
				s.fanDuelRunner(view, sideHome, int(quote.home)),
				s.fanDuelRunner(view, sideAway, int(quote.away)),
			},
		}},
	}
}

func (s *Simulator) fanDuelRunner(view gameView, runnerSide side, americanOdds int) fanDuelRunner {
	runner := fanDuelRunner{Side: fanDuelSideName(runnerSide), Status: fanDuelRunnerActive, AmericanOdds: &americanOdds}
	if view.isSuspended(bookFanDuel, runnerSide, s.now()) {
		runner.Status = fanDuelRunnerSuspended
	}
	if s.chaos.backgroundChance(fanDuelMissingPriceRate) {
		runner.AmericanOdds = nil
	}
	return runner
}

func fanDuelSideName(runnerSide side) string {
	if runnerSide == sideHome {
		return "HOME"
	}
	return "AWAY"
}
