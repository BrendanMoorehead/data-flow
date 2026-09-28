package store

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/BrendanMoorehead/data-flow/internal/canonical"
)

type PollAttempt struct {
	Source        canonical.SourceID
	Slice         string
	At            time.Time
	NextAttemptAt time.Time
	Err           error
}

type SliceHealth struct {
	Source              canonical.SourceID
	Slice               string
	LastAttemptAt       time.Time
	LastSuccessAt       *time.Time
	LastError           string
	LastErrorAt         *time.Time
	ConsecutiveFailures int
	NextAttemptAt       time.Time
}

func (s *Store) RecordPollAttempt(ctx context.Context, attempt PollAttempt) error {
	if attempt.Err == nil {
		return s.recordPollSuccess(ctx, attempt)
	}
	return s.recordPollFailure(ctx, attempt)
}

func (s *Store) recordPollSuccess(ctx context.Context, attempt PollAttempt) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO slice_health (source, slice, last_attempt_at, last_success_at, consecutive_failures, next_attempt_at)
		 VALUES (?, ?, ?, ?, 0, ?)
		 ON CONFLICT (source, slice) DO UPDATE SET
		     last_attempt_at = excluded.last_attempt_at,
		     last_success_at = excluded.last_success_at,
		     consecutive_failures = 0,
		     next_attempt_at = excluded.next_attempt_at`,
		attempt.Source, attempt.Slice, toMillis(attempt.At), toMillis(attempt.At), toMillis(attempt.NextAttemptAt))
	if err != nil {
		return fmt.Errorf("record poll success: %w", err)
	}
	return nil
}

func (s *Store) recordPollFailure(ctx context.Context, attempt PollAttempt) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO slice_health (source, slice, last_attempt_at, last_error, last_error_at, consecutive_failures, next_attempt_at)
		 VALUES (?, ?, ?, ?, ?, 1, ?)
		 ON CONFLICT (source, slice) DO UPDATE SET
		     last_attempt_at = excluded.last_attempt_at,
		     last_error = excluded.last_error,
		     last_error_at = excluded.last_error_at,
		     consecutive_failures = slice_health.consecutive_failures + 1,
		     next_attempt_at = excluded.next_attempt_at`,
		attempt.Source, attempt.Slice, toMillis(attempt.At), attempt.Err.Error(), toMillis(attempt.At),
		toMillis(attempt.NextAttemptAt))
	if err != nil {
		return fmt.Errorf("record poll failure: %w", err)
	}
	return nil
}

func (s *Store) ListSliceHealth(ctx context.Context) ([]SliceHealth, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT source, slice, last_attempt_at, last_success_at, COALESCE(last_error, ''), last_error_at,
		        consecutive_failures, next_attempt_at
		 FROM slice_health
		 ORDER BY source, slice`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var healths []SliceHealth
	for rows.Next() {
		health, err := scanSliceHealth(rows)
		if err != nil {
			return nil, err
		}
		healths = append(healths, health)
	}
	return healths, rows.Err()
}

func scanSliceHealth(rows rowScanner) (SliceHealth, error) {
	var health SliceHealth
	var lastAttemptAt, nextAttemptAt int64
	var lastSuccessAt, lastErrorAt sql.NullInt64
	err := rows.Scan(&health.Source, &health.Slice, &lastAttemptAt, &lastSuccessAt,
		&health.LastError, &lastErrorAt, &health.ConsecutiveFailures, &nextAttemptAt)
	health.LastAttemptAt = fromMillis(lastAttemptAt)
	health.LastSuccessAt = fromNullableMillis(lastSuccessAt)
	health.LastErrorAt = fromNullableMillis(lastErrorAt)
	health.NextAttemptAt = fromMillis(nextAttemptAt)
	return health, err
}
