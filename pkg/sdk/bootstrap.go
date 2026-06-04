package sdk

import (
	"fmt"
	"os"
	"path/filepath"
)

// LoadBootstrapFromFile reads a previously saved bootstrap file and
// returns the raw JSON bytes for use with WithBootstrap.
func LoadBootstrapFromFile(path string) ([]byte, error) {
	data, err := os.ReadFile(path) //nolint:gosec // path is caller-supplied by design
	if err != nil {
		return nil, fmt.Errorf("sdk: load bootstrap: %w", err)
	}
	return data, nil
}

// SaveBootstrapToFile atomically writes the raw snapshot bytes to path
// using a temp-file + rename, so a crash mid-write never leaves a
// corrupt file.
func SaveBootstrapToFile(path string, snap []byte) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".bootstrap-*.tmp")
	if err != nil {
		return fmt.Errorf("sdk: save bootstrap: %w", err)
	}

	tmpPath := tmp.Name()
	defer func() {
		_ = os.Remove(tmpPath)
	}()

	if _, err := tmp.Write(snap); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("sdk: save bootstrap: write: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("sdk: save bootstrap: sync: %w", err)
	}

	if err := tmp.Close(); err != nil {
		return fmt.Errorf("sdk: save bootstrap: close: %w", err)
	}

	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("sdk: save bootstrap: rename: %w", err)
	}

	return nil
}
