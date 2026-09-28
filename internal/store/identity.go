package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/BrendanMoorehead/data-flow/internal/canonical"
)

func (s *Store) SeedTeams(ctx context.Context, teams []canonical.Team, aliases []canonical.TeamAlias) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	for _, team := range teams {
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO teams (id, name) VALUES (?, ?) ON CONFLICT (id) DO UPDATE SET name = excluded.name`,
			team.ID, team.Name); err != nil {
			return fmt.Errorf("seed team %s: %w", team.ID, err)
		}
	}
	for _, alias := range aliases {
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO team_aliases (source, alias, team_id) VALUES (?, ?, ?)
			 ON CONFLICT (source, alias) DO UPDATE SET team_id = excluded.team_id`,
			alias.Source, alias.Alias, alias.TeamID); err != nil {
			return fmt.Errorf("seed alias %s/%s: %w", alias.Source, alias.Alias, err)
		}
	}
	return tx.Commit()
}

func (s *Store) TeamIDForAlias(ctx context.Context, source canonical.SourceID, alias string) (canonical.TeamID, bool, error) {
	var teamID canonical.TeamID
	err := s.db.QueryRowContext(ctx,
		`SELECT team_id FROM team_aliases WHERE source = ? AND alias = ?`, source, alias).Scan(&teamID)
	return teamID, scanFound(err), ignoreNoRows(err)
}

func (s *Store) EventIDForSourceRef(ctx context.Context, source canonical.SourceID, providerEventID string) (canonical.EventID, bool, error) {
	var eventID canonical.EventID
	err := s.db.QueryRowContext(ctx,
		`SELECT event_id FROM event_source_refs WHERE source = ? AND provider_event_id = ?`,
		source, providerEventID).Scan(&eventID)
	return eventID, scanFound(err), ignoreNoRows(err)
}

func (s *Store) FindEventByMatchup(ctx context.Context, matchup canonical.Matchup, startsFrom, startsTo time.Time) (canonical.EventID, bool, error) {
	var eventID canonical.EventID
	err := s.db.QueryRowContext(ctx,
		`SELECT id FROM events
		 WHERE home_team_id = ? AND away_team_id = ? AND starts_at BETWEEN ? AND ?
		 ORDER BY starts_at LIMIT 1`,
		matchup.Home, matchup.Away, toMillis(startsFrom), toMillis(startsTo)).Scan(&eventID)
	return eventID, scanFound(err), ignoreNoRows(err)
}

func (s *Store) CreateEvent(ctx context.Context, matchup canonical.Matchup, league string, startsAt time.Time) (canonical.EventID, error) {
	result, err := s.db.ExecContext(ctx,
		`INSERT INTO events (league, home_team_id, away_team_id, starts_at) VALUES (?, ?, ?, ?)`,
		league, matchup.Home, matchup.Away, toMillis(startsAt))
	if err != nil {
		return 0, fmt.Errorf("create event: %w", err)
	}
	eventID, err := result.LastInsertId()
	return canonical.EventID(eventID), err
}

func (s *Store) LinkSourceRef(ctx context.Context, source canonical.SourceID, providerEventID string, eventID canonical.EventID) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO event_source_refs (source, provider_event_id, event_id) VALUES (?, ?, ?)`,
		source, providerEventID, eventID)
	return err
}

func (s *Store) ListEvents(ctx context.Context) ([]canonical.Event, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT events.id, events.league, home.name, away.name, events.starts_at
		 FROM events
		 JOIN teams AS home ON home.id = events.home_team_id
		 JOIN teams AS away ON away.id = events.away_team_id
		 ORDER BY events.starts_at, events.id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []canonical.Event
	for rows.Next() {
		var event canonical.Event
		var startsAt int64
		if err := rows.Scan(&event.ID, &event.League, &event.HomeTeam, &event.AwayTeam, &startsAt); err != nil {
			return nil, err
		}
		event.StartsAt = fromMillis(startsAt)
		events = append(events, event)
	}
	return events, rows.Err()
}

func scanFound(err error) bool {
	return err == nil
}

func ignoreNoRows(err error) error {
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	return err
}
