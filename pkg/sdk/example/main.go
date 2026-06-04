package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/flagstonehq/flagstone/pkg/sdk"
)

func main() {
	endpoint := envOr("FLAGSTONE_ENDPOINT", "http://localhost:8080")
	apiKey := os.Getenv("FLAGSTONE_API_KEY")
	if apiKey == "" {
		log.Fatal("FLAGSTONE_API_KEY env var is required")
	}
	if err := run(endpoint, apiKey); err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
}

func run(endpoint, apiKey string) error {
	client, err := sdk.New(
		sdk.WithEndpoint(endpoint),
		sdk.WithAPIKey(apiKey),
	)
	if err != nil {
		return fmt.Errorf("sdk.New: %w", err)
	}
	defer func() { _ = client.Close() }()
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	if err := client.Start(ctx); err != nil {
		return fmt.Errorf("client.Start: %w", err)
	}
	http.HandleFunc("/checkout", func(w http.ResponseWriter, r *http.Request) {
		userID := r.URL.Query().Get("user_id")
		country := r.URL.Query().Get("country")
		if userID == "" {
			http.Error(w, `{"error":"user_id is required"}`, http.StatusBadRequest)
			return
		}
		enabled, err := client.Bool(r.Context(), "new-checkout", map[string]any{
			"user_id": userID,
			"country": country,
		})
		if err != nil {
			http.Error(w, fmt.Sprintf(`{"error":%q}`, err.Error()), http.StatusInternalServerError)
			return
		}
		checkout := "v1"
		if enabled {
			checkout = "v2"
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"checkout": checkout})
	})
	addr := ":9000"
	srv := &http.Server{Addr: addr, ReadHeaderTimeout: 5 * time.Second}
	go func() {
		log.Printf("example server listening on %s (SDK endpoint=%s)", addr, endpoint)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("http: %v", err)
		}
	}()
	<-ctx.Done()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = srv.Shutdown(shutdownCtx)
	return nil
}
func envOr(k, fallback string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return fallback
}
