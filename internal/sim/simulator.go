package sim

import (
	"context"
	"math"
	"net/http"
	"strconv"
	"time"

	"github.com/BrendanMoorehead/data-flow/internal/httpserve"
)

type Options struct {
	Seed               uint64
	BackgroundProblems bool
	Now                func() time.Time
}

type Simulator struct {
	world *world
	chaos *chaos
	now   func() time.Time
}

func New(opts Options) *Simulator {
	return &Simulator{
		world: newWorld(opts.Seed, opts.Now),
		chaos: newChaos(opts.Seed, opts.BackgroundProblems, opts.Now),
		now:   opts.Now,
	}
}

func (s *Simulator) Run(ctx context.Context, driftInterval time.Duration) {
	s.world.run(ctx, driftInterval)
}

func (s *Simulator) withDraftKingsProblems(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if s.chaos.isActive(ScenarioDraftKingsOutage) {
			httpserve.WriteError(w, http.StatusServiceUnavailable, "draftkings feed unavailable")
			return
		}
		if s.chaos.isActive(ScenarioDraftKingsSlow) && !waitOrCancelled(r.Context(), draftKingsSlowResponseDelay) {
			return
		}
		if s.chaos.backgroundChance(draftKingsBackgroundErrorRate) {
			httpserve.WriteError(w, http.StatusInternalServerError, "intermittent draftkings error")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Simulator) withAggregatorProblems(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if retryAfter, limited := s.chaos.aggregatorRetryAfter(); limited {
			w.Header().Set("Retry-After", strconv.Itoa(int(math.Ceil(retryAfter.Seconds()))))
			httpserve.WriteError(w, http.StatusTooManyRequests, "aggregator rate limit exceeded")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func waitOrCancelled(ctx context.Context, delay time.Duration) bool {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}
