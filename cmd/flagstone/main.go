package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/exaring/otelpgx"
	"github.com/flagstonehq/flagstone/internal/api"
	"github.com/flagstonehq/flagstone/internal/config"
	"github.com/flagstonehq/flagstone/internal/storage"
	"github.com/flagstonehq/flagstone/internal/telemetry"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"go.opentelemetry.io/otel"
	"go.uber.org/zap"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

type readinessResponse struct {
	Status string                    `json:"status"`
	Checks map[string]readinessCheck `json:"checks"`
}

type readinessCheck struct {
	Status    string `json:"status"`
	LatencyMS int64  `json:"latency_ms,omitempty"`
	Error     string `json:"error,omitempty"`
}

type healthResponse struct {
	Status        string `json:"status"`
	Version       string `json:"version"`
	UptimeSeconds int64  `json:"uptime_seconds"`
}

func main() {
	logger, err := zap.NewProduction()
	if err != nil {
		fmt.Fprintf(os.Stderr, "logger init: %v\n", err)
		os.Exit(1)
	}
	defer logger.Sync() //nolint:errcheck // sync on shutdown is best-effort

	cfg, err := config.Load()
	if err != nil {
		logger.Fatal("failed to load config", zap.Error(err))
	}

	startedAt := time.Now()

	logger.Info("starting flagstone",
		zap.String("version", version),
		zap.String("commit", commit),
		zap.String("date", date),
		zap.String("addr", cfg.Addr),
		zap.String("environment", cfg.Environment),
	)

	rootCtx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	shutdownOtel, err := telemetry.SetupOTel(rootCtx, "flagstone")
	if err != nil {
		logger.Fatal("failed to setup telemetry", zap.Error(err))
	}
	defer func() {
		if err := shutdownOtel(context.Background()); err != nil {
			logger.Error("telemetry shutdown error", zap.Error(err))
		}
	}()

	dbPool, err := connectPostgresWithRetry(rootCtx, cfg.DatabaseURL, logger)
	if err != nil {
		logger.Fatal("failed to connect to postgres", zap.Error(err))
	}
	defer dbPool.Close()

	redisClient := redis.NewClient(&redis.Options{
		Addr: redisAddrFromURL(cfg.RedisURL),
	})
	defer redisClient.Close() //nolint:errcheck // shutdown is best-effort

	stores := storage.NewStores(dbPool)

	metrics, err := telemetry.NewMetrics(otel.GetMeterProvider())
	if err != nil {
		logger.Fatal("failed to create metrics", zap.Error(err))
	}
	if metrics != nil {
		if err := metrics.RegisterDBPoolStats(func() telemetry.DBPoolStats {
			stat := dbPool.Stat()
			return telemetry.DBPoolStats{
				IdleConns:          int64(stat.IdleConns()),
				AcquiredTotal:      stat.AcquireCount(),
				AcquireWaitSeconds: stat.AcquireDuration().Seconds(),
			}
		}); err != nil {
			logger.Warn("failed to register db pool stats", zap.Error(err))
		}
	}

	apiServer := api.NewServer(stores, dbPool, cfg, logger, redisClient, metrics)
	apiServer.StartCleanup(rootCtx)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, healthResponse{
			Status:        "ok",
			Version:       version,
			UptimeSeconds: int64(time.Since(startedAt).Seconds()),
		})
	})

	mux.HandleFunc("GET /readyz", func(w http.ResponseWriter, r *http.Request) {
		resp, status := checkReadiness(r.Context(), dbPool, redisClient)
		writeJSON(w, status, resp)
	})

	mux.Handle("/api/v1/", apiServer.Routes())

	handler := telemetry.WrapWithTracing(mux, "/healthz", "/readyz")

	srv := &http.Server{
		Addr:         cfg.Addr,
		Handler:      handler,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	go func() {
		logger.Info("listening", zap.String("addr", cfg.Addr))
		if err := srv.ListenAndServe(); err != nil {
			logger.Error("server error", zap.Error(err))
			os.Exit(1) //nolint:gocritic // exit on unrecoverable server error
		}
	}()

	<-rootCtx.Done()
	logger.Info("shutting down gracefully")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Fatal("forced shutdown", zap.Error(err))
	}

	logger.Info("server stopped")
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func redisAddrFromURL(raw string) string {
	const prefix = "redis://"
	if len(raw) >= len(prefix) && raw[:len(prefix)] == prefix {
		return raw[len(prefix):]
	}
	return raw
}

func checkReadiness(ctx context.Context, dbPool *pgxpool.Pool, redisClient *redis.Client) (resp readinessResponse, code int) {
	resp = readinessResponse{
		Status: "ready",
		Checks: map[string]readinessCheck{},
	}
	dbStart := time.Now()
	dbCtx, dbCancel := context.WithTimeout(ctx, 2*time.Second)
	defer dbCancel()
	if err := dbPool.Ping(dbCtx); err != nil {
		resp.Status = "not_ready"
		resp.Checks["postgres"] = readinessCheck{
			Status: "down",
			Error:  err.Error(),
		}
	} else {
		resp.Checks["postgres"] = readinessCheck{
			Status:    "up",
			LatencyMS: time.Since(dbStart).Milliseconds(),
		}
	}
	redisStart := time.Now()
	redisCtx, redisCancel := context.WithTimeout(ctx, 1*time.Second)
	defer redisCancel()
	if err := redisClient.Ping(redisCtx).Err(); err != nil {
		resp.Checks["redis"] = readinessCheck{
			Status: "down",
			Error:  err.Error(),
		}
	} else {
		resp.Checks["redis"] = readinessCheck{
			Status:    "up",
			LatencyMS: time.Since(redisStart).Milliseconds(),
		}
	}
	if resp.Checks["postgres"].Status != "up" {
		code = http.StatusServiceUnavailable
		return
	}
	code = http.StatusOK
	return
}

func connectPostgresWithRetry(ctx context.Context, databaseURL string, logger *zap.Logger) (*pgxpool.Pool, error) {
	var lastErr error
	backoff := time.Second

	for attempt := 1; attempt <= 5; attempt++ {
		poolCfg, err := pgxpool.ParseConfig(databaseURL)
		if err != nil {
			return nil, fmt.Errorf("parse pg config: %w", err)
		}
		poolCfg.MaxConns = 25
		poolCfg.MinConns = 5
		poolCfg.MaxConnLifetime = time.Hour
		poolCfg.MaxConnIdleTime = 5 * time.Minute
		poolCfg.HealthCheckPeriod = 30 * time.Second
		poolCfg.ConnConfig.Tracer = otelpgx.NewTracer()

		pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
		if err == nil {
			pingCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
			pingErr := pool.Ping(pingCtx)
			cancel()
			if pingErr == nil {
				return pool, nil
			}
			lastErr = pingErr
			pool.Close()
		} else {
			lastErr = err
		}

		logger.Warn("postgres not ready, retrying",
			zap.Int("attempt", attempt),
			zap.Duration("backoff", backoff),
			zap.Error(lastErr),
		)

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(backoff):
			backoff *= 2
		}
	}

	return nil, fmt.Errorf("connect postgres after retries: %w", lastErr)
}
