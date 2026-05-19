package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go.uber.org/zap"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	// Production logger: JSON output, no caller info (cheaper at high QPS),
	// ISO8601 timestamps. Use NewDevelopment() for human-readable local logs.
	logger, err := zap.NewProduction()
	if err != nil {
		fmt.Fprintf(os.Stderr, "logger init: %v\n", err)
		os.Exit(1)
	}
	defer logger.Sync() //nolint:errcheck // sync on shutdown is best-effort

	logger.Info("starting flagstone",
		zap.String("version", version),
		zap.String("commit", commit),
		zap.String("date", date),
	)

	// TODO: Load config from environment / flags
	// TODO: Initialize database connection pool (see DESIGN.md → Connection pool sizing)
	// TODO: Initialize Redis client
	// TODO: Initialize rule engine
	// TODO: Initialize OpenTelemetry provider
	// TODO: Set up HTTP routes (API + SSE)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"status":"ok","version":"%s"}`, version)
	})

	addr := envOr("FLAGSTONE_ADDR", ":8080")
	srv := &http.Server{
		Addr:         addr,
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second, // SSE connections need longer writes
		IdleTimeout:  120 * time.Second,
	}

	// Graceful shutdown: listen for SIGINT/SIGTERM, drain connections, exit.
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go func() {
		logger.Info("listening", zap.String("addr", addr))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("server error", zap.Error(err))
			os.Exit(1)
		}
	}()

	<-ctx.Done()
	logger.Info("shutting down gracefully...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("forced shutdown", zap.Error(err))
		os.Exit(1)
	}

	logger.Info("server stopped")
}

// envOr returns the value of the environment variable key, or fallback if unset.
func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
