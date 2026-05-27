package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/mail"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/thomas-vilte/flagstone/internal/api/middleware"
	"github.com/thomas-vilte/flagstone/internal/auth"
	"github.com/thomas-vilte/flagstone/internal/storage"
	"go.uber.org/zap"
)

const (
	maxFailedLoginAttempts = 5
	loginLockoutWindow     = 15 * time.Minute
)

type loginRequest struct {
	Email      string `json:"email"`
	Password   string `json:"password"`
	TenantSlug string `json:"tenant_slug,omitempty"`
}

type tokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	ExpiresIn   int64  `json:"expires_in"`
}

// tenantSummary is the public shape of a tenant returned to the client
// when they need to disambiguate between memberships.
type tenantSummary struct {
	ID   uuid.UUID `json:"id"`
	Slug string    `json:"slug"`
	Name string    `json:"name"`
}

var (
	errMultipleTenants = errors.New("multiple tenants for user")
	errTenantMismatch  = errors.New("user is not a member of the requested tenant")
	errNoTenantAccess  = errors.New("user has no tenant access")
)

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := DecodeJSON(r, &req); err != nil {
		s.writeDecodeError(w, r, err)
		return
	}

	if err := validateLogin(&req); err != nil {
		middleware.Error(w, r, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
		return
	}

	ctx := r.Context()

	user, err := s.stores.Users.GetByEmail(ctx, req.Email)
	if err != nil {
		if errors.Is(err, storage.ErrUserNotFound) {
			_ = auth.VerifyPassword(s.fakePasswordHash, req.Password)
			middleware.Error(w, r, http.StatusUnauthorized, "INVALID_CREDENTIALS", "Invalid email or password.")
			return
		}
		s.logger.Error("login: get user by email", zap.Error(err))
		middleware.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "An internal server error occurred.")
		return
	}

	count, err := s.stores.LoginAttempts.CountSince(ctx, user.ID, time.Now().UTC().Add(-loginLockoutWindow))
	if err != nil {
		s.logger.Error("login: count attempts", zap.Error(err))
		middleware.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "An internal server error occurred.")
		return
	}
	if count >= maxFailedLoginAttempts {
		middleware.Error(w, r, http.StatusLocked, "ACCOUNT_LOCKED", "Account temporarily locked due to too many failed attempts. Try again later.")
		return
	}

	if user.PasswordHash == nil || auth.VerifyPassword(*user.PasswordHash, req.Password) != nil {
		s.recordFailedLogin(ctx, user.ID, r)
		middleware.Error(w, r, http.StatusUnauthorized, "INVALID_CREDENTIALS", "Invalid email or password.")
		return
	}

	tenantID, role, err := s.resolveUserTenant(ctx, user.ID, req.TenantSlug)
	if err != nil {
		switch {
		case errors.Is(err, errMultipleTenants):
			s.writeMultipleTenants(ctx, w, r, user.ID)
		case errors.Is(err, errNoTenantAccess),
			errors.Is(err, errTenantMismatch),
			errors.Is(err, storage.ErrTenantNotFound),
			errors.Is(err, storage.ErrNotFound):
			middleware.Error(w, r, http.StatusUnauthorized, "INVALID_CREDENTIALS", "Invalid email or password.")
		default:
			s.logger.Error("login: resolve tenant", zap.Error(err))
			middleware.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "An internal server error occurred.")
		}
		return
	}

	if err := s.stores.LoginAttempts.ClearForUser(ctx, user.ID); err != nil {
		s.logger.Warn("login: clear attempts", zap.Error(err))
	}

	accessToken, err := auth.GenerateAccessToken(user.ID, tenantID, role, s.cfg.JWTSecret, s.cfg.AccessTokenTTL)
	if err != nil {
		s.logger.Error("login: generate access token", zap.Error(err))
		middleware.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "An internal server error occurred.")
		return
	}

	refreshRaw, refreshHash, err := auth.GenerateRefreshToken(32)
	if err != nil {
		s.logger.Error("login: generate refresh token", zap.Error(err))
		middleware.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "An internal server error occurred.")
		return
	}

	now := time.Now().UTC()
	session := &storage.Session{
		UserID:      user.ID,
		TenantID:    tenantID,
		RefreshHash: refreshHash,
		UserAgent:   stringPtr(strings.TrimSpace(r.UserAgent())),
		IPAddress:   requestIP(r),
		ExpiresAt:   now.Add(s.cfg.RefreshTokenTTL),
	}

	if err := s.stores.Sessions.Create(ctx, session); err != nil {
		s.logger.Error("login: create session", zap.Error(err))
		middleware.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "An internal server error occurred.")
		return
	}

	if err := s.stores.Users.UpdateLastLogin(ctx, user.ID, now); err != nil {
		s.logger.Error("login: update last login", zap.Error(err))
	}

	if err := s.stores.AuditLogs.Insert(ctx, &storage.AuditLogEntry{
		TenantID:     tenantID,
		ActorID:      uuidPtr(user.ID),
		ActorType:    "user",
		Action:       "auth.login",
		ResourceType: "user",
		ResourceID:   uuidPtr(user.ID),
		IPAddress:    requestIP(r),
		UserAgent:    stringPtr(strings.TrimSpace(r.UserAgent())),
	}); err != nil {
		s.logger.Error("login: insert audit log", zap.Error(err))
	}

	s.setRefreshCookie(w, refreshRaw, now.Add(s.cfg.RefreshTokenTTL))
	middleware.JSON(w, http.StatusOK, tokenResponse{
		AccessToken: accessToken,
		TokenType:   "Bearer",
		ExpiresIn:   int64(s.cfg.AccessTokenTTL.Seconds()),
	})
}

