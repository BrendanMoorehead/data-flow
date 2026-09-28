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

func (s *Store) RecordObservation(ctx context.Context, eventID canonical.EventID, observation canonical.Observation, receivedAt time.Time) (RecordResult, error) {
	key := observation.KeyFor(eventID)
	contentKey := contentKeyFor(key, observation)

	inserted, err := s.insertObservation(ctx, contentKey, key, observation, receivedAt)
	if err != nil {
		return 0, err
	}
	if inserted {
		return RecordInserted, nil
	}
	if err := s.confirmObservation(ctx, contentKey, receivedAt); err != nil {
		return 0, err
	}
	return RecordConfirmed, nil
}

func (s *Store) insertObservation(ctx context.Context, contentKey string, key canonical.PriceKey, observation canonical.Observation, receivedAt time.Time) (bool, error) {
	result, err := s.db.ExecContext(ctx,
		`INSERT INTO observations (
		     content_key, source, book, event_id, market, side, decimal_price,
		     raw_price, raw_format, observed_at, received_at, last_confirmed_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT (content_key) DO NOTHING`,
		contentKey, observation.Source, key.Book, key.EventID, key.Market, key.Side, float64(observation.Price),
		observation.RawPrice, observation.RawFormat, toMillis(observation.ObservedAt),
		toMillis(receivedAt), toMillis(receivedAt))
	if err != nil {
		return false, fmt.Errorf("insert observation: %w", err)
	}
	rowsInserted, err := result.RowsAffected()
	return rowsInserted == 1, err
}

func (s *Store) confirmObservation(ctx context.Context, contentKey string, confirmedAt time.Time) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE observations SET last_confirmed_at = MAX(last_confirmed_at, ?) WHERE content_key = ?`,
		toMillis(confirmedAt), contentKey)
	if err != nil {
		return fmt.Errorf("confirm observation: %w", err)
	}
	return nil
}

func (s *Store) LatestSourcePrices(ctx context.Context) ([]canonical.SourcePrice, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT event_id, book, market, side, source, decimal_price, observed_at, last_confirmed_at
		 FROM (
		     SELECT *, ROW_NUMBER() OVER (
		         PARTITION BY event_id, book, market, side, source
		         ORDER BY observed_at DESC, id DESC
		     ) AS recency_rank
		     FROM observations
		 )
		 WHERE recency_rank = 1`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var prices []canonical.SourcePrice
	for rows.Next() {
		price, err := scanSourcePrice(rows)
		if err != nil {
			return nil, err
		}
		prices = append(prices, price)
	}
	return prices, rows.Err()
}

func scanSourcePrice(rows rowScanner) (canonical.SourcePrice, error) {
	var price canonical.SourcePrice
	var decimalPrice float64
	var observedAt, lastConfirmedAt int64
	err := rows.Scan(&price.Key.EventID, &price.Key.Book, &price.Key.Market, &price.Key.Side,
		&price.Source, &decimalPrice, &observedAt, &lastConfirmedAt)
	price.Price = odds.Decimal(decimalPrice)
	price.ObservedAt = fromMillis(observedAt)
	price.LastConfirmedAt = fromMillis(lastConfirmedAt)
	return price, err
}

func contentKeyFor(key canonical.PriceKey, observation canonical.Observation) string {
	identity := fmt.Sprintf("%s|%s|%d|%s|%s|%s|%d",
		observation.Source, key.Book, key.EventID, key.Market, key.Side,
		observation.Price, toMillis(observation.ObservedAt))
	digest := sha256.Sum256([]byte(identity))
	return hex.EncodeToString(digest[:])
}
