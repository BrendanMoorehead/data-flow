package store

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/BrendanMoorehead/data-flow/internal/canonical"
	"github.com/BrendanMoorehead/data-flow/internal/odds"
)

type RecordResult int

const (
	RecordInserted RecordResult = iota
	RecordConfirmed
)

type ObservationRecord struct {
	EventID     canonical.EventID
	Slice       string
	Observation canonical.Observation
	ReceivedAt  time.Time
}

type SliceObservation struct {
	ContentKey string
	Latest     canonical.SourcePrice
}

const latestObservationPerKeyQuery = `
	SELECT content_key, event_id, book, market, side, source, status, decimal_price,
	       observed_at, last_confirmed_at, has_source_timestamp
	FROM (
	    SELECT *, ROW_NUMBER() OVER (
	        PARTITION BY event_id, book, market, side, source
	        ORDER BY observed_at DESC, id DESC
	    ) AS recency_rank
	    FROM observations
	    %s
	)
	WHERE recency_rank = 1`

func (s *Store) RecordObservation(ctx context.Context, record ObservationRecord) (RecordResult, error) {
	key := record.Observation.KeyFor(record.EventID)
	contentKey := contentKeyFor(key, record.Observation)

	inserted, err := s.insertObservation(ctx, contentKey, key, record)
	if err != nil {
		return 0, err
	}
	if inserted {
		return RecordInserted, nil
	}
	if err := s.ConfirmObservation(ctx, contentKey, record.ReceivedAt); err != nil {
		return 0, err
	}
	return RecordConfirmed, nil
}

func (s *Store) insertObservation(ctx context.Context, contentKey string, key canonical.PriceKey, record ObservationRecord) (bool, error) {
	observation := record.Observation
	result, err := s.db.ExecContext(ctx,
		`INSERT INTO observations (
		     content_key, source, slice, book, event_id, market, side, status, decimal_price,
		     raw_price, raw_format, observed_at, has_source_timestamp, received_at, last_confirmed_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT (content_key) DO NOTHING`,
		contentKey, observation.Source, record.Slice, key.Book, key.EventID, key.Market, key.Side,
		observation.Status, float64(observation.Price), observation.RawPrice, observation.RawFormat,
		toMillis(observation.ObservedAt), observation.HasSourceTimestamp,
		toMillis(record.ReceivedAt), toMillis(record.ReceivedAt))
	if err != nil {
		return false, fmt.Errorf("insert observation: %w", err)
	}
	rowsInserted, err := result.RowsAffected()
	return rowsInserted == 1, err
}

func (s *Store) ConfirmObservation(ctx context.Context, contentKey string, confirmedAt time.Time) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE observations SET last_confirmed_at = MAX(last_confirmed_at, ?) WHERE content_key = ?`,
		toMillis(confirmedAt), contentKey)
	if err != nil {
		return fmt.Errorf("confirm observation: %w", err)
	}
	return nil
}

func (s *Store) LatestSourcePrices(ctx context.Context) ([]canonical.SourcePrice, error) {
	observations, err := s.queryLatestObservations(ctx, "")
	if err != nil {
		return nil, err
	}
	prices := make([]canonical.SourcePrice, 0, len(observations))
	for _, observation := range observations {
		prices = append(prices, observation.Latest)
	}
	return prices, nil
}

func (s *Store) LatestForSourceSlice(ctx context.Context, source canonical.SourceID, slice string) ([]SliceObservation, error) {
	return s.queryLatestObservations(ctx, "WHERE source = ? AND slice = ?", source, slice)
}

func (s *Store) queryLatestObservations(ctx context.Context, filter string, args ...any) ([]SliceObservation, error) {
	rows, err := s.db.QueryContext(ctx, fmt.Sprintf(latestObservationPerKeyQuery, filter), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var observations []SliceObservation
	for rows.Next() {
		observation, err := scanSliceObservation(rows)
		if err != nil {
			return nil, err
		}
		observations = append(observations, observation)
	}
	return observations, rows.Err()
}

func scanSliceObservation(rows rowScanner) (SliceObservation, error) {
	var observation SliceObservation
	latest := &observation.Latest
	var decimalPrice float64
	var observedAt, lastConfirmedAt int64
	err := rows.Scan(&observation.ContentKey, &latest.Key.EventID, &latest.Key.Book, &latest.Key.Market,
		&latest.Key.Side, &latest.Source, &latest.Status, &decimalPrice, &observedAt, &lastConfirmedAt,
		&latest.HasSourceTimestamp)
	latest.Price = odds.Decimal(decimalPrice)
	latest.ObservedAt = fromMillis(observedAt)
	latest.LastConfirmedAt = fromMillis(lastConfirmedAt)
	return observation, err
}

func contentKeyFor(key canonical.PriceKey, observation canonical.Observation) string {
	identity := fmt.Sprintf("%s|%s|%d|%s|%s|%s|%s|%d",
		observation.Source, key.Book, key.EventID, key.Market, key.Side, observation.Status,
		observation.Price, toMillis(observation.ObservedAt))
	digest := sha256.Sum256([]byte(identity))
	return hex.EncodeToString(digest[:])
}