func (s *Server) handleRefresh(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie("refresh_token")
	if err != nil || strings.TrimSpace(cookie.Value) == "" {
		middleware.Error(w, r, http.StatusUnauthorized, "INVALID_CREDENTIALS", "Invalid or expired refresh token.")
		return
	}

	ctx := r.Context()
	refreshHash := auth.HashRefreshToken(cookie.Value)

	if revoked, err := s.stores.RevokedTokens.Lookup(ctx, refreshHash); err == nil {
		s.handleRefreshReuse(ctx, w, r, revoked)
		return
	} else if !errors.Is(err, storage.ErrNotFound) {
		s.logger.Error("refresh: revoked lookup", zap.Error(err))
		middleware.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "An internal server error occurred.")
		return
	}

	session, err := s.stores.Sessions.GetByRefreshHash(ctx, refreshHash)
	if err != nil {
		middleware.Error(w, r, http.StatusUnauthorized, "INVALID_CREDENTIALS", "Invalid or expired refresh token.")
		return
	}

	now := time.Now().UTC()
	if !session.ExpiresAt.After(now) {
		_ = s.stores.Sessions.DeleteByID(ctx, session.ID)
		middleware.Error(w, r, http.StatusUnauthorized, "INVALID_CREDENTIALS", "Invalid or expired refresh token.")
		return
	}

	user, err := s.stores.Users.GetByID(ctx, session.UserID)
	if err != nil {
		middleware.Error(w, r, http.StatusUnauthorized, "INVALID_CREDENTIALS", "Invalid or expired refresh token.")
		return
	}

	role, err := s.stores.Members.GetRole(ctx, session.TenantID, session.UserID)
	if err != nil {
		middleware.Error(w, r, http.StatusUnauthorized, "INVALID_CREDENTIALS", "Invalid or expired refresh token.")
		return
	}

	newRefreshRaw, newRefreshHash, err := auth.GenerateRefreshToken(32)
	if err != nil {
		s.logger.Error("refresh: generate refresh token", zap.Error(err))
		middleware.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "An internal server error occurred.")
		return
	}

	newSession := &storage.Session{
		UserID:      user.ID,
		TenantID:    session.TenantID,
		RefreshHash: newRefreshHash,
		UserAgent:   stringPtr(strings.TrimSpace(r.UserAgent())),
		IPAddress:   requestIP(r),
		ExpiresAt:   now.Add(s.cfg.RefreshTokenTTL),
	}

	if err := s.runInTx(ctx, func(txStores *storage.Stores) error {
		if err := txStores.RevokedTokens.Insert(ctx, &storage.RevokedRefreshToken{
			TokenHash: refreshHash,
			UserID:    user.ID,
			ExpiresAt: session.ExpiresAt,
		}); err != nil {
			return fmt.Errorf("revoke old token: %w", err)
		}
		if err := txStores.Sessions.DeleteByID(ctx, session.ID); err != nil {
			return fmt.Errorf("delete old session: %w", err)
		}
		if err := txStores.Sessions.Create(ctx, newSession); err != nil {
			return fmt.Errorf("create new session: %w", err)
		}
		return nil
	}); err != nil {
		s.logger.Error("refresh: rotate", zap.Error(err))
		middleware.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "An internal server error occurred.")
		return
	}

	accessToken, err := auth.GenerateAccessToken(user.ID, session.TenantID, role, s.cfg.JWTSecret, s.cfg.AccessTokenTTL)
	if err != nil {
		s.logger.Error("refresh: generate access token", zap.Error(err))
		middleware.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "An internal server error occurred.")
		return
	}

	if err := s.stores.AuditLogs.Insert(ctx, &storage.AuditLogEntry{
		TenantID:     session.TenantID,
		ActorID:      uuidPtr(user.ID),
		ActorType:    "user",
		Action:       "auth.refresh",
		ResourceType: "session",
		ResourceID:   uuidPtr(newSession.ID),
		IPAddress:    requestIP(r),
		UserAgent:    stringPtr(strings.TrimSpace(r.UserAgent())),
	}); err != nil {
		s.logger.Error("refresh: insert audit log", zap.Error(err))
	}

	s.setRefreshCookie(w, newRefreshRaw, now.Add(s.cfg.RefreshTokenTTL))
	middleware.JSON(w, http.StatusOK, tokenResponse{
		AccessToken: accessToken,
		TokenType:   "Bearer",
		ExpiresIn:   int64(s.cfg.AccessTokenTTL.Seconds()),
	})
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	claims := middleware.ClaimsFromContext(r.Context())
	if claims == nil {
		middleware.Error(w, r, http.StatusUnauthorized, "INVALID_CREDENTIALS", "Authentication is required.")
		return
	}
	userID, err := claims.UserID()
	if err != nil {
		middleware.Error(w, r, http.StatusUnauthorized, "INVALID_CREDENTIALS", "Authentication is required.")
		return
	}
	tenantID, err := claims.TenantUUID()
	if err != nil {
		middleware.Error(w, r, http.StatusUnauthorized, "INVALID_CREDENTIALS", "Authentication is required.")
		return
	}

	ctx := r.Context()
	if cookie, err := r.Cookie("refresh_token"); err == nil && strings.TrimSpace(cookie.Value) != "" {
		refreshHash := auth.HashRefreshToken(cookie.Value)
		if session, err := s.stores.Sessions.GetByRefreshHash(ctx, refreshHash); err == nil {
			_ = s.runInTx(ctx, func(txStores *storage.Stores) error {
				if err := txStores.RevokedTokens.Insert(ctx, &storage.RevokedRefreshToken{
					TokenHash: refreshHash,
					UserID:    session.UserID,
					ExpiresAt: session.ExpiresAt,
				}); err != nil {
					return err
				}
				return txStores.Sessions.DeleteByID(ctx, session.ID)
			})
		}
	}

	if err := s.stores.AuditLogs.Insert(ctx, &storage.AuditLogEntry{
		TenantID:     tenantID,
		ActorID:      uuidPtr(userID),
		ActorType:    "user",
		Action:       "auth.logout",
		ResourceType: "user",
		ResourceID:   uuidPtr(userID),
		IPAddress:    requestIP(r),
		UserAgent:    stringPtr(strings.TrimSpace(r.UserAgent())),
	}); err != nil {
		s.logger.Error("logout: insert audit log", zap.Error(err))
	}
	s.clearRefreshCookie(w)
	w.WriteHeader(http.StatusNoContent)
}

