package store

import (
	"context"
	"fmt"
	"time"

	"github.com/BrendanMoorehead/data-flow/internal/canonical"
)

type QuarantineRecord struct {
	Source        canonical.SourceID
	Slice         string
	Reason        string
	Raw           string
	QuarantinedAt time.Time
}

func (s *Store) Quarantine(ctx context.Context, record QuarantineRecord) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO quarantined_observations (source, slice, reason, raw, quarantined_at) VALUES (?, ?, ?, ?, ?)`,
		record.Source, record.Slice, record.Reason, record.Raw, toMillis(record.QuarantinedAt))
	if err != nil {
		return fmt.Errorf("quarantine observation: %w", err)
	}
	return nil
}

func (s *Store) QuarantineCountsBySource(ctx context.Context) (map[canonical.SourceID]int, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT source, COUNT(*) FROM quarantined_observations GROUP BY source`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	counts := make(map[canonical.SourceID]int)
	for rows.Next() {
		var source canonical.SourceID
		var count int
		if err := rows.Scan(&source, &count); err != nil {
			return nil, err
		}
		counts[source] = count
	}
	return counts, rows.Err()
}
