package store

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/BrendanMoorehead/data-flow/internal/canonical"
)

type SliceHealth struct {
	Source              canonical.SourceID
	Slice               string
	LastAttemptAt       time.Time
	LastSuccessAt       *time.Time
	LastError           string
	LastErrorAt         *time.Time
	ConsecutiveFailures int
}

func (s *Store) RecordPollSuccess(ctx context.Context, source canonical.SourceID, slice string, at time.Time) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO slice_health (source, slice, last_attempt_at, last_success_at, consecutive_failures)
		 VALUES (?, ?, ?, ?, 0)
		 ON CONFLICT (source, slice) DO UPDATE SET
		     last_attempt_at = excluded.last_attempt_at,
		     last_success_at = excluded.last_success_at,
		     consecutive_failures = 0`,
		source, slice, toMillis(at), toMillis(at))
	if err != nil {
		return fmt.Errorf("record poll success: %w", err)
	}
	return nil
}

func (s *Store) RecordPollFailure(ctx context.Context, source canonical.SourceID, slice string, at time.Time, pollErr error) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO slice_health (source, slice, last_attempt_at, last_error, last_error_at, consecutive_failures)
		 VALUES (?, ?, ?, ?, ?, 1)
		 ON CONFLICT (source, slice) DO UPDATE SET
		     last_attempt_at = excluded.last_attempt_at,
		     last_error = excluded.last_error,
		     last_error_at = excluded.last_error_at,
		     consecutive_failures = slice_health.consecutive_failures + 1`,
		source, slice, toMillis(at), pollErr.Error(), toMillis(at))
	if err != nil {
		return fmt.Errorf("record poll failure: %w", err)
	}
	return nil
}

func (s *Store) ListSliceHealth(ctx context.Context) ([]SliceHealth, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT source, slice, last_attempt_at, last_success_at, COALESCE(last_error, ''), last_error_at, consecutive_failures
		 FROM slice_health
		 ORDER BY source, slice`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var healths []SliceHealth
	for rows.Next() {
		var health SliceHealth
		var lastAttemptAt int64
		var lastSuccessAt, lastErrorAt sql.NullInt64
		if err := rows.Scan(&health.Source, &health.Slice, &lastAttemptAt, &lastSuccessAt,
			&health.LastError, &lastErrorAt, &health.ConsecutiveFailures); err != nil {
			return nil, err
		}
		health.LastAttemptAt = fromMillis(lastAttemptAt)
		health.LastSuccessAt = fromNullableMillis(lastSuccessAt)
		health.LastErrorAt = fromNullableMillis(lastErrorAt)
		healths = append(healths, health)
	}
	return healths, rows.Err()
}