// handleRefreshReuse responds to a replayed refresh token. We kill every
// session for the user and audit it so an operator can investigate.
func (s *Server) handleRefreshReuse(ctx context.Context, w http.ResponseWriter, r *http.Request, revoked *storage.RevokedRefreshToken) {
	if err := s.stores.Sessions.DeleteByUserID(ctx, revoked.UserID); err != nil {
		s.logger.Error("refresh reuse: delete sessions", zap.Error(err))
	}
	if err := s.stores.AuditLogs.Insert(ctx, &storage.AuditLogEntry{
		ActorID:      uuidPtr(revoked.UserID),
		ActorType:    "user",
		Action:       "auth.refresh_reuse",
		ResourceType: "user",
		ResourceID:   uuidPtr(revoked.UserID),
		IPAddress:    requestIP(r),
		UserAgent:    stringPtr(strings.TrimSpace(r.UserAgent())),
	}); err != nil {
		s.logger.Error("refresh reuse: audit log", zap.Error(err))
	}
	s.logger.Warn("refresh token reuse detected", zap.String("user_id", revoked.UserID.String()))
	s.clearRefreshCookie(w)
	middleware.Error(w, r, http.StatusUnauthorized, "INVALID_CREDENTIALS", "Invalid or expired refresh token.")
}

