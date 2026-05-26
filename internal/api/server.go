package api

import (
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/thomas-vilte/flagstone/internal/api/middleware"
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
}

// NewServer creates a new API Server with the given dependencies.
func NewServer(stores *storage.Stores, dbPool *pgxpool.Pool, cfg *config.Config, logger *zap.Logger) *Server {
	return &Server{
		stores: stores,
		dbPool: dbPool,
		cfg:    cfg,
		logger: logger,
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

	return mux
}

func (s *Server) withMiddleware(next http.Handler, mws ...func(http.Handler) http.Handler) http.Handler {
	for i := len(mws) - 1; i >= 0; i-- {
		next = mws[i](next)
	}
	return next
}
