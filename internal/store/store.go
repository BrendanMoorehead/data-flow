package store

import (
	"context"
	"database/sql"
	_ "embed"
	"fmt"
	"time"

	_ "modernc.org/sqlite"
)

//go:embed schema.sql
var schema string

type rowScanner interface {
	Scan(dest ...any) error
}

type Store struct {
	db *sql.DB
}

func Open(ctx context.Context, path string) (*Store, error) {
	db, err := sql.Open("sqlite", dataSourceName(path))
	if err != nil {
		return nil, fmt.Errorf("open sqlite %s: %w", path, err)
	}
	db.SetMaxOpenConns(1)
	if _, err := db.ExecContext(ctx, schema); err != nil {
		db.Close()
		return nil, fmt.Errorf("apply schema: %w", err)
	}
	return &Store{db: db}, nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

func dataSourceName(path string) string {
	return "file:" + path + "?_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)"
}

func toMillis(t time.Time) int64 {
	return t.UnixMilli()
}

func fromMillis(millis int64) time.Time {
	return time.UnixMilli(millis).UTC()
}

func fromNullableMillis(millis sql.NullInt64) *time.Time {
	if !millis.Valid {
		return nil
	}
	t := fromMillis(millis.Int64)
	return &t
}
