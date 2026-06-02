package api

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/flagstonehq/flagstone/internal/api/middleware"
	"github.com/flagstonehq/flagstone/internal/auth"
	"github.com/flagstonehq/flagstone/internal/storage"
	"go.uber.org/zap"
)

type createAPIKeyRequest struct {
	Name      string     `json:"name"`
	EnvHint   *string    `json:"env_hint,omitempty"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
}

type apiKeyResponse struct {
	ID            uuid.UUID  `json:"id"`
	EnvironmentID uuid.UUID  `json:"environment_id"`
	Name          string     `json:"name"`
	KeyPrefix     string     `json:"key_prefix"`
	LastUsedAt    *time.Time `json:"last_used_at,omitempty"`
	ExpiresAt     *time.Time `json:"expires_at,omitempty"`
	RevokedAt     *time.Time `json:"revoked_at,omitempty"`
	CreatedAt     time.Time  `json:"created_at"`
	CreatedBy     *uuid.UUID `json:"created_by,omitempty"`
}

type createAPIKeyResponseBody struct {
	apiKeyResponse
	Key string `json:"key"`
}

func apiKeyResponseFromKey(k *storage.APIKey) apiKeyResponse {
	return apiKeyResponse{
		ID:            k.ID,
		EnvironmentID: k.EnvironmentID,
		Name:          k.Name,
		KeyPrefix:     k.KeyPrefix,
		LastUsedAt:    k.LastUsedAt,
		ExpiresAt:     k.ExpiresAt,
		RevokedAt:     k.RevokedAt,
		CreatedAt:     k.CreatedAt,
		CreatedBy:     k.CreatedBy,
	}
}

func (s *Server) handleCreateAPIKey(w http.ResponseWriter, r *http.Request) {
	var req createAPIKeyRequest
	if err := DecodeJSON(r, &req); err != nil {
		s.writeDecodeError(w, r, err)
		return
	}

	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		middleware.Error(w, r, http.StatusBadRequest, "VALIDATION_ERROR", "name is required.")
		return
	}
	if len(req.Name) > 255 {
		middleware.Error(w, r, http.StatusBadRequest, "VALIDATION_ERROR", "name must not exceed 255 characters.")
		return
	}

	envHint := "live"
	if req.EnvHint != nil {
		hint := strings.TrimSpace(*req.EnvHint)
		if hint != "" {
			envHint = hint
		}
	}

	claims := middleware.ClaimsFromContext(r.Context())
	tenantID, err := claims.TenantUUID()
	if err != nil {
		middleware.Error(w, r, http.StatusUnauthorized, "INVALID_CREDENTIALS", "Invalid token claims.")
		return
	}
	userID, err := claims.UserID()
	if err != nil {
		middleware.Error(w, r, http.StatusUnauthorized, "INVALID_CREDENTIALS", "Invalid token claims.")
		return
	}

	projectSlug := r.PathValue("slug")
	envSlug := r.PathValue("envSlug")

	project, err := s.stores.Projects.GetBySlug(r.Context(), tenantID, projectSlug)
	if err != nil {
		if errors.Is(err, storage.ErrProjectNotFound) {
			middleware.Error(w, r, http.StatusNotFound, "NOT_FOUND", "Project not found.")
			return
		}
		s.logger.Error("create apikey: get project", zap.Error(err))
		middleware.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "An internal server error occurred.")
		return
	}

	env, err := s.stores.Environments.GetBySlug(r.Context(), project.ID, envSlug)
	if err != nil {
		if errors.Is(err, storage.ErrEnvironmentNotFound) {
			middleware.Error(w, r, http.StatusNotFound, "NOT_FOUND", "Environment not found.")
			return
		}
		s.logger.Error("create apikey: get environment", zap.Error(err))
		middleware.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "An internal server error occurred.")
		return
	}

	rawKey, keyHash, keyPrefix, err := auth.GenerateAPIKey(envHint, 32)
	if err != nil {
		s.logger.Error("create apikey: generate key", zap.Error(err))
		middleware.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "An internal server error occurred.")
		return
	}

	key := &storage.APIKey{
		EnvironmentID: env.ID,
		Name:          req.Name,
		KeyHash:       keyHash,
		KeyPrefix:     keyPrefix,
		ExpiresAt:     req.ExpiresAt,
		CreatedBy:     uuidPtr(userID),
	}

	if err := s.stores.APIKeys.Create(r.Context(), key); err != nil {
		if errors.Is(err, storage.ErrDuplicateKey) {
			middleware.Error(w, r, http.StatusConflict, "DUPLICATE_KEY", "An API key with this hash already exists.")
			return
		}
		s.logger.Error("create apikey", zap.Error(err))
		middleware.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "An internal server error occurred.")
		return
	}

	s.writeAudit(r, tenantID, userID, "apikey.created", "api_key", uuidPtr(key.ID))

	resp := createAPIKeyResponseBody{
		apiKeyResponse: apiKeyResponseFromKey(key),
		Key:            rawKey,
	}

	middleware.JSON(w, http.StatusCreated, resp)
}

func (s *Server) handleListAPIKeys(w http.ResponseWriter, r *http.Request) {
	claims := middleware.ClaimsFromContext(r.Context())
	tenantID, err := claims.TenantUUID()
	if err != nil {
		middleware.Error(w, r, http.StatusUnauthorized, "INVALID_CREDENTIALS", "Invalid token claims.")
		return
	}

	projectSlug := r.PathValue("slug")
	envSlug := r.PathValue("envSlug")

	project, err := s.stores.Projects.GetBySlug(r.Context(), tenantID, projectSlug)
	if err != nil {
		if errors.Is(err, storage.ErrProjectNotFound) {
			middleware.Error(w, r, http.StatusNotFound, "NOT_FOUND", "Project not found.")
			return
		}
		s.logger.Error("list apikeys: get project", zap.Error(err))
		middleware.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "An internal server error occurred.")
		return
	}

	env, err := s.stores.Environments.GetBySlug(r.Context(), project.ID, envSlug)
	if err != nil {
		if errors.Is(err, storage.ErrEnvironmentNotFound) {
			middleware.Error(w, r, http.StatusNotFound, "NOT_FOUND", "Environment not found.")
			return
		}
		s.logger.Error("list apikeys: get environment", zap.Error(err))
		middleware.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "An internal server error occurred.")
		return
	}

	keys, err := s.stores.APIKeys.ListByEnvironment(r.Context(), env.ID)
	if err != nil {
		s.logger.Error("list apikeys", zap.Error(err))
		middleware.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "An internal server error occurred.")
		return
	}

	resp := make([]apiKeyResponse, 0, len(keys))
	for _, k := range keys {
		resp = append(resp, apiKeyResponseFromKey(&k))
	}

	middleware.JSON(w, http.StatusOK, resp)
}

func (s *Server) handleRevokeAPIKey(w http.ResponseWriter, r *http.Request) {
	claims := middleware.ClaimsFromContext(r.Context())
	tenantID, err := claims.TenantUUID()
	if err != nil {
		middleware.Error(w, r, http.StatusUnauthorized, "INVALID_CREDENTIALS", "Invalid token claims.")
		return
	}
	userID, err := claims.UserID()
	if err != nil {
		middleware.Error(w, r, http.StatusUnauthorized, "INVALID_CREDENTIALS", "Invalid token claims.")
		return
	}

	projectSlug := r.PathValue("slug")
	envSlug := r.PathValue("envSlug")
	keyIDStr := r.PathValue("id")

	keyID, err := uuid.Parse(keyIDStr)
	if err != nil {
		middleware.Error(w, r, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid API key ID format.")
		return
	}

	project, err := s.stores.Projects.GetBySlug(r.Context(), tenantID, projectSlug)
	if err != nil {
		if errors.Is(err, storage.ErrProjectNotFound) {
			middleware.Error(w, r, http.StatusNotFound, "NOT_FOUND", "Project not found.")
			return
		}
		s.logger.Error("revoke apikey: get project", zap.Error(err))
		middleware.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "An internal server error occurred.")
		return
	}

	_, err = s.stores.Environments.GetBySlug(r.Context(), project.ID, envSlug)
	if err != nil {
		if errors.Is(err, storage.ErrEnvironmentNotFound) {
			middleware.Error(w, r, http.StatusNotFound, "NOT_FOUND", "Environment not found.")
			return
		}
		s.logger.Error("revoke apikey: get environment", zap.Error(err))
		middleware.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "An internal server error occurred.")
		return
	}

	if err := s.stores.APIKeys.Revoke(r.Context(), keyID, time.Now().UTC()); err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			middleware.Error(w, r, http.StatusNotFound, "NOT_FOUND", "API key not found.")
			return
		}
		s.logger.Error("revoke apikey", zap.Error(err))
		middleware.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "An internal server error occurred.")
		return
	}

	s.writeAudit(r, tenantID, userID, "apikey.revoked", "api_key", uuidPtr(keyID))

	w.WriteHeader(http.StatusNoContent)
}
