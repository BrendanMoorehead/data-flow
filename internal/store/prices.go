package store

import (
	"context"
	"fmt"
	"time"

	"github.com/BrendanMoorehead/data-flow/internal/canonical"
	"github.com/BrendanMoorehead/data-flow/internal/odds"
)

func (s *Store) UpsertResolvedPrices(ctx context.Context, prices []canonical.ResolvedPrice, resolvedAt time.Time) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	for _, price := range prices {
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO resolved_prices (
			     event_id, book, market, side, decimal_price, source, observed_at, last_confirmed_at, resolved_at)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
			 ON CONFLICT (event_id, book, market, side) DO UPDATE SET
			     decimal_price = excluded.decimal_price,
			     source = excluded.source,
			     observed_at = excluded.observed_at,
			     last_confirmed_at = excluded.last_confirmed_at,
			     resolved_at = excluded.resolved_at`,
			price.Key.EventID, price.Key.Book, price.Key.Market, price.Key.Side, float64(price.Price),
			price.Source, toMillis(price.ObservedAt), toMillis(price.LastConfirmedAt), toMillis(resolvedAt)); err != nil {
			return fmt.Errorf("upsert resolved price: %w", err)
		}
	}
	return tx.Commit()
}

func (s *Store) ResolvedPricesForEvent(ctx context.Context, eventID canonical.EventID) ([]canonical.ResolvedPrice, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT event_id, book, market, side, decimal_price, source, observed_at, last_confirmed_at
		 FROM resolved_prices
		 WHERE event_id = ?
		 ORDER BY book, market, side`, eventID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var prices []canonical.ResolvedPrice
	for rows.Next() {
		price, err := scanResolvedPrice(rows)
		if err != nil {
			return nil, err
		}
		prices = append(prices, price)
	}
	return prices, rows.Err()
}

func scanResolvedPrice(rows rowScanner) (canonical.ResolvedPrice, error) {
	var price canonical.ResolvedPrice
	var decimalPrice float64
	var observedAt, lastConfirmedAt int64
	err := rows.Scan(&price.Key.EventID, &price.Key.Book, &price.Key.Market, &price.Key.Side,
		&decimalPrice, &price.Source, &observedAt, &lastConfirmedAt)
	price.Price = odds.Decimal(decimalPrice)
	price.ObservedAt = fromMillis(observedAt)
	price.LastConfirmedAt = fromMillis(lastConfirmedAt)
	return price, err
}
