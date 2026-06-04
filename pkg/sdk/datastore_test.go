package sdk

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInMemoryDataStore_RoundTrip(t *testing.T) {
	ctx := context.Background()
	s := NewInMemoryDataStore()
	defer s.Close()

	raw := []byte(`{"flags":{"a":{"enabled":true}},"segments":{}}`)
	if err := s.Save(ctx, raw); err != nil {
		t.Fatal(err)
	}
	got, err := s.Load(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, raw) {
		t.Fatalf("got %s, want %s", string(got), string(raw))
	}
}

func TestFileDataStore_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "snapshot.json")
	ctx := context.Background()

	s1 := NewFileDataStore(path)
	raw := []byte(`{"flags":{"b":{"enabled":false}},"segments":{}}`)
	if err := s1.Save(ctx, raw); err != nil {
		t.Fatal(err)
	}
	s1.Close()

	s2 := NewFileDataStore(path)
	defer s2.Close()
	got, err := s2.Load(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, raw) {
		t.Fatalf("got %s, want %s", string(got), string(raw))
	}
}

func TestFileDataStore_AtomicWrite(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "atomic.json")
	s := NewFileDataStore(path)
	ctx := context.Background()
	raw := []byte(`{"flags":{},"segments":{}}`)

	if err := s.Save(ctx, raw); err != nil {
		t.Fatal(err)
	}
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".datastore-") {
			t.Fatalf("temp file left behind: %s", e.Name())
		}
	}
	got, _ := s.Load(ctx)
	if !bytes.Equal(got, raw) {
		t.Fatalf("got %s, want %s", string(got), string(raw))
	}
}

func TestInMemoryDataStore_ErrClosed(t *testing.T) {
	ctx := context.Background()
	s := NewInMemoryDataStore()
	s.Close()
	_, err := s.Load(ctx)
	if !errors.Is(err, ErrClosed) {
		t.Fatalf("expected ErrClosed after Close, got %v", err)
	}
	if err := s.Save(ctx, []byte("x")); !errors.Is(err, ErrClosed) {
		t.Fatalf("expected ErrClosed after Close, got %v", err)
	}
}

func TestFileDataStore_ErrClosed(t *testing.T) {
	s := NewFileDataStore(filepath.Join(t.TempDir(), "snap.json"))
	s.Close()
	ctx := context.Background()
	_, err := s.Load(ctx)
	if !errors.Is(err, ErrClosed) {
		t.Fatalf("expected ErrClosed after Close, got %v", err)
	}
	if err := s.Save(ctx, []byte("x")); !errors.Is(err, ErrClosed) {
		t.Fatalf("expected ErrClosed after Close, got %v", err)
	}
}

func TestDataStore_LoadOnStartup(t *testing.T) {
	ctx := context.Background()
	store := NewInMemoryDataStore()
	snap := []byte(sampleSnapshot)
	if err := store.Save(ctx, snap); err != nil {
		t.Fatal(err)
	}

	c, err := New(
		WithEndpoint("http://never-hit.invalid"),
		WithAPIKey("k"),
		WithDataStore(store),
	)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()

	_ = c.Start(ctx)

	val := c.Bool(ctx, "new-checkout", true, nil)
	if val != false {
		t.Fatalf("expected false from store-loaded data, got %v", val)
	}
}