func (s *Server) recordFailedLogin(ctx context.Context, userID uuid.UUID, r *http.Request) {
	err := s.stores.LoginAttempts.Record(ctx, &storage.LoginAttempt{
		UserID:    userID,
		IPAddress: requestIP(r),
		UserAgent: stringPtr(strings.TrimSpace(r.UserAgent())),
	})
	if err != nil {
		s.logger.Warn("login: record attempt", zap.Error(err))
	}
}

func validateLogin(req *loginRequest) error {
	if strings.TrimSpace(req.Email) == "" {
		return fmt.Errorf("email is required")
	}

	if _, err := mail.ParseAddress(req.Email); err != nil {
		return fmt.Errorf("email is not a valid email address")
	}

	if req.Password == "" {
		return fmt.Errorf("password is required")
	}
	return nil
}

func (s *Server) writeDecodeError(w http.ResponseWriter, r *http.Request, err error) {
	code := "VALIDATION_ERROR"
	status := http.StatusBadRequest
	msg := err.Error()

	switch {
	case errors.Is(err, ErrBodyTooLarge):
		status = http.StatusRequestEntityTooLarge
		code = "BODY_TOO_LARGE"
		msg = "Request body must not exceed 1 MB."
	case errors.Is(err, ErrUnsupportedContentType):
		status = http.StatusUnsupportedMediaType
		code = "UNSUPPORTED_MEDIA_TYPE"
		msg = "Content-Type must be application/json."
	}

	middleware.Error(w, r, status, code, msg)
}

func (s *Server) setRefreshCookie(w http.ResponseWriter, raw string, expiresAt time.Time) {
	http.SetCookie(w, &http.Cookie{
		Name:     "refresh_token",
		Value:    raw,
		HttpOnly: true,
		Secure:   s.cfg.IsProd(),
		SameSite: http.SameSiteStrictMode,
		Path:     "/api/v1/auth",
		Expires:  expiresAt,
	})
}

func (s *Server) clearRefreshCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     "refresh_token",
		Value:    "",
		HttpOnly: true,
		Secure:   s.cfg.IsProd(),
		SameSite: http.SameSiteStrictMode,
		Path:     "/api/v1/auth",
		MaxAge:   -1,
		Expires:  time.Unix(0, 0),
	})
}

