package syncer

import (
	"context"
	"math/rand/v2"
	"sync"
	"time"

	"github.com/BrendanMoorehead/data-flow/internal/backoff"
	"github.com/BrendanMoorehead/data-flow/internal/provider"
	"github.com/BrendanMoorehead/data-flow/internal/source"
)

const (
	dispatchInterval = 200 * time.Millisecond
	maxBackoff       = time.Minute
)

type providerRunner struct {
	syncer   *Syncer
	provider provider.Provider
	config   source.Config
	schedule *sliceSchedule
	jobs     chan provider.SliceKey
}

func newProviderRunner(syncer *Syncer, p provider.Provider, config source.Config) *providerRunner {
	return &providerRunner{
		syncer:   syncer,
		provider: p,
		config:   config,
		schedule: newSliceSchedule(),
		jobs:     make(chan provider.SliceKey),
	}
}

func (r *providerRunner) run(ctx context.Context) {
	var running sync.WaitGroup
	startTask(&running, func() { r.refreshCatalogPeriodically(ctx) })
	startTask(&running, func() { r.dispatchDueSlices(ctx) })
	for range max(r.config.Workers, 1) {
		startTask(&running, func() { r.pollAssignedSlices(ctx) })
	}
	running.Wait()
}

func (r *providerRunner) refreshCatalogPeriodically(ctx context.Context) {
	consecutiveFailures := 0
	for {
		var delay time.Duration
		consecutiveFailures, delay = r.refreshCatalog(ctx, consecutiveFailures)
		if !sleepContext(ctx, delay) {
			return
		}
	}
}

func (r *providerRunner) refreshCatalog(ctx context.Context, failuresBefore int) (failuresAfter int, delay time.Duration) {
	attemptedAt := r.syncer.now()
	err := r.loadSlices(ctx, attemptedAt)
	failuresAfter = nextFailureCount(failuresBefore, err == nil)
	delay = r.catalogDelayAfter(failuresAfter)
	r.syncer.recordAttempt(ctx, r.provider.Source(), provider.CatalogSlice, attemptedAt, attemptedAt.Add(delay), err)
	return failuresAfter, delay
}

func (r *providerRunner) loadSlices(ctx context.Context, now time.Time) error {
	slices, err := r.provider.Slices(ctx)
	if err != nil {
		return err
	}
	r.schedule.replaceSlices(slices, now)
	return nil
}

func (r *providerRunner) pollDueSlicesOnce(ctx context.Context) {
	for _, slice := range r.schedule.claimDue(r.syncer.now()) {
		r.pollAndReschedule(ctx, slice)
	}
}

func (r *providerRunner) dispatchDueSlices(ctx context.Context) {
	ticker := time.NewTicker(dispatchInterval)
	defer ticker.Stop()
	for {
		for _, slice := range r.schedule.claimDue(r.syncer.now()) {
			select {
			case <-ctx.Done():
				return
			case r.jobs <- slice:
			}
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (r *providerRunner) pollAssignedSlices(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case slice := <-r.jobs:
			r.pollAndReschedule(ctx, slice)
		}
	}
}

func (r *providerRunner) pollAndReschedule(ctx context.Context, slice provider.SliceKey) {
	attemptedAt := r.syncer.now()
	err := r.syncer.pollSlice(ctx, r.provider, slice)
	nextAttemptAt := r.schedule.finish(slice, err == nil, r.syncer.now(), r.sliceDelayAfter)
	r.syncer.recordAttempt(ctx, r.provider.Source(), string(slice), attemptedAt, nextAttemptAt, err)
}

func (r *providerRunner) sliceDelayAfter(consecutiveFailures int) time.Duration {
	return r.delayAfter(r.config.PollInterval, consecutiveFailures)
}

func (r *providerRunner) catalogDelayAfter(consecutiveFailures int) time.Duration {
	return r.delayAfter(r.config.CatalogRefreshInterval, consecutiveFailures)
}

func (r *providerRunner) delayAfter(healthyInterval time.Duration, consecutiveFailures int) time.Duration {
	if consecutiveFailures == 0 {
		return healthyInterval
	}
	return backoff.Delay(r.config.PollInterval, maxBackoff, consecutiveFailures, rand.Float64())
}

func startTask(group *sync.WaitGroup, task func()) {
	group.Add(1)
	go func() {
		defer group.Done()
		task()
	}()
}

func sleepContext(ctx context.Context, duration time.Duration) bool {
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}
