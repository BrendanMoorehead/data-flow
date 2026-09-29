package identity

import (
	"errors"
	"testing"
	"testing/fstest"
)

const validTeams = "team_id,name\nnba-lal,Los Angeles Lakers\n"

func seedFiles(teams, aliases string) fstest.MapFS {
	return fstest.MapFS{
		"teams.csv":   {Data: []byte(teams)},
		"aliases.csv": {Data: []byte(aliases)},
	}
}

func TestDefaultSeedGivesEveryTeamAnAliasForEachProvider(t *testing.T) {
	seed, err := LoadDefaultSeed()
	if err != nil {
		t.Fatalf("load default seed: %v", err)
	}

	sourcesPerTeam := map[string]int{}
	for _, alias := range seed.Aliases {
		sourcesPerTeam[string(alias.TeamID)]++
	}
	for _, team := range seed.Teams {
		if sourcesPerTeam[string(team.ID)] != 3 {
			t.Errorf("%s has %d aliases, want one per provider (3)", team.ID, sourcesPerTeam[string(team.ID)])
		}
	}
}

func TestLoadSeedRejectsAliasForUnknownTeam(t *testing.T) {
	files := seedFiles(validTeams, "team_id,source,alias\nnba-xxx,aggregator,Nobody\n")

	if _, err := LoadSeed(files, "teams.csv", "aliases.csv"); !errors.Is(err, ErrAliasForNoTeam) {
		t.Errorf("error = %v, want ErrAliasForNoTeam", err)
	}
}

func TestLoadSeedRejectsDuplicateAliasForOneSource(t *testing.T) {
	files := seedFiles(validTeams, "team_id,source,alias\nnba-lal,aggregator,Lakers\nnba-lal,aggregator,Lakers\n")

	if _, err := LoadSeed(files, "teams.csv", "aliases.csv"); !errors.Is(err, ErrDuplicateAlias) {
		t.Errorf("error = %v, want ErrDuplicateAlias", err)
	}
}

func TestLoadSeedRejectsWrongHeader(t *testing.T) {
	files := seedFiles("id,team\nnba-lal,Los Angeles Lakers\n", "team_id,source,alias\n")

	if _, err := LoadSeed(files, "teams.csv", "aliases.csv"); !errors.Is(err, ErrUnexpectedHeader) {
		t.Errorf("error = %v, want ErrUnexpectedHeader", err)
	}
}
