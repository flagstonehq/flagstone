package sdk

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBootstrap_Preload(t *testing.T) {
	c, err := New(
		WithEndpoint("http://bootstrap-test.invalid"),
		WithAPIKey("k"),
		WithBootstrap([]byte(sampleSnapshot)),
	)
	if err != nil {
		t.Fatal(err)
	}
	val := c.Bool(context.Background(), "new-checkout", false, nil)
	if val != false {
		t.Fatalf("expected false from bootstrap, got %v", val)
	}
}
func TestBootstrap_InvalidJSON(t *testing.T) {
	_, err := New(
		WithEndpoint("http://bootstrap-test.invalid"),
		WithAPIKey("k"),
		WithBootstrap([]byte("not json")),
	)
	if err == nil {
		t.Fatal("expected error for invalid bootstrap JSON")
	}
	if !strings.Contains(err.Error(), "bootstrap") {
		t.Fatalf("expected error mentioning bootstrap, got %v", err)
	}
}
func TestBootstrap_MissingFile(t *testing.T) {
	srv, _ := newTestServer(t, nil)
	path := t.TempDir() + "/nonexistent-file"
	c, err := New(
		WithEndpoint(srv.URL),
		WithAPIKey("k"),
		WithBootstrapFile(path),
	)
	if err != nil {
		t.Fatal(err)
	}
	// The client should be able to fetch from the network.
	if err := c.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	val := c.Bool(context.Background(), "new-checkout", false, nil)
	if val != false {
		t.Fatalf("expected false from network response, got %v", val)
	}
}
func TestBootstrap_Offline(t *testing.T) {
	c, err := New(
		WithOffline(true),
		WithBootstrap([]byte(sampleSnapshot)),
	)
	if err != nil {
		t.Fatal(err)
	}
	val := c.Bool(context.Background(), "new-checkout", true, nil)
	if val != false {
		t.Fatalf("expected false from bootstrap in offline mode, got %v", val)
	}
}
func TestOffline_AllDefaultValues(t *testing.T) {
	c, err := New(WithOffline(true))
	if err != nil {
		t.Fatal(err)
	}
	// Bool returns the supplied default when offline and no bootstrap.
	val := c.Bool(context.Background(), "any-flag", true, nil)
	if val != true {
		t.Fatalf("expected default true, got %v", val)
	}
	str := c.String(context.Background(), "any-flag", "fallback", nil)
	if str != "fallback" {
		t.Fatalf("expected 'fallback', got %q", str)
	}
	num := c.Number(context.Background(), "any-flag", 42, nil)
	if num != 42 {
		t.Fatalf("expected 42, got %v", num)
	}
}
func TestOffline_InitializedTrue(t *testing.T) {
	c, err := New(WithOffline(true))
	if err != nil {
		t.Fatal(err)
	}
	if !c.Initialized() {
		t.Fatal("expected Initialized() == true in offline mode")
	}
}
func TestSaveBootstrap_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "snapshot.json")
	data := []byte(`{"flags":{},"segments":{},"fetched_at":"2024-01-01T00:00:00Z"}`)
	if err := SaveBootstrapToFile(path, data); err != nil {
		t.Fatal(err)
	}
	got, err := LoadBootstrapFromFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, data) {
		t.Fatalf("got %s, want %s", string(got), string(data))
	}
}
func TestSaveBootstrap_Atomic(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nested", "does-not-exit", "snapshot.json")
	err := SaveBootstrapToFile(path, []byte(`{}`))
	if err == nil {
		t.Fatal("expected error for non-existent directory")
	}
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".bootstrap-") {
			t.Fatalf("temp file left behind: %s", e.Name())
		}
	}
}
