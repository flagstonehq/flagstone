package sdk

import (
	"context"
	"errors"
)

// ErrClosed is returned by Load, Save, and Close once a DataStore has
// been closed. Callers can check it with errors.Is.
var ErrClosed = errors.New("sdk: datastore closed")

// DataStore persists the raw snapshot JSON — the exact body returned by
// GET /api/v1/sdk/snapshot. Trafficking in []byte means an external
// implementation never needs to import or construct an SDK type; it just
// stores and returns bytes.
//
// FileDataStore (same package) is the worked example. Third-party
// implementations (Redis, S3, Postgres, etc.) are ~50 lines each and
// never import an unexported SDK type.
type DataStore interface {
	// Load returns the most recent raw snapshot, or (nil, nil) if no
	// data has been stored yet. Implementations should return quickly
	// — the SDK calls Load once on startup, before the first network
	// fetch.
	Load(ctx context.Context) ([]byte, error)

	// Save persists raw atomically. After Save returns nil, a
	// subsequent Load MUST return these bytes (or a later Save).
	// Implementations must ensure no torn writes.
	Save(ctx context.Context, raw []byte) error

	// Close releases any resources held by the store. After Close,
	// Load and Save MUST return ErrClosed. Idempotent — calling
	// Close twice is not an error.
	Close() error
}
