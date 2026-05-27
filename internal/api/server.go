package api

import (
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/thomas-vilte/flagstone/internal/api/middleware"
	"github.com/thomas-vilte/flagstone/internal/auth"
	"github.com/thomas-vilte/flagstone/internal/config"
	"github.com/thomas-vilte/flagstone/internal/storage"
	"go.uber.org/zap"
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
