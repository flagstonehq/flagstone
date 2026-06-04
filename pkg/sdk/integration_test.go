package sdk

import (
	"context"
	"os"
	"testing"
	"time"
)

func TestIntegration_RealBackend(t *testing.T) {
	endpoint := os.Getenv("BACKEND_URL")
	apiKey := os.Getenv("FLAGSTONE_API_KEY")
	if endpoint == "" || apiKey == "" {
		t.Skip("BACKEND_URL or FLAGSTONE_API_KEY not set; skipping integration test")
	}

	c, err := New(WithEndpoint(endpoint), WithAPIKey(apiKey))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	all, err := c.All(ctx, map[string]any{"user_id": "test"})
	if err != nil {
		t.Fatalf("All: %v", err)
	}
	if len(all) == 0 {
		t.Log("no flags in environment (empty snapshot) — that's fine")
	}
}
