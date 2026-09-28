package provider

import (
	"context"
	"fmt"

	"github.com/BrendanMoorehead/data-flow/internal/canonical"
)

type SliceKey string

const CatalogSlice = "catalog"

type Rejection struct {
	Reason string
}

type Snapshot struct {
	Observations []canonical.Observation
	Rejections   []Rejection
}

type Provider interface {
	Source() canonical.SourceID
	Slices(ctx context.Context) ([]SliceKey, error)
	Poll(ctx context.Context, slice SliceKey) (Snapshot, error)
}

type SnapshotBuilder struct {
	snapshot Snapshot
}

func (b *SnapshotBuilder) Add(observation canonical.Observation) {
	b.snapshot.Observations = append(b.snapshot.Observations, observation)
}

func (b *SnapshotBuilder) Reject(format string, args ...any) {
	b.snapshot.Rejections = append(b.snapshot.Rejections, Rejection{Reason: fmt.Sprintf(format, args...)})
}

func (b *SnapshotBuilder) Snapshot() Snapshot {
	return b.snapshot
}