// resolveUserTenant picks the tenant for the login. The flow is:
//
//   - tenantSlug != "": find the tenant, then verify the user is a member.
//     Mismatches fall back to INVALID_CREDENTIALS so we don't leak which
//     tenants exist.
//
//   - tenantSlug == "" and the user has exactly one tenant: use it.
//
//   - tenantSlug == "" and the user has many tenants: surface
//     errMultipleTenants so the caller can prompt for a slug.
func (s *Server) resolveUserTenant(ctx context.Context, userID uuid.UUID, tenantSlug string) (uuid.UUID, string, error) {
	tenantSlug = strings.TrimSpace(tenantSlug)
	if tenantSlug != "" {
		tenant, err := s.stores.Tenants.GetBySlug(ctx, tenantSlug)
		if err != nil {
			return uuid.Nil, "", err
		}
		role, err := s.stores.Members.GetRole(ctx, tenant.ID, userID)
		if err != nil {
			if errors.Is(err, storage.ErrNotFound) {
				return uuid.Nil, "", errTenantMismatch
			}
			return uuid.Nil, "", err
		}
		return tenant.ID, role, nil
	}

	members, err := s.stores.Members.ListByUser(ctx, userID)
	if err != nil {
		return uuid.Nil, "", err
	}
	if len(members) == 0 {
		return uuid.Nil, "", errNoTenantAccess
	}
	if len(members) > 1 {
		return uuid.Nil, "", errMultipleTenants
	}
	return members[0].TenantID, members[0].Role, nil
}

// writeMultipleTenants returns a 409 listing the tenants the user could
// pick from. The client retries the login including tenant_slug.
func (s *Server) writeMultipleTenants(ctx context.Context, w http.ResponseWriter, r *http.Request, userID uuid.UUID) {
	members, err := s.stores.Members.ListByUser(ctx, userID)
	if err != nil {
		s.logger.Error("login: list memberships", zap.Error(err))
		middleware.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "An internal server error occurred.")
		return
	}

	summaries := make([]tenantSummary, 0, len(members))
	for _, m := range members {
		tenant, err := s.stores.Tenants.GetByID(ctx, m.TenantID)
		if err != nil {
			s.logger.Warn("login: fetch tenant for multi-tenant response",
				zap.String("tenant_id", m.TenantID.String()), zap.Error(err))
			continue
		}
		summaries = append(summaries, tenantSummary{
			ID:   tenant.ID,
			Slug: tenant.Slug,
			Name: tenant.Name,
		})
	}

	reqID := middleware.RequestIDFromContext(r.Context())
	payload := struct {
		Error struct {
			Code             string          `json:"code"`
			Message          string          `json:"message"`
			RequestID        string          `json:"request_id,omitempty"`
			AvailableTenants []tenantSummary `json:"available_tenants"`
		} `json:"error"`
	}{}
	payload.Error.Code = "MULTIPLE_TENANTS"
	payload.Error.Message = "User belongs to multiple tenants. Retry with tenant_slug."
	payload.Error.RequestID = reqID
	payload.Error.AvailableTenants = summaries

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusConflict)
	_ = json.NewEncoder(w).Encode(payload)
}

// runInTx executes fn inside a serializable-isolation transaction. The
// stores fn receives all run inside that tx, so any failure rolls back
// every write together.
func (s *Server) runInTx(ctx context.Context, fn func(txStores *storage.Stores) error) error {
	tx, err := s.stores.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if err := fn(s.stores.WithTx(tx)); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}
	return nil
}

func requestIP(r *http.Request) *net.IP {
	host := strings.TrimSpace(r.Header.Get("X-Forwarded-For"))
	if host == "" {
		host = strings.TrimSpace(r.Header.Get("X-Real-IP"))
	}

	if host == "" {
		remoteHost, _, err := net.SplitHostPort(r.RemoteAddr)
		if err == nil {
			host = remoteHost
		} else {
			host = r.RemoteAddr
		}
	}
	host = strings.TrimSpace(strings.Split(host, ",")[0])
	ip := net.ParseIP(host)
	if ip == nil {
		return nil
	}

	return &ip
}

func stringPtr(v string) *string {
	if v == "" {
		return nil
	}
	return &v
}

func uuidPtr(id uuid.UUID) *uuid.UUID {
	return &id
}
