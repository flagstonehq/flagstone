package sdk

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

// State describes the Client's connectivity and data-readiness.
type State int

// Connectivity states for the SDK client.
const (
	StateInitializing State = iota // New() called, no fetch yet
	StateConnected                 // first snapshot fetch (or bootstrap) succeeded
	StateStale                     // last fetch failed, but cache has data
	StateOffline                   // WithOffline(true) was used
	StateError                     // first fetch failed, cache is empty
)

var stateNames = map[State]string{
	StateInitializing: "INITIALIZING",
	StateConnected:    "CONNECTED",
	StateStale:        "STALE",
	StateOffline:      "OFFLINE",
	StateError:        "ERROR",
}

func (s State) String() string {
	if n, ok := stateNames[s]; ok {
		return n
	}
	return fmt.Sprintf("State(%d)", s)
}

// StatusProvider exposes the Client's runtime state for observability. Use
// Client.Status() to obtain the implementation.
type StatusProvider interface {
	// State is the current connectivity and data-readiness state. Atomic.
	State() State
	// Initialized is true when the Client has left the initializing phase:
	// the first fetch succeeded, bootstrap was loaded, or offline mode is on.
	Initialized() bool
	// LastError is the most recent fetch or SSE error, or nil.
	LastError() error
	// LastUpdated is when the snapshot was last successfully fetched. Zero
	// time if never.
	LastUpdated() time.Time
	// WaitForReady blocks until Initialized() is true or ctx is cancelled.
	// Returns ctx.Err() on timeout or cancellation.
	WaitForReady(ctx context.Context) error
}

var _ StatusProvider = (*clientStatus)(nil)

type clientStatus struct {
	state     atomic.Uint32
	lastErr   atomic.Value // error or nil
	lastUpd   atomic.Value // time.Time
	readyCh   chan struct{}
	closeOnce sync.Once
	mu        sync.Mutex
	callbacks []func(State)
}

func newClientStatus() *clientStatus {
	return &clientStatus{
		readyCh: make(chan struct{}),
	}
}

func (s *clientStatus) State() State {
	return State(s.state.Load())
}

func (s *clientStatus) Initialized() bool {
	return State(s.state.Load()) != StateInitializing
}
func (s *clientStatus) LastError() error {
	v, _ := s.lastErr.Load().(errStore)
	return v.err
}
func (s *clientStatus) LastUpdated() time.Time {
	v := s.lastUpd.Load()
	if v == nil {
		return time.Time{}
	}
	t, _ := v.(time.Time)
	return t
}
func (s *clientStatus) WaitForReady(ctx context.Context) error {
	select {
	case <-s.readyCh:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// transition atomically moves to newState. If the state changes and the
// previous state was Initializing, the ready channel is closed (exactly
// once). Registered callbacks are fired sequentially (must not block).
func (s *clientStatus) transition(newState State) {
	old := State(s.state.Load())
	if old == newState {
		return
	}
	s.state.Store(uint32(newState))
	if old == StateInitializing {
		s.closeOnce.Do(func() { close(s.readyCh) })
	}
	s.mu.Lock()
	fire := make([]func(State), len(s.callbacks))
	copy(fire, s.callbacks)
	s.mu.Unlock()
	for _, cb := range fire {
		cb(newState)
	}
}

type errStore struct {
	err error
}

func (s *clientStatus) setLastError(err error) {
	s.lastErr.Store(errStore{err: err})
}

func (s *clientStatus) setLastUpdated(t time.Time) {
	s.lastUpd.Store(t)
}

func (s *clientStatus) addCallback(cb func(State)) {
	s.mu.Lock()
	s.callbacks = append(s.callbacks, cb)
	s.mu.Unlock()
}
