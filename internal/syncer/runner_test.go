package syncer

import (
	"context"
	"io"
	"log/slog"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/BrendanMoorehead/data-flow/internal/canonical"
	"github.com/BrendanMoorehead/data-flow/internal/identity"
	"github.com/BrendanMoorehead/data-flow/internal/provider"
	"github.com/BrendanMoorehead/data-flow/internal/source"
	"github.com/BrendanMoorehead/data-flow/internal/store"
)

const hangingSlice provider.SliceKey = "hanging"

type hangingProvider struct {
	mu           sync.Mutex
	pollsBySlice map[provider.SliceKey]int
}

func (p *hangingProvider) Source() canonical.SourceID {
	return canonical.SourceDraftKingsDirect
}

func (p *hangingProvider) Slices(context.Context) ([]provider.SliceKey, error) {
	return []provider.SliceKey{hangingSlice, "quick-a", "quick-b"}, nil
}

func (p *hangingProvider) Poll(ctx context.Context, slice provider.SliceKey) (provider.Snapshot, error) {
	p.mu.Lock()
	p.pollsBySlice[slice]++
	p.mu.Unlock()
	if slice == hangingSlice {
		<-ctx.Done()
		return provider.Snapshot{}, ctx.Err()
	}
	return provider.Snapshot{}, nil
}

func (p *hangingProvider) polls(slice provider.SliceKey) int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.pollsBySlice[slice]
}

func newTestSyncer(t *testing.T) *Syncer {
	t.Helper()
	testStore, err := store.Open(context.Background(), filepath.Join(t.TempDir(), "runner.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { testStore.Close() })
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	return New(Dependencies{
		Store:  testStore,
		Events: identity.NewResolver(testStore, logger),
		Logger: logger,
		Now:    time.Now,
	})
}

func TestHangingSliceDoesNotStopOtherSlicesFromPolling(t *testing.T) {
	fake := &hangingProvider{pollsBySlice: make(map[provider.SliceKey]int)}
	config := source.Config{
		ID:                     canonical.SourceDraftKingsDirect,
		PollInterval:           50 * time.Millisecond,
		CatalogRefreshInterval: time.Minute,
		Workers:                2,
	}
	runner := newProviderRunner(newTestSyncer(t), fake, config)
	ctx, cancel := context.WithTimeout(context.Background(), 1500*time.Millisecond)
	defer cancel()

	runner.run(ctx)

	for _, slice := range []provider.SliceKey{"quick-a", "quick-b"} {
		if polls := fake.polls(slice); polls < 3 {
			t.Errorf("%s polled %d times while another slice hung, want at least 3", slice, polls)
		}
	}
}
