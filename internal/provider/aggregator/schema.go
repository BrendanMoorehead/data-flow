package aggregator

import "time"

type pageJSON struct {
	Page       int        `json:"page"`
	TotalPages int        `json:"total_pages"`
	Data       []gameJSON `json:"data"`
}

type gameJSON struct {
	ID           string          `json:"id"`
	Sport        string          `json:"sport"`
	CommenceTime time.Time       `json:"commence_time"`
	HomeTeam     string          `json:"home_team"`
	AwayTeam     string          `json:"away_team"`
	Bookmakers   []bookmakerJSON `json:"bookmakers"`
}

type bookmakerJSON struct {
	Key        string       `json:"key"`
	LastUpdate time.Time    `json:"last_update"`
	Markets    []marketJSON `json:"markets"`
}

type marketJSON struct {
	Key      string        `json:"key"`
	Outcomes []outcomeJSON `json:"outcomes"`
}

type outcomeJSON struct {
	Name  string  `json:"name"`
	Price float64 `json:"price"`
}
