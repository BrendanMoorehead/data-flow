package identity

import "github.com/BrendanMoorehead/data-flow/internal/canonical"

type teamFixture struct {
	id             canonical.TeamID
	name           string
	aggregatorName string
	draftKingsName string
	fanDuelName    string
}

var nbaTeamFixtures = []teamFixture{
	{id: "nba-lal", name: "Los Angeles Lakers", aggregatorName: "Los Angeles Lakers", draftKingsName: "LA Lakers", fanDuelName: "Lakers"},
	{id: "nba-bos", name: "Boston Celtics", aggregatorName: "Boston Celtics", draftKingsName: "BOS Celtics", fanDuelName: "Celtics"},
	{id: "nba-gsw", name: "Golden State Warriors", aggregatorName: "Golden State Warriors", draftKingsName: "GS Warriors", fanDuelName: "Warriors"},
	{id: "nba-den", name: "Denver Nuggets", aggregatorName: "Denver Nuggets", draftKingsName: "DEN Nuggets", fanDuelName: "Nuggets"},
	{id: "nba-mil", name: "Milwaukee Bucks", aggregatorName: "Milwaukee Bucks", draftKingsName: "MIL Bucks", fanDuelName: "Bucks"},
	{id: "nba-phi", name: "Philadelphia 76ers", aggregatorName: "Philadelphia 76ers", draftKingsName: "PHI 76ers", fanDuelName: "76ers"},
	{id: "nba-nyk", name: "New York Knicks", aggregatorName: "New York Knicks", draftKingsName: "NY Knicks", fanDuelName: "Knicks"},
	{id: "nba-mia", name: "Miami Heat", aggregatorName: "Miami Heat", draftKingsName: "MIA Heat", fanDuelName: "Heat"},
	{id: "nba-dal", name: "Dallas Mavericks", aggregatorName: "Dallas Mavericks", draftKingsName: "DAL Mavericks", fanDuelName: "Mavericks"},
	{id: "nba-phx", name: "Phoenix Suns", aggregatorName: "Phoenix Suns", draftKingsName: "PHO Suns", fanDuelName: "Suns"},
	{id: "nba-okc", name: "Oklahoma City Thunder", aggregatorName: "Oklahoma City Thunder", draftKingsName: "OKC Thunder", fanDuelName: "Thunder"},
	{id: "nba-min", name: "Minnesota Timberwolves", aggregatorName: "Minnesota Timberwolves", draftKingsName: "MIN Timberwolves", fanDuelName: "Timberwolves"},
}

func SeedTeams() []canonical.Team {
	teams := make([]canonical.Team, 0, len(nbaTeamFixtures))
	for _, fixture := range nbaTeamFixtures {
		teams = append(teams, canonical.Team{ID: fixture.id, Name: fixture.name})
	}
	return teams
}

func SeedAliases() []canonical.TeamAlias {
	aliases := make([]canonical.TeamAlias, 0, 3*len(nbaTeamFixtures))
	for _, fixture := range nbaTeamFixtures {
		aliases = append(aliases,
			canonical.TeamAlias{Source: canonical.SourceAggregator, Alias: fixture.aggregatorName, TeamID: fixture.id},
			canonical.TeamAlias{Source: canonical.SourceDraftKingsDirect, Alias: fixture.draftKingsName, TeamID: fixture.id},
			canonical.TeamAlias{Source: canonical.SourceFanDuelDirect, Alias: fixture.fanDuelName, TeamID: fixture.id},
		)
	}
	return aliases
}
