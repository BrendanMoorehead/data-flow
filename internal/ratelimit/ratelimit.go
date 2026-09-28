package ratelimit

import (
	"context"
	"net/http"
	"strconv"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

const defaultRetryAfter = 5 * time.Second

type Budget struct {
	limiter     *rate.Limiter
	mu          sync.Mutex
	pausedUntil time.Time
}

func NewBudget(requestsPerSecond float64, burst int) *Budget {
	return &Budget{limiter: rate.NewLimiter(rate.Limit(requestsPerSecond), burst)}
}

func (b *Budget) Wait(ctx context.Context) error {
	if err := sleepUntil(ctx, b.resumeAt()); err != nil {
		return err
	}
	return b.limiter.Wait(ctx)
}

func (b *Budget) PauseFor(duration time.Duration) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if resumeAt := time.Now().Add(duration); resumeAt.After(b.pausedUntil) {
		b.pausedUntil = resumeAt
	}
}

func (b *Budget) resumeAt() time.Time {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.pausedUntil
}

type Transport struct {
	Base   http.RoundTripper
	Budget *Budget
}

func (t *Transport) RoundTrip(request *http.Request) (*http.Response, error) {
	if err := t.Budget.Wait(request.Context()); err != nil {
		return nil, err
	}
	response, err := t.Base.RoundTrip(request)
	if err == nil && response.StatusCode == http.StatusTooManyRequests {
		t.Budget.PauseFor(retryAfter(response.Header))
	}
	return response, err
}

func retryAfter(header http.Header) time.Duration {
	seconds, err := strconv.Atoi(header.Get("Retry-After"))
	if err != nil || seconds <= 0 {
		return defaultRetryAfter
	}
	return time.Duration(seconds) * time.Second
}

func sleepUntil(ctx context.Context, resumeAt time.Time) error {
	wait := time.Until(resumeAt)
	if wait <= 0 {
		return nil
	}
	timer := time.NewTimer(wait)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
