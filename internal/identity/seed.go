package identity

import (
	"encoding/csv"
	"errors"
	"fmt"
	"io/fs"
	"slices"

	"github.com/BrendanMoorehead/data-flow/fixtures"
	"github.com/BrendanMoorehead/data-flow/internal/canonical"
)

var (
	ErrUnexpectedHeader = errors.New("unexpected csv header")
	ErrMalformedRow     = errors.New("malformed csv row")
	ErrAliasForNoTeam   = errors.New("alias refers to an unknown team")
	ErrDuplicateAlias   = errors.New("alias is listed twice for the same source")
)

var (
	teamsHeader   = []string{"team_id", "name"}
	aliasesHeader = []string{"team_id", "source", "alias"}
)

type Seed struct {
	Teams   []canonical.Team
	Aliases []canonical.TeamAlias
}

func LoadDefaultSeed() (Seed, error) {
	return LoadSeed(fixtures.Files, fixtures.TeamsFile, fixtures.TeamAliasesFile)
}

func LoadSeed(files fs.FS, teamsPath, aliasesPath string) (Seed, error) {
	teams, err := readTeams(files, teamsPath)
	if err != nil {
		return Seed{}, err
	}
	aliases, err := readAliases(files, aliasesPath)
	if err != nil {
		return Seed{}, err
	}
	if err := validateAliases(teams, aliases); err != nil {
		return Seed{}, fmt.Errorf("%s: %w", aliasesPath, err)
	}
	return Seed{Teams: teams, Aliases: aliases}, nil
}

func readTeams(files fs.FS, path string) ([]canonical.Team, error) {
	rows, err := readRows(files, path, teamsHeader)
	if err != nil {
		return nil, err
	}
	teams := make([]canonical.Team, 0, len(rows))
	for _, row := range rows {
		teams = append(teams, canonical.Team{ID: canonical.TeamID(row[0]), Name: row[1]})
	}
	return teams, nil
}

func readAliases(files fs.FS, path string) ([]canonical.TeamAlias, error) {
	rows, err := readRows(files, path, aliasesHeader)
	if err != nil {
		return nil, err
	}
	aliases := make([]canonical.TeamAlias, 0, len(rows))
	for _, row := range rows {
		aliases = append(aliases, canonical.TeamAlias{TeamID: canonical.TeamID(row[0]), Source: canonical.SourceID(row[1]), Alias: row[2]})
	}
	return aliases, nil
}

func readRows(files fs.FS, path string, header []string) ([][]string, error) {
	file, err := files.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", path, err)
	}
	defer file.Close()

	reader := csv.NewReader(file)
	reader.FieldsPerRecord = len(header)
	records, err := reader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("%s: %w: %v", path, ErrMalformedRow, err)
	}
	if len(records) == 0 || !slices.Equal(records[0], header) {
		return nil, fmt.Errorf("%s: %w, want %v", path, ErrUnexpectedHeader, header)
	}
	return records[1:], nil
}

func validateAliases(teams []canonical.Team, aliases []canonical.TeamAlias) error {
	knownTeams := make(map[canonical.TeamID]bool, len(teams))
	for _, team := range teams {
		knownTeams[team.ID] = true
	}
	seen := make(map[canonical.TeamAlias]bool, len(aliases))
	for _, alias := range aliases {
		if !knownTeams[alias.TeamID] {
			return fmt.Errorf("%w: %s", ErrAliasForNoTeam, alias.TeamID)
		}
		sourceSpelling := canonical.TeamAlias{Source: alias.Source, Alias: alias.Alias}
		if seen[sourceSpelling] {
			return fmt.Errorf("%w: %s %q", ErrDuplicateAlias, alias.Source, alias.Alias)
		}
		seen[sourceSpelling] = true
	}
	return nil
}
