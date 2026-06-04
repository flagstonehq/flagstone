package sdk

import (
	"context"
	"sync"
	"sync/atomic"
)

// InMemoryDataStore is a DataStore that keeps the snapshot in a []byte
// guarded by a sync.RWMutex. It is the reference implementation and the
// default when no WithDataStore option is given.
type InMemoryDataStore struct {
	mu     sync.RWMutex
	data   []byte
	closed atomic.Bool
}

// NewInMemoryDataStore returns an empty InMemoryDataStore.
func NewInMemoryDataStore() *InMemoryDataStore {
	return &InMemoryDataStore{}
}

// Load returns a copy of the stored bytes, or (nil, nil) if empty.
func (s *InMemoryDataStore) Load(_ context.Context) ([]byte, error) {
	if s.closed.Load() {
		return nil, ErrClosed
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.data == nil {
		return nil, nil
	}
	out := make([]byte, len(s.data))
	copy(out, s.data)
	return out, nil
}

// Save stores a copy of raw.
func (s *InMemoryDataStore) Save(_ context.Context, raw []byte) error {
	if s.closed.Load() {
		return ErrClosed
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data = make([]byte, len(raw))
	copy(s.data, raw)
	return nil
}

// Close marks the store as closed. Subsequent Load or Save calls return
// ErrClosed. Idempotent.
func (s *InMemoryDataStore) Close() error {
	s.closed.Store(true)
	return nil
}
