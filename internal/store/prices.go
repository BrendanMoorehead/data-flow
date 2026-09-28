package store

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/BrendanMoorehead/data-flow/internal/canonical"
	"github.com/BrendanMoorehead/data-flow/internal/odds"
)

const reasonSeparator = ","

func (s *Store) UpsertResolvedPrices(ctx context.Context, prices []canonical.ResolvedPrice, resolvedAt time.Time) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	for _, price := range prices {
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO resolved_prices (
			     event_id, book, market, side, status, decimal_price, source, observed_at, last_confirmed_at,
			     confidence_level, confidence_reasons, resolved_at)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
			 ON CONFLICT (event_id, book, market, side) DO UPDATE SET
			     status = excluded.status,
			     decimal_price = excluded.decimal_price,
			     source = excluded.source,
			     observed_at = excluded.observed_at,
			     last_confirmed_at = excluded.last_confirmed_at,
			     confidence_level = excluded.confidence_level,
			     confidence_reasons = excluded.confidence_reasons,
			     resolved_at = excluded.resolved_at`,
			price.Key.EventID, price.Key.Book, price.Key.Market, price.Key.Side, price.Status,
			float64(price.Price), price.Source, toMillis(price.ObservedAt), toMillis(price.LastConfirmedAt),
			price.Confidence.Level, joinReasons(price.Confidence.Reasons), toMillis(resolvedAt)); err != nil {
			return fmt.Errorf("upsert resolved price: %w", err)
		}
	}
	return tx.Commit()
}

func (s *Store) ResolvedPricesForEvent(ctx context.Context, eventID canonical.EventID) ([]canonical.ResolvedPrice, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT event_id, book, market, side, status, decimal_price, source, observed_at, last_confirmed_at,
		        confidence_level, confidence_reasons
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
	var reasons string
	err := rows.Scan(&price.Key.EventID, &price.Key.Book, &price.Key.Market, &price.Key.Side, &price.Status,
		&decimalPrice, &price.Source, &observedAt, &lastConfirmedAt, &price.Confidence.Level, &reasons)
	price.Price = odds.Decimal(decimalPrice)
	price.ObservedAt = fromMillis(observedAt)
	price.LastConfirmedAt = fromMillis(lastConfirmedAt)
	price.Confidence.Reasons = splitReasons(reasons)
	return price, err
}

func joinReasons(reasons []canonical.ConfidenceReason) string {
	texts := make([]string, 0, len(reasons))
	for _, reason := range reasons {
		texts = append(texts, string(reason))
	}
	return strings.Join(texts, reasonSeparator)
}

func splitReasons(joined string) []canonical.ConfidenceReason {
	if joined == "" {
		return []canonical.ConfidenceReason{}
	}
	var reasons []canonical.ConfidenceReason
	for _, text := range strings.Split(joined, reasonSeparator) {
		reasons = append(reasons, canonical.ConfidenceReason(text))
	}
	return reasons
}
