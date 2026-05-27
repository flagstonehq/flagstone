package api

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/mail"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/thomas-vilte/flagstone/internal/api/middleware"
	"github.com/thomas-vilte/flagstone/internal/auth"
	"github.com/thomas-vilte/flagstone/internal/storage"
	"go.uber.org/zap"
)

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type tokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	ExpiresIn   int64  `json:"expires_in"`
}

var errMultipleTenants = errors.New("multiple tenants for user")

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

	user, err := s.stores.Users.GetByEmail(r.Context(), req.Email)
	if err != nil {
		if errors.Is(err, storage.ErrUserNotFound) {
			middleware.Error(w, r, http.StatusUnauthorized, "INVALID_CREDENTIALS", "Invalid email or password.")
			return
		}
		s.logger.Error("login: get user by email", zap.Error(err))
		middleware.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "An internal server error occurred.")
		return
	}

	if user.PasswordHash == nil || auth.VerifyPassword(*user.PasswordHash, req.Password) != nil {
		middleware.Error(w, r, http.StatusUnauthorized, "INVALID_CREDENTIALS", "Invalid email or password.")
		return
	}

	tenantID, role, err := s.resolveUserTenant(r.Context(), user.ID)
	if err != nil {
		switch {
		case errors.Is(err, errMultipleTenants):
			middleware.Error(w, r, http.StatusConflict, "MULTIPLE_TENANTS", "User belongs to multiple tenants.")
		case errors.Is(err, storage.ErrNotFound):
			middleware.Error(w, r, http.StatusUnauthorized, "INVALID_CREDENTIALS", "Invalid email or password.")
		default:
			s.logger.Error("login: resolve tenant", zap.Error(err))
			middleware.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "An internal server error occurred.")
		}
		return
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

	if err := s.stores.Sessions.Create(r.Context(), session); err != nil {
		s.logger.Error("login: create session", zap.Error(err))
		middleware.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "An internal server error occurred.")
		return
	}

	if err := s.stores.Users.UpdateLastLogin(r.Context(), user.ID, now); err != nil {
		s.logger.Error("login: update last login", zap.Error(err))
	}

	if err := s.stores.AuditLogs.Insert(r.Context(), &storage.AuditLogEntry{
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

	refreshHash := auth.HashRefreshToken(cookie.Value)
	session, err := s.stores.Sessions.GetByRefreshHash(r.Context(), refreshHash)
	if err != nil {
		middleware.Error(w, r, http.StatusUnauthorized, "INVALID_CREDENTIALS", "Invalid or expired refresh token.")
		return
	}

	now := time.Now().UTC()
	if !session.ExpiresAt.After(now) {
		_ = s.stores.Sessions.DeleteByID(r.Context(), session.ID)
		middleware.Error(w, r, http.StatusUnauthorized, "INVALID_CREDENTIALS", "Invalid or expired refresh token.")
		return
	}

	user, err := s.stores.Users.GetByID(r.Context(), session.UserID)
	if err != nil {
		middleware.Error(w, r, http.StatusUnauthorized, "INVALID_CREDENTIALS", "Invalid or expired refresh token.")
		return
	}

	role, err := s.stores.Members.GetRole(r.Context(), session.TenantID, session.UserID)
	if err != nil {
		middleware.Error(w, r, http.StatusUnauthorized, "INVALID_CREDENTIALS", "Invalid or expired refresh token.")
		return
	}

	if err := s.stores.Sessions.DeleteByID(r.Context(), session.ID); err != nil {
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

	if err := s.stores.Sessions.Create(r.Context(), newSession); err != nil {
		s.logger.Error("refresh: create session", zap.Error(err))
		middleware.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "An internal server error occurred.")
		return
	}

	accessToken, err := auth.GenerateAccessToken(user.ID, session.TenantID, role, s.cfg.JWTSecret, s.cfg.AccessTokenTTL)
	if err != nil {
		s.logger.Error("refresh: generate access token", zap.Error(err))
		middleware.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "An internal server error occurred.")
		return
	}

	if err := s.stores.AuditLogs.Insert(r.Context(), &storage.AuditLogEntry{
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
	if cookie, err := r.Cookie("refresh_token"); err == nil && strings.TrimSpace(cookie.Value) != "" {
		refreshHash := auth.HashRefreshToken(cookie.Value)
		if session, err := s.stores.Sessions.GetByRefreshHash(r.Context(), refreshHash); err == nil {
			_ = s.stores.Sessions.DeleteByID(r.Context(), session.ID)
		}
	}
	if err := s.stores.AuditLogs.Insert(r.Context(), &storage.AuditLogEntry{
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

func (s *Server) resolveUserTenant(ctx context.Context, userID uuid.UUID) (uuid.UUID, string, error) {
	members, err := s.stores.Members.ListByUser(ctx, userID)
	if err != nil {
		return uuid.Nil, "", err
	}

	if len(members) == 0 {
		return uuid.Nil, "", storage.ErrNotFound
	}

	if len(members) > 1 {
		return uuid.Nil, "", errMultipleTenants
	}
	return members[0].TenantID, members[0].Role, nil
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
