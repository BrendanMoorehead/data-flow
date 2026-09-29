package fixtures

import "embed"

//go:embed *.csv
var Files embed.FS

const (
	TeamsFile       = "teams.csv"
	TeamAliasesFile = "team_aliases.csv"
)
