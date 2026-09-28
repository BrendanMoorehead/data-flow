package fanduel

type eventsJSON struct {
	Events []eventJSON `json:"events"`
}

type eventJSON struct {
	ID        string       `json:"id"`
	StartTime int64        `json:"startTime"`
	HomeTeam  teamJSON     `json:"homeTeam"`
	AwayTeam  teamJSON     `json:"awayTeam"`
	Markets   []marketJSON `json:"markets"`
}

type teamJSON struct {
	Nickname string `json:"nickname"`
}

type marketJSON struct {
	MarketType string       `json:"marketType"`
	Runners    []runnerJSON `json:"runners"`
}

type runnerJSON struct {
	Side         string `json:"side"`
	Status       string `json:"status"`
	AmericanOdds *int   `json:"americanOdds"`
}
