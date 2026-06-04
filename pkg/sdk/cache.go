package sdk

import (
	"sync/atomic"
	"time"

	"github.com/flagstonehq/flagstone/pkg/engine"
)

// snapshot is the SDK's cached view of all flag configs and segments
// for a single environment. Once published via snapshotCache.store,
// it is treated as immutable: the single-writer refresh loop always
// publishes a freshly allocated *snapshot, so concurrent readers can
// load it lock-free without coordinating with writers.
type snapshot struct {
	flags     map[string]engine.FlagConfig
	segments  map[string]engine.Segment
	fetchedAt time.Time
}

func newSnapshot() *snapshot {
	return &snapshot{
		flags:    make(map[string]engine.FlagConfig),
		segments: make(map[string]engine.Segment),
	}
}

// snapshotCache holds the most recently published snapshot. Reads are
// lock-free atomic loads; the only writer is the refresh loop (see
// Client.refreshLoop), which guarantees that once a *snapshot is
// returned from get() it will not be mutated.
type snapshotCache struct {
	value atomic.Pointer[snapshot]
}

func newSnapshotCache() *snapshotCache {
	c := &snapshotCache{}
	c.value.Store(newSnapshot())
	return c
}

// get returns the current snapshot. Never returns nil.
func (c *snapshotCache) get() *snapshot { return c.value.Load() }

// store publishes a new snapshot. Only the refresh loop calls this.
func (c *snapshotCache) store(s *snapshot) { c.value.Store(s) }
