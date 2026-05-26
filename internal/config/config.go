// Package config loads and validates application configuration from
// environment variables. All config is loaded once at startup — no hot
// reloading. This keeps the config surface small and predictable.
package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

// Config holds all application configuration.
type Config struct {
	// Server
	Addr        string // e.g. ":8080"
	Environment string // "development", "staging", "production"

	// Database
	DatabaseURL string // Postgres connection string

	// Redis
	RedisURL string // Redis connection string

	// Auth
	JWTSecret         string
	AccessTokenTTL    time.Duration
	RefreshTokenTTL   time.Duration
	BcryptCost        int
	APIKeyRandomBytes int

	// Rate Limiting
	LoginRateLimit    int // per minute per IP
	EvaluateRateLimit int // per minute per API key

	// Cache
	FlagCacheTTL time.Duration
}

// Load reads configuration from environment variables with sensible defaults.
// It returns an error if any required variable is missing.
func Load() (*Config, error) {
	cfg := &Config{
		Addr:              envOr("FLAGSTONE_ADDR", ":8080"),
		Environment:       envOr("FLAGSTONE_ENV", "development"),
		DatabaseURL:       os.Getenv("DATABASE_URL"),
		RedisURL:          envOr("REDIS_URL", "redis://localhost:6379"),
		JWTSecret:         os.Getenv("JWT_SECRET"),
		AccessTokenTTL:    envDuration("ACCESS_TOKEN_TTL", 15*time.Minute),
		RefreshTokenTTL:   envDuration("REFRESH_TOKEN_TTL", 7*24*time.Hour),
		BcryptCost:        envInt("BCRYPT_COST", 12),
		APIKeyRandomBytes: envInt("API_KEY_RANDOM_BYTES", 32),
		LoginRateLimit:    envInt("LOGIN_RATE_LIMIT", 5),
		EvaluateRateLimit: envInt("EVALUATE_RATE_LIMIT", 1000),
		FlagCacheTTL:      envDuration("FLAG_CACHE_TTL", 30*time.Second),
	}

	if err := cfg.validate(); err != nil {
		return nil, fmt.Errorf("config: %w", err)
	}

	return cfg, nil
}

// IsProd returns true if running in production mode.
func (c *Config) IsProd() bool {
	return c.Environment == "production"
}

func (c *Config) validate() error {
	if c.DatabaseURL == "" {
		return fmt.Errorf("DATABASE_URL is required")
	}
	if c.JWTSecret == "" {
		return fmt.Errorf("JWT_SECRET is required (generate with: openssl rand -hex 32)")
	}
	if len(c.JWTSecret) < 32 {
		return fmt.Errorf("JWT_SECRET must be at least 32 characters")
	}
	return nil
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func envInt(key string, fallback int) int {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return n
}

func envDuration(key string, fallback time.Duration) time.Duration {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return fallback
	}
	return d
}
