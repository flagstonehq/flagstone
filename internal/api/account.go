package api

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/flagstonehq/flagstone/internal/api/middleware"
	"github.com/flagstonehq/flagstone/internal/auth"
	"github.com/flagstonehq/flagstone/internal/storage"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

type meResponse struct {
	ID          uuid.UUID  `json:"id"`
	Email       string     `json:"email"`
	Role        string     `json:"role"`
	CreatedAt   time.Time  `json:"created_at"`
	LastLoginAt *time.Time `json:"last_login_at,omitempty"`
}

type changePasswordRequest struct {
	CurrentPassword string `json:"current_password"`
	NewPassword     string `json:"new_password"`
	ConfirmPassword string `json:"confirm_password"`
}

type sessionResponse struct {
	ID        uuid.UUID `json:"id"`
	IPAddress *string   `json:"ip_address,omitempty"`
	UserAgent *string   `json:"user_agent,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	ExpiresAt time.Time `json:"expires_at"`
	IsCurrent bool      `json:"is_current"`
}

func (s *Server) handleGetMe(w http.ResponseWriter, r *http.Request) {
	claims := middleware.ClaimsFromContext(r.Context())
	if claims == nil {
		middleware.Error(w, r, http.StatusUnauthorized, "INVALID_CREDENTIALS", "Authentication is required.")
		return
	}

	userID, err := claims.UserID()
	if err != nil {
		middleware.Error(w, r, http.StatusUnauthorized, "INVALID_CREDENTIALS", "Invalid token claims.")
		return
	}
	tenantID, err := claims.TenantUUID()
	if err != nil {
		middleware.Error(w, r, http.StatusUnauthorized, "INVALID_CREDENTIALS", "Invalid token claims.")
		return
	}

	user, err := s.stores.Users.GetByID(r.Context(), userID)
	if err != nil {
		if errors.Is(err, storage.ErrUserNotFound) {
			middleware.Error(w, r, http.StatusNotFound, "NOT_FOUND", "User not found.")
			return
		}
		s.logger.Error("get me: get user", zap.Error(err))
		middleware.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "An internal server error occurred.")
		return
	}

	role, err := s.stores.Members.GetRole(r.Context(), tenantID, userID)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			middleware.Error(w, r, http.StatusForbidden, "INSUFFICIENT_PERMISSIONS", "Not a member of this tenant.")
			return
		}
		s.logger.Error("get me: get role", zap.Error(err))
		middleware.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "An internal server error occurred.")
		return
	}

	middleware.JSON(w, http.StatusOK, meResponse{
		ID:          user.ID,
		Email:       user.Email,
		Role:        role,
		CreatedAt:   user.CreatedAt,
		LastLoginAt: user.LastLoginAt,
	})
}

func (s *Server) handleChangePassword(w http.ResponseWriter, r *http.Request) {
	claims := middleware.ClaimsFromContext(r.Context())
	if claims == nil {
		middleware.Error(w, r, http.StatusUnauthorized, "INVALID_CREDENTIALS", "Authentication is required.")
		return
	}

	userID, err := claims.UserID()
	if err != nil {
		middleware.Error(w, r, http.StatusUnauthorized, "INVALID_CREDENTIALS", "Invalid token claims.")
		return
	}

	var req changePasswordRequest
	if err := DecodeJSON(r, &req); err != nil {
		s.writeDecodeError(w, r, err)
		return
	}

	if req.NewPassword != req.ConfirmPassword {
		middleware.Error(w, r, http.StatusBadRequest, "VALIDATION_ERROR", "New password and confirm password do not match.")
		return
	}
	if len(req.NewPassword) < 8 {
		middleware.Error(w, r, http.StatusBadRequest, "VALIDATION_ERROR", "New password must be at least 8 characters.")
		return
	}
	if len(req.NewPassword) > 72 {
		middleware.Error(w, r, http.StatusBadRequest, "VALIDATION_ERROR", "New password must not exceed 72 characters.")
		return
	}

	user, err := s.stores.Users.GetByID(r.Context(), userID)
	if err != nil {
		if errors.Is(err, storage.ErrUserNotFound) {
			middleware.Error(w, r, http.StatusNotFound, "NOT_FOUND", "User not found.")
			return
		}
		s.logger.Error("change password: get user", zap.Error(err))
		middleware.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "An internal server error occurred.")
		return
	}

	if user.PasswordHash == nil || auth.VerifyPassword(*user.PasswordHash, req.CurrentPassword) != nil {
		middleware.Error(w, r, http.StatusUnauthorized, "INVALID_CREDENTIALS", "Current password is incorrect.")
		return
	}

	newHash, err := auth.HashPassword(req.NewPassword, s.cfg.BcryptCost)
	if err != nil {
		s.logger.Error("change password: hash password", zap.Error(err))
		middleware.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "An internal server error occurred.")
		return
	}

	ctx := r.Context()
	if err := s.stores.Users.UpdatePasswordHash(ctx, userID, newHash); err != nil {
		s.logger.Error("change password: update hash", zap.Error(err))
		middleware.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "An internal server error occurred.")
		return
	}

	tenantID, err := claims.TenantUUID()
	if err == nil {
		if err := s.stores.AuditLogs.Insert(ctx, &storage.AuditLogEntry{
			TenantID:     tenantID,
			ActorID:      uuidPtr(userID),
			ActorType:    "user",
			Action:       "auth.password_changed",
			ResourceType: "user",
			ResourceID:   uuidPtr(userID),
			IPAddress:    requestIP(r),
			UserAgent:    stringPtr(strings.TrimSpace(r.UserAgent())),
		}); err != nil {
			s.logger.Error("change password: insert audit log", zap.Error(err))
		}
	}

	if err := s.stores.Sessions.DeleteByUserID(ctx, userID); err != nil {
		s.logger.Error("change password: delete sessions", zap.Error(err))
	}

	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleListSessions(w http.ResponseWriter, r *http.Request) {
	claims := middleware.ClaimsFromContext(r.Context())
	if claims == nil {
		middleware.Error(w, r, http.StatusUnauthorized, "INVALID_CREDENTIALS", "Authentication is required.")
		return
	}

	userID, err := claims.UserID()
	if err != nil {
		middleware.Error(w, r, http.StatusUnauthorized, "INVALID_CREDENTIALS", "Invalid token claims.")
		return
	}

	currentSID, _ := claims.SessionID()

	sessions, err := s.stores.Sessions.ListByUserID(r.Context(), userID)
	if err != nil {
		s.logger.Error("list sessions", zap.Error(err))
		middleware.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "An internal server error occurred.")
		return
	}

	resp := make([]sessionResponse, 0, len(sessions))
	for _, sess := range sessions {
		var ipStr *string
		if sess.IPAddress != nil {
			s := sess.IPAddress.String()
			ipStr = &s
		}
		resp = append(resp, sessionResponse{
			ID:        sess.ID,
			IPAddress: ipStr,
			UserAgent: sess.UserAgent,
			CreatedAt: sess.CreatedAt,
			ExpiresAt: sess.ExpiresAt,
			IsCurrent: currentSID != uuid.Nil && sess.ID == currentSID,
		})
	}

	middleware.JSON(w, http.StatusOK, resp)
}

func (s *Server) handleRevokeSession(w http.ResponseWriter, r *http.Request) {
	claims := middleware.ClaimsFromContext(r.Context())
	if claims == nil {
		middleware.Error(w, r, http.StatusUnauthorized, "INVALID_CREDENTIALS", "Authentication is required.")
		return
	}

	userID, err := claims.UserID()
	if err != nil {
		middleware.Error(w, r, http.StatusUnauthorized, "INVALID_CREDENTIALS", "Invalid token claims.")
		return
	}

	sessionIDStr := r.PathValue("id")
	sessionID, err := uuid.Parse(sessionIDStr)
	if err != nil {
		middleware.Error(w, r, http.StatusBadRequest, "VALIDATION_ERROR", "Session ID must be a valid UUID.")
		return
	}

	currentSID, _ := claims.SessionID()
	if currentSID != uuid.Nil && sessionID == currentSID {
		middleware.Error(w, r, http.StatusBadRequest, "VALIDATION_ERROR", "Cannot revoke the current session.")
		return
	}

	session, err := s.stores.Sessions.GetByID(r.Context(), sessionID)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			middleware.Error(w, r, http.StatusNotFound, "NOT_FOUND", "Session not found.")
			return
		}
		s.logger.Error("revoke session: get session", zap.Error(err))
		middleware.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "An internal server error occurred.")
		return
	}

	if session.UserID != userID {
		middleware.Error(w, r, http.StatusNotFound, "NOT_FOUND", "Session not found.")
		return
	}

	if err := s.stores.Sessions.DeleteByID(r.Context(), sessionID); err != nil {
		s.logger.Error("revoke session: delete", zap.Error(err))
		middleware.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "An internal server error occurred.")
		return
	}

	tenantID, err := claims.TenantUUID()
	if err == nil {
		if err := s.stores.AuditLogs.Insert(r.Context(), &storage.AuditLogEntry{
			TenantID:     tenantID,
			ActorID:      uuidPtr(userID),
			ActorType:    "user",
			Action:       "auth.session_revoked",
			ResourceType: "session",
			ResourceID:   uuidPtr(sessionID),
			IPAddress:    requestIP(r),
			UserAgent:    stringPtr(strings.TrimSpace(r.UserAgent())),
		}); err != nil {
			s.logger.Error("revoke session: insert audit log", zap.Error(err))
		}
	}

	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleRevokeAllSessions(w http.ResponseWriter, r *http.Request) {
	claims := middleware.ClaimsFromContext(r.Context())
	if claims == nil {
		middleware.Error(w, r, http.StatusUnauthorized, "INVALID_CREDENTIALS", "Authentication is required.")
		return
	}

	userID, err := claims.UserID()
	if err != nil {
		middleware.Error(w, r, http.StatusUnauthorized, "INVALID_CREDENTIALS", "Invalid token claims.")
		return
	}

	ctx := r.Context()
	if err := s.stores.Sessions.DeleteByUserID(ctx, userID); err != nil {
		s.logger.Error("revoke all sessions: delete", zap.Error(err))
		middleware.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "An internal server error occurred.")
		return
	}

	tenantID, err := claims.TenantUUID()
	if err == nil {
		if err := s.stores.AuditLogs.Insert(ctx, &storage.AuditLogEntry{
			TenantID:     tenantID,
			ActorID:      uuidPtr(userID),
			ActorType:    "user",
			Action:       "auth.sessions_revoked_all",
			ResourceType: "user",
			ResourceID:   uuidPtr(userID),
			IPAddress:    requestIP(r),
			UserAgent:    stringPtr(strings.TrimSpace(r.UserAgent())),
		}); err != nil {
			s.logger.Error("revoke all sessions: insert audit log", zap.Error(err))
		}
	}

	w.WriteHeader(http.StatusNoContent)
}
