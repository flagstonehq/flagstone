package api

import (
	"context"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/thomas-vilte/flagstone/internal/api/middleware"
	"github.com/thomas-vilte/flagstone/internal/auth"
	"github.com/thomas-vilte/flagstone/internal/config"
	"github.com/thomas-vilte/flagstone/internal/engine"
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
	engine *engine.Engine

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
		engine:           engine.New(logger),
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

	projectsCreateHandler := s.withMiddleware(
		http.HandlerFunc(s.handleCreateProject),
		middleware.RequestID(),
		middleware.Logger(s.logger),
		middleware.BodyLimit(1<<20),
		middleware.RequireJSONContentType(),
		middleware.AuthJWT(s.cfg.JWTSecret),
		middleware.RequireRole(auth.RoleAdmin),
	)
	mux.Handle("POST /api/v1/projects", recoverMW(projectsCreateHandler))
	projectsListHandler := s.withMiddleware(
		http.HandlerFunc(s.handleListProjects),
		middleware.RequestID(),
		middleware.Logger(s.logger),
		middleware.AuthJWT(s.cfg.JWTSecret),
		middleware.RequireRole(auth.RoleViewer),
	)
	mux.Handle("GET /api/v1/projects", recoverMW(projectsListHandler))
	projectsGetHandler := s.withMiddleware(
		http.HandlerFunc(s.handleGetProject),
		middleware.RequestID(),
		middleware.Logger(s.logger),
		middleware.AuthJWT(s.cfg.JWTSecret),
		middleware.RequireRole(auth.RoleViewer),
	)
	mux.Handle("GET /api/v1/projects/{slug}", recoverMW(projectsGetHandler))
	projectsUpdateHandler := s.withMiddleware(
		http.HandlerFunc(s.handleUpdateProject),
		middleware.RequestID(),
		middleware.Logger(s.logger),
		middleware.BodyLimit(1<<20),
		middleware.RequireJSONContentType(),
		middleware.AuthJWT(s.cfg.JWTSecret),
		middleware.RequireRole(auth.RoleAdmin),
	)
	mux.Handle("PUT /api/v1/projects/{slug}", recoverMW(projectsUpdateHandler))

	envsListHandler := s.withMiddleware(
		http.HandlerFunc(s.handleListEnvironments),
		middleware.RequestID(),
		middleware.Logger(s.logger),
		middleware.AuthJWT(s.cfg.JWTSecret),
		middleware.RequireRole(auth.RoleViewer),
	)
	mux.Handle("GET /api/v1/projects/{slug}/environments", recoverMW(envsListHandler))

	envsCreateHandler := s.withMiddleware(
		http.HandlerFunc(s.handleCreateEnvironment),
		middleware.RequestID(),
		middleware.Logger(s.logger),
		middleware.BodyLimit(1<<20),
		middleware.RequireJSONContentType(),
		middleware.AuthJWT(s.cfg.JWTSecret),
		middleware.RequireRole(auth.RoleAdmin),
	)
	mux.Handle("POST /api/v1/projects/{slug}/environments", recoverMW(envsCreateHandler))
	envsDeleteHandler := s.withMiddleware(
		http.HandlerFunc(s.handleDeleteEnvironment),
		middleware.RequestID(),
		middleware.Logger(s.logger),
		middleware.AuthJWT(s.cfg.JWTSecret),
		middleware.RequireRole(auth.RoleOwner),
	)
	mux.Handle("DELETE /api/v1/projects/{slug}/environments/{envSlug}", recoverMW(envsDeleteHandler))

	flagsCreateHandler := s.withMiddleware(
		http.HandlerFunc(s.handleCreateFlag),
		middleware.RequestID(),
		middleware.Logger(s.logger),
		middleware.BodyLimit(1<<20),
		middleware.RequireJSONContentType(),
		middleware.AuthJWT(s.cfg.JWTSecret),
		middleware.RequireRole(auth.RoleMember),
	)
	mux.Handle("POST /api/v1/projects/{slug}/flags", recoverMW(flagsCreateHandler))
	flagsListHandler := s.withMiddleware(
		http.HandlerFunc(s.handleListFlags),
		middleware.RequestID(),
		middleware.Logger(s.logger),
		middleware.AuthJWT(s.cfg.JWTSecret),
		middleware.RequireRole(auth.RoleViewer),
	)
	mux.Handle("GET /api/v1/projects/{slug}/flags", recoverMW(flagsListHandler))
	flagsGetHandler := s.withMiddleware(
		http.HandlerFunc(s.handleGetFlag),
		middleware.RequestID(),
		middleware.Logger(s.logger),
		middleware.AuthJWT(s.cfg.JWTSecret),
		middleware.RequireRole(auth.RoleViewer),
	)
	mux.Handle("GET /api/v1/projects/{slug}/flags/{key}", recoverMW(flagsGetHandler))
	flagsUpdateHandler := s.withMiddleware(
		http.HandlerFunc(s.handleUpdateFlag),
		middleware.RequestID(),
		middleware.Logger(s.logger),
		middleware.BodyLimit(1<<20),
		middleware.RequireJSONContentType(),
		middleware.AuthJWT(s.cfg.JWTSecret),
		middleware.RequireRole(auth.RoleMember),
	)
	mux.Handle("PUT /api/v1/projects/{slug}/flags/{key}", recoverMW(flagsUpdateHandler))
	flagsArchiveHandler := s.withMiddleware(
		http.HandlerFunc(s.handleArchiveFlag),
		middleware.RequestID(),
		middleware.Logger(s.logger),
		middleware.AuthJWT(s.cfg.JWTSecret),
		middleware.RequireRole(auth.RoleAdmin),
	)
	mux.Handle("DELETE /api/v1/projects/{slug}/flags/{key}", recoverMW(flagsArchiveHandler))

	flagEnvGetHandler := s.withMiddleware(
		http.HandlerFunc(s.handleGetFlagEnvironment),
		middleware.RequestID(),
		middleware.Logger(s.logger),
		middleware.AuthJWT(s.cfg.JWTSecret),
		middleware.RequireRole(auth.RoleViewer),
	)
	mux.Handle("GET /api/v1/projects/{slug}/flags/{key}/environments/{envSlug}", recoverMW(flagEnvGetHandler))

	flagEnvUpdateHandler := s.withMiddleware(
		http.HandlerFunc(s.handleUpdateFlagEnvironment),
		middleware.RequestID(),
		middleware.Logger(s.logger),
		middleware.BodyLimit(1<<20),
		middleware.RequireJSONContentType(),
		middleware.AuthJWT(s.cfg.JWTSecret),
		middleware.RequireRole(auth.RoleMember),
	)
	mux.Handle("PUT /api/v1/projects/{slug}/flags/{key}/environments/{envSlug}", recoverMW(flagEnvUpdateHandler))

	segmentsCreateHandler := s.withMiddleware(
		http.HandlerFunc(s.handleCreateSegment),
		middleware.RequestID(),
		middleware.Logger(s.logger),
		middleware.BodyLimit(1<<20),
		middleware.RequireJSONContentType(),
		middleware.AuthJWT(s.cfg.JWTSecret),
		middleware.RequireRole(auth.RoleMember),
	)
	mux.Handle("POST /api/v1/projects/{slug}/segments", recoverMW(segmentsCreateHandler))
	segmentsListHandler := s.withMiddleware(
		http.HandlerFunc(s.handleListSegments),
		middleware.RequestID(),
		middleware.Logger(s.logger),
		middleware.AuthJWT(s.cfg.JWTSecret),
		middleware.RequireRole(auth.RoleViewer),
	)
	mux.Handle("GET /api/v1/projects/{slug}/segments", recoverMW(segmentsListHandler))
	segmentsGetHandler := s.withMiddleware(
		http.HandlerFunc(s.handleGetSegment),
		middleware.RequestID(),
		middleware.Logger(s.logger),
		middleware.AuthJWT(s.cfg.JWTSecret),
		middleware.RequireRole(auth.RoleViewer),
	)
	mux.Handle("GET /api/v1/projects/{slug}/segments/{key}", recoverMW(segmentsGetHandler))
	segmentsUpdateHandler := s.withMiddleware(
		http.HandlerFunc(s.handleUpdateSegment),
		middleware.RequestID(),
		middleware.Logger(s.logger),
		middleware.BodyLimit(1<<20),
		middleware.RequireJSONContentType(),
		middleware.AuthJWT(s.cfg.JWTSecret),
		middleware.RequireRole(auth.RoleMember),
	)
	mux.Handle("PUT /api/v1/projects/{slug}/segments/{key}", recoverMW(segmentsUpdateHandler))
	segmentsArchiveHandler := s.withMiddleware(
		http.HandlerFunc(s.handleArchiveSegment),
		middleware.RequestID(),
		middleware.Logger(s.logger),
		middleware.AuthJWT(s.cfg.JWTSecret),
		middleware.RequireRole(auth.RoleAdmin),
	)
	mux.Handle("DELETE /api/v1/projects/{slug}/segments/{key}", recoverMW(segmentsArchiveHandler))

	apikeysCreateHandler := s.withMiddleware(
		http.HandlerFunc(s.handleCreateAPIKey),
		middleware.RequestID(),
		middleware.Logger(s.logger),
		middleware.BodyLimit(1<<20),
		middleware.RequireJSONContentType(),
		middleware.AuthJWT(s.cfg.JWTSecret),
		middleware.RequireRole(auth.RoleAdmin),
	)
	mux.Handle("POST /api/v1/projects/{slug}/environments/{envSlug}/apikeys", recoverMW(apikeysCreateHandler))
	apikeysListHandler := s.withMiddleware(
		http.HandlerFunc(s.handleListAPIKeys),
		middleware.RequestID(),
		middleware.Logger(s.logger),
		middleware.AuthJWT(s.cfg.JWTSecret),
		middleware.RequireRole(auth.RoleMember),
	)
	mux.Handle("GET /api/v1/projects/{slug}/environments/{envSlug}/apikeys", recoverMW(apikeysListHandler))
	apikeysRevokeHandler := s.withMiddleware(
		http.HandlerFunc(s.handleRevokeAPIKey),
		middleware.RequestID(),
		middleware.Logger(s.logger),
		middleware.AuthJWT(s.cfg.JWTSecret),
		middleware.RequireRole(auth.RoleAdmin),
	)
	mux.Handle("DELETE /api/v1/projects/{slug}/environments/{envSlug}/apikeys/{id}", recoverMW(apikeysRevokeHandler))

	evaluateSingleHandler := s.withMiddleware(
		http.HandlerFunc(s.handleEvaluateFlag),
		middleware.RequestID(),
		middleware.Logger(s.logger),
		middleware.BodyLimit(1<<20),
		middleware.RequireJSONContentType(),
		middleware.AuthAPIKey(s.stores),
	)
	mux.Handle("POST /api/v1/evaluate/flags/{key}", recoverMW(evaluateSingleHandler))

	evaluateBulkHandler := s.withMiddleware(
		http.HandlerFunc(s.handleEvaluateFlags),
		middleware.RequestID(),
		middleware.Logger(s.logger),
		middleware.BodyLimit(1<<20),
		middleware.RequireJSONContentType(),
		middleware.AuthAPIKey(s.stores),
	)
	mux.Handle("POST /api/v1/evaluate/flags", recoverMW(evaluateBulkHandler))

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
