package draftkings

import "time"

type eventListJSON struct {
	Events []eventSummaryJSON `json:"events"`
}

type eventSummaryJSON struct {
	EventID string `json:"eventId"`
}

type eventOffersJSON struct {
	Event  eventJSON   `json:"event"`
	Offers []offerJSON `json:"offers"`
}

type eventJSON struct {
	EventID   string    `json:"eventId"`
	League    string    `json:"league"`
	HomeTeam  string    `json:"homeTeam"`
	AwayTeam  string    `json:"awayTeam"`
	StartDate time.Time `json:"startDate"`
}

type offerJSON struct {
	Type      string        `json:"type"`
	Status    string        `json:"status"`
	UpdatedAt time.Time     `json:"updatedAt"`
	Outcomes  []outcomeJSON `json:"outcomes"`
}

type outcomeJSON struct {
	Participant  string `json:"participant"`
	Role         string `json:"role"`
	OddsAmerican string `json:"oddsAmerican"`
}
