package api

import (
	"context"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/thomas-vilte/flagstone/internal/api/middleware"
	"github.com/thomas-vilte/flagstone/internal/auth"
	"github.com/thomas-vilte/flagstone/internal/config"
	"github.com/thomas-vilte/flagstone/internal/storage"
	"go.uber.org/zap"
)

const (
	cleanupInterval       = 30 * time.Minute
	cleanupOpTimeout      = 30 * time.Second
	loginAttemptRetention = 1 * time.Hour
)

// Server holds shared dependencies for all API handlers and registers routes.
type Server struct {
	stores *storage.Stores
	dbPool *pgxpool.Pool
	cfg    *config.Config
	logger *zap.Logger

	// fakePasswordHash is a precomputed bcrypt hash used to keep login
	// response times constant when the email isn't found in the DB. Without
	// it, the absence of a bcrypt comparison would leak which emails exist.
	fakePasswordHash string
}

// NewServer creates a new API Server with the given dependencies.
func NewServer(stores *storage.Stores, dbPool *pgxpool.Pool, cfg *config.Config, logger *zap.Logger) *Server {
	fake, err := auth.HashPassword("flagstone-timing-decoy", cfg.BcryptCost)
	if err != nil {
		logger.Warn("could not precompute fake password hash; login timing oracle defense disabled", zap.Error(err))
	}
	return &Server{
		stores:           stores,
		dbPool:           dbPool,
		cfg:              cfg,
		logger:           logger,
		fakePasswordHash: fake,
	}
}

// Routes returns an http.Handler that serves all /api/v1/* endpoints.
func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()

	recoverMW := middleware.RecoverPanic(s.logger)

	setupHandler := s.withMiddleware(
		http.HandlerFunc(s.handleSetup),
		middleware.RequestID(),
		middleware.Logger(s.logger),
		middleware.BodyLimit(1<<20),
		middleware.RequireJSONContentType(),
	)
	mux.Handle("POST /api/v1/setup", recoverMW(setupHandler))

	loginHandler := s.withMiddleware(
		http.HandlerFunc(s.handleLogin),
		middleware.RequestID(),
		middleware.Logger(s.logger),
		middleware.BodyLimit(1<<20),
		middleware.RequireJSONContentType(),
	)
	mux.Handle("POST /api/v1/auth/login", recoverMW(loginHandler))

	refreshHandler := s.withMiddleware(
		http.HandlerFunc(s.handleRefresh),
		middleware.RequestID(),
		middleware.Logger(s.logger),
	)
	mux.Handle("POST /api/v1/auth/refresh", recoverMW(refreshHandler))

	logoutHandler := s.withMiddleware(
		http.HandlerFunc(s.handleLogout),
		middleware.RequestID(),
		middleware.Logger(s.logger),
		middleware.AuthJWT(s.cfg.JWTSecret),
	)
	mux.Handle("POST /api/v1/auth/logout", recoverMW(logoutHandler))

	return mux
}

func (s *Server) withMiddleware(next http.Handler, mws ...func(http.Handler) http.Handler) http.Handler {
	for i := len(mws) - 1; i >= 0; i-- {
		next = mws[i](next)
	}
	return next
}

// StartCleanup launches a background goroutine that periodically deletes
// expired revoked refresh tokens and old login attempt records.
func (s *Server) StartCleanup(ctx context.Context) {
	s.logger.Info("starting periodic cleanup", zap.Duration("interval", cleanupInterval))
	go func() {
		ticker := time.NewTicker(cleanupInterval)
		defer ticker.Stop()

		s.runOnce(ctx)

		for {
			select {
			case <-ticker.C:
				s.runOnce(ctx)
			case <-ctx.Done():
				s.logger.Info("cleanup stopped")
				return
			}
		}
	}()
}

func (s *Server) runOnce(ctx context.Context) {
	s.runCleanup(ctx, "delete expired revoked tokens", s.stores.RevokedTokens.DeleteExpired)
	s.runCleanup(ctx, "delete old login attempts", func(opCtx context.Context) (int64, error) {
		return s.stores.LoginAttempts.DeleteOlderThan(opCtx, time.Now().UTC().Add(-loginAttemptRetention))
	})
}

func (s *Server) runCleanup(ctx context.Context, name string, fn func(context.Context) (int64, error)) {
	opCtx, cancel := context.WithTimeout(ctx, cleanupOpTimeout)
	defer cancel()
	n, err := fn(opCtx)
	if err != nil {
		s.logger.Error("cleanup: "+name, zap.Error(err))
		return
	}
	if n > 0 {
		s.logger.Info("cleanup: "+name, zap.Int64("count", n))
	}
}
