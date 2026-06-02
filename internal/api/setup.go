package api

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"errors"
	"fmt"
	"net/http"
	"net/mail"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/thomas-vilte/flagstone/internal/api/middleware"
	"github.com/thomas-vilte/flagstone/internal/auth"
	"go.uber.org/zap"
)

type setupRequest struct {
	TenantName    string `json:"tenant_name"`
	AdminEmail    string `json:"admin_email"`
	AdminPassword string `json:"admin_password"`
}

type setupResponse struct {
	TenantID    uuid.UUID `json:"tenant_id"`
	UserID      uuid.UUID `json:"user_id"`
	AccessToken string    `json:"access_token"`
}

func (s *Server) handleSetup(w http.ResponseWriter, r *http.Request) {
	var req setupRequest
	if err := DecodeJSON(r, &req); err != nil {
		code := "VALIDATION_ERROR"
		status := http.StatusBadRequest
		msg := err.Error()

		switch {
		case errors.Is(err, ErrBodyTooLarge):
			code = "BODY_TOO_LARGE"
			status = http.StatusRequestEntityTooLarge
			msg = "Request body must not exceed 1 MB."
		case errors.Is(err, ErrUnsupportedContentType):
			code = "UNSUPPORTED_MEDIA_TYPE"
			status = http.StatusUnsupportedMediaType
			msg = "Content-Type must be application/json."
		}

		middleware.Error(w, r, status, code, msg)
		return
	}

	if err := validateSetup(&req); err != nil {
		middleware.Error(w, r, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
		return
	}

	var alreadyInitialized bool
	if err := s.dbPool.QueryRow(r.Context(), `SELECT EXISTS(SELECT 1 FROM tenants)`).Scan(&alreadyInitialized); err != nil {
		s.logger.Error("failed to check bootstrap state", zap.Error(err))
		middleware.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "An internal server error occurred.")
		return
	}
	if alreadyInitialized {
		middleware.Error(w, r, http.StatusConflict, "ALREADY_INITIALIZED", "This instance is already set up.")
		return
	}

	tenantID := uuid.New()
	userID := uuid.New()
	slug := slugifyTenantName(req.TenantName)

	passwordHash, err := auth.HashPassword(req.AdminPassword, s.cfg.BcryptCost)
	if err != nil {
		s.logger.Error("failed to hash password", zap.Error(err))
		middleware.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "An internal server error occurred.")
		return
	}

	refreshRaw, refreshHash, err := auth.GenerateRefreshToken(32)
	if err != nil {
		s.logger.Error("failed to generate refresh token", zap.Error(err))
		middleware.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "An internal server error occurred.")
		return
	}

	now := time.Now().UTC()
	expiresAt := now.Add(s.cfg.RefreshTokenTTL)

	tx, err := s.dbPool.BeginTx(r.Context(), pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		s.logger.Error("failed to begin bootstrap transaction", zap.Error(err))
		middleware.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "An internal server error occurred.")
		return
	}
	defer func() { _ = tx.Rollback(r.Context()) }()

	if err := executeBootstrap(r.Context(), tx, tenantID, userID, slug, req, passwordHash, refreshHash, now, expiresAt); err != nil {
		if errors.Is(err, errAlreadyInitialized) {
			middleware.Error(w, r, http.StatusConflict, "ALREADY_INITIALIZED", "This instance is already set up.")
			return
		}
		s.logger.Error("bootstrap transaction failed", zap.Error(err))
		middleware.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "An internal server error occurred.")
		return
	}

	if err := tx.Commit(r.Context()); err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "40001" {
			middleware.Error(w, r, http.StatusConflict, "ALREADY_INITIALIZED", "This instance is already set up.")
			return
		}
		s.logger.Error("bootstrap commit failed", zap.Error(err))
		middleware.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "An internal server error occurred.")
		return
	}

	accessToken, err := auth.GenerateAccessToken(userID, tenantID, "owner", s.cfg.JWTSecret, s.cfg.AccessTokenTTL, uuid.Nil)
	if err != nil {
		s.logger.Error("failed to generate access token", zap.Error(err))
		middleware.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "An internal server error occurred.")
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "refresh_token",
		Value:    refreshRaw,
		HttpOnly: true,
		Secure:   s.cfg.IsProd(),
		SameSite: http.SameSiteStrictMode,
		Path:     "/api/v1/auth",
		Expires:  expiresAt,
	})

	middleware.JSON(w, http.StatusCreated, setupResponse{
		TenantID:    tenantID,
		UserID:      userID,
		AccessToken: accessToken,
	})
}

