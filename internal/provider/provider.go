package provider

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/BrendanMoorehead/data-flow/internal/canonical"
)

type SliceKey string

const CatalogSlice = "catalog"

type Rejection struct {
	Reason string
	Raw    string
}

// Snapshot is one complete fetch of a slice. MissingMeansOffBoard is set by providers that
// signal an off-the-board market by leaving it out rather than by sending a status.
type Snapshot struct {
	Observations         []canonical.Observation
	Rejections           []Rejection
	MissingMeansOffBoard bool
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

func (b *SnapshotBuilder) Reject(raw any, format string, args ...any) {
	b.snapshot.Rejections = append(b.snapshot.Rejections, NewRejection(raw, format, args...))
}

func (b *SnapshotBuilder) MarkMissingAsOffBoard() {
	b.snapshot.MissingMeansOffBoard = true
}

func (b *SnapshotBuilder) Snapshot() Snapshot {
	return b.snapshot
}

func NewRejection(raw any, format string, args ...any) Rejection {
	return Rejection{Reason: fmt.Sprintf(format, args...), Raw: rawJSON(raw)}
}

func rawJSON(raw any) string {
	encoded, err := json.Marshal(raw)
	if err != nil {
		return fmt.Sprintf("%+v", raw)
	}
	return string(encoded)
}
