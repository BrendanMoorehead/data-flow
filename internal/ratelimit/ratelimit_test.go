package ratelimit

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestRetryAfterReadsSecondsAndFallsBackToDefault(t *testing.T) {
	cases := map[string]time.Duration{"3": 3 * time.Second, "": defaultRetryAfter, "soon": defaultRetryAfter}
	for headerValue, want := range cases {
		header := http.Header{"Retry-After": {headerValue}}
		if got := retryAfter(header); got != want {
			t.Errorf("retryAfter(%q) = %v, want %v", headerValue, got, want)
		}
	}
}

func TestRateLimitedResponsePausesTheWholeBudget(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "1")
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer server.Close()
	budget := NewBudget(100, 10)
	client := &http.Client{Transport: &Transport{Base: http.DefaultTransport, Budget: budget}}

	response, err := client.Get(server.URL)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	response.Body.Close()

	waitStarted := time.Now()
	if err := budget.Wait(context.Background()); err != nil {
		t.Fatalf("wait: %v", err)
	}
	if waited := time.Since(waitStarted); waited < 900*time.Millisecond {
		t.Errorf("next request waited %v after a 429 with Retry-After 1, want about 1s", waited)
	}
}

func TestWaitGivesUpWhenContextEnds(t *testing.T) {
	budget := NewBudget(100, 10)
	budget.PauseFor(time.Minute)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()

	if err := budget.Wait(ctx); err == nil {
		t.Error("Wait returned nil while paused past the context deadline")
	}
}
