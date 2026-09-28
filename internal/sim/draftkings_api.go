package sim

import (
	"fmt"
	"net/http"
	"time"

	"github.com/BrendanMoorehead/data-flow/internal/httpserve"
)

const (
	draftKingsLeagueNBA       = "NBA"
	draftKingsOfferMoneyline  = "MONEYLINE"
	draftKingsOfferStatusOpen = "OPEN"
	draftKingsEventIDOffset   = 5500
)

type draftKingsEventList struct {
	Events []draftKingsEventSummary `json:"events"`
}

type draftKingsEventSummary struct {
	EventID string `json:"eventId"`
	Name    string `json:"name"`
}

type draftKingsEventOffers struct {
	Event  draftKingsEvent   `json:"event"`
	Offers []draftKingsOffer `json:"offers"`
}

type draftKingsEvent struct {
	EventID   string    `json:"eventId"`
	League    string    `json:"league"`
	HomeTeam  string    `json:"homeTeam"`
	AwayTeam  string    `json:"awayTeam"`
	StartDate time.Time `json:"startDate"`
}

type draftKingsOffer struct {
	Type      string              `json:"type"`
	Status    string              `json:"status"`
	UpdatedAt time.Time           `json:"updatedAt"`
	Outcomes  []draftKingsOutcome `json:"outcomes"`
}

type draftKingsOutcome struct {
	Participant  string `json:"participant"`
	Role         string `json:"role"`
	OddsAmerican string `json:"oddsAmerican"`
}

func (w *World) DraftKingsHandler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/events", w.serveDraftKingsEvents)
	mux.HandleFunc("GET /api/events/{eventID}/offers", w.serveDraftKingsOffers)
	return mux
}

func (w *World) serveDraftKingsEvents(rw http.ResponseWriter, r *http.Request) {
	events := []draftKingsEventSummary{}
	if r.URL.Query().Get("league") == draftKingsLeagueNBA {
		for _, view := range w.snapshot() {
			events = append(events, draftKingsEventSummary{
				EventID: draftKingsEventID(view),
				Name:    view.away.shortName + " @ " + view.home.shortName,
			})
		}
	}
	httpserve.WriteJSON(rw, http.StatusOK, draftKingsEventList{Events: events})
}

func (w *World) serveDraftKingsOffers(rw http.ResponseWriter, r *http.Request) {
	eventID := r.PathValue("eventID")
	view, found := findGameView(w.snapshot(), eventID)
	if !found {
		httpserve.WriteError(rw, http.StatusNotFound, "no event "+eventID)
		return
	}
	httpserve.WriteJSON(rw, http.StatusOK, draftKingsEventOffersFrom(view))
}

func findGameView(views []gameView, draftKingsID string) (gameView, bool) {
	for _, view := range views {
		if draftKingsEventID(view) == draftKingsID {
			return view, true
		}
	}
	return gameView{}, false
}

func draftKingsEventOffersFrom(view gameView) draftKingsEventOffers {
	quote := view.quotes[bookDraftKings]
	return draftKingsEventOffers{
		Event: draftKingsEvent{
			EventID:   draftKingsEventID(view),
			League:    draftKingsLeagueNBA,
			HomeTeam:  view.home.shortName,
			AwayTeam:  view.away.shortName,
			StartDate: view.startsAt,
		},
		Offers: []draftKingsOffer{{
			Type:      draftKingsOfferMoneyline,
			Status:    draftKingsOfferStatusOpen,
			UpdatedAt: quote.updatedAt,
			Outcomes: []draftKingsOutcome{
				{Participant: view.home.shortName, Role: "home", OddsAmerican: quote.home.String()},
				{Participant: view.away.shortName, Role: "away", OddsAmerican: quote.away.String()},
			},
		}},
	}
}

func draftKingsEventID(view gameView) string {
	return fmt.Sprintf("dk-%d", draftKingsEventIDOffset+view.number)
}
