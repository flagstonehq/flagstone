package sdk

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync/atomic"
)

// FileDataStore is a DataStore that persists the raw snapshot as a
// single file on disk. It uses the same atomic-write pattern as
// SaveBootstrapToFile: write to a temp file, fsync, then rename.
// This guarantees that a crash mid-write leaves the previous
// snapshot intact.
type FileDataStore struct {
	path   string
	closed atomic.Bool
}

// NewFileDataStore returns a FileDataStore that reads from and writes
// to the given file path.
func NewFileDataStore(path string) *FileDataStore {
	return &FileDataStore{path: path}
}

// Load reads the file from disk. Returns (nil, nil) if the file does
// not exist, or ErrClosed if Close has been called.
func (s *FileDataStore) Load(_ context.Context) ([]byte, error) {
	if s.closed.Load() {
		return nil, ErrClosed
	}
	data, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("sdk: file datastore read: %w", err)
	}
	return data, nil
}

// Save writes raw to a temp file, fsyncs, then renames to the final
// path for crash-safe atomicity. Returns ErrClosed if Close has been
// called.
func (s *FileDataStore) Save(_ context.Context, raw []byte) error {
	if s.closed.Load() {
		return ErrClosed
	}
	dir := filepath.Dir(s.path)
	tmp, err := os.CreateTemp(dir, ".datastore-*.tmp")
	if err != nil {
		return fmt.Errorf("sdk: file datastore save: %w", err)
	}
	tmpPath := tmp.Name()
	defer func() { _ = os.Remove(tmpPath) }()

	if _, err := tmp.Write(raw); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("sdk: file datastore write: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("sdk: file datastore sync: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("sdk: file datastore close: %w", err)
	}
	if err := os.Rename(tmpPath, s.path); err != nil {
		return fmt.Errorf("sdk: file datastore rename: %w", err)
	}
	return nil
}

// Close marks the store as closed. Subsequent Load or Save calls return
// ErrClosed. Idempotent.
func (s *FileDataStore) Close() error {
	s.closed.Store(true)
	return nil
}