func validateSetup(req *setupRequest) error {
	if strings.TrimSpace(req.TenantName) == "" {
		return fmt.Errorf("tenant_name is required")
	}
	if len(req.TenantName) > 255 {
		return fmt.Errorf("tenant_name must not exceed 255 characters")
	}

	if strings.TrimSpace(req.AdminEmail) == "" {
		return fmt.Errorf("admin_email is required")
	}
	if _, err := mail.ParseAddress(req.AdminEmail); err != nil {
		return fmt.Errorf("admin_email is not a valid email address")
	}

	if req.AdminPassword == "" {
		return fmt.Errorf("admin_password is required")
	}
	if len(req.AdminPassword) < 8 {
		return fmt.Errorf("admin_password must be at least 8 characters")
	}
	if len(req.AdminPassword) > 72 {
		return fmt.Errorf("admin_password must not exceed 72 characters")
	}

	return nil
}

func cryptoRandInt64() int64 {
	var buf [8]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return 0
	}
	return int64(binary.LittleEndian.Uint64(buf[:]))
}

func (s *Server) handleSetupStatus(w http.ResponseWriter, r *http.Request) {
	initialized, err := s.stores.Tenants.ExistsAny(r.Context())
	if err != nil {
		s.logger.Error("setup status: check exists", zap.Error(err))
		middleware.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "An internal server error occurred.")
		return
	}
	middleware.JSON(w, http.StatusOK, struct {
		Initialized bool   `json:"initialized"`
		Message     string `json:"message,omitempty"`
	}{
		Initialized: initialized,
		Message:     "System is ready. Please sign in.",
	})
}

func slugifyTenantName(name string) string {
	var b strings.Builder
	prevDash := false
	for _, r := range strings.ToLower(strings.TrimSpace(name)) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			prevDash = false
		} else if !prevDash {
			b.WriteRune('-')
			prevDash = true
		}
	}
	slug := strings.Trim(b.String(), "-")
	if slug == "" {
		slug = fmt.Sprintf("tenant-%d", cryptoRandInt64()%100000)
	}
	return slug
}

var errAlreadyInitialized = errors.New("storage: already initialized")

// Bootstrap writes 5 tables atomically. Stores don't accept a tx yet, so
// this drops to raw SQL. Don't copy the pattern elsewhere — refactor stores
// to take a Querier when more handlers need multi-table txs.
func executeBootstrap(
	ctx context.Context,
	tx pgx.Tx,
	tenantID, userID uuid.UUID,
	slug string,
	req setupRequest,
	passwordHash, refreshHash string,
	now, expiresAt time.Time,
) error {
	var exists bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM tenants)`).Scan(&exists); err != nil {
		return fmt.Errorf("check tenants: %w", err)
	}
	if exists {
		return errAlreadyInitialized
	}

	if _, err := tx.Exec(ctx, `
		INSERT INTO tenants (id, slug, name, plan, created_at, updated_at)
		VALUES ($1, $2, $3, 'free', $4, $4)
	`, tenantID, slug, strings.TrimSpace(req.TenantName), now); err != nil {
		return fmt.Errorf("insert tenant: %w", err)
	}

	if _, err := tx.Exec(ctx, `
		INSERT INTO users (id, email, password_hash, email_verified_at, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $4, $4)
	`, userID, strings.TrimSpace(req.AdminEmail), passwordHash, now); err != nil {
		return fmt.Errorf("insert user: %w", err)
	}

	if _, err := tx.Exec(ctx, `
		INSERT INTO tenant_members (tenant_id, user_id, role, created_at)
		VALUES ($1, $2, 'owner', $3)
	`, tenantID, userID, now); err != nil {
		return fmt.Errorf("insert member: %w", err)
	}

	if _, err := tx.Exec(ctx, `
		INSERT INTO sessions (id, user_id, tenant_id, refresh_hash, expires_at, created_at)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, uuid.New(), userID, tenantID, refreshHash, expiresAt, now); err != nil {
		return fmt.Errorf("insert session: %w", err)
	}

	if _, err := tx.Exec(ctx, `
		INSERT INTO audit_log (id, tenant_id, actor_type, action, resource_type, resource_id, created_at)
		VALUES ($1, $2, 'system', 'system.bootstrap', 'tenant', $3, $4)
	`, uuid.New(), tenantID, tenantID, now); err != nil {
		return fmt.Errorf("insert audit log: %w", err)
	}

	return nil
}
