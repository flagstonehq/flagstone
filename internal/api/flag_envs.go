package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/thomas-vilte/flagstone/internal/api/middleware"
	"github.com/thomas-vilte/flagstone/internal/storage"
	"go.uber.org/zap"
)

type updateFlagEnvRequest struct {
	Enabled      bool             `json:"enabled"`
	Rules        json.RawMessage  `json:"rules"`
	DefaultValue *json.RawMessage `json:"default_value,omitempty"`
	Version      int64            `json:"version"`
}

type flagEnvironmentResponse struct {
	FlagID        uuid.UUID        `json:"flag_id"`
	EnvironmentID uuid.UUID        `json:"environment_id"`
	Enabled       bool             `json:"enabled"`
	Rules         json.RawMessage  `json:"rules"`
	DefaultValue  *json.RawMessage `json:"default_value,omitempty"`
	Version       int64            `json:"version"`
	CreatedAt     string           `json:"created_at"`
	UpdatedAt     string           `json:"updated_at"`
	UpdatedBy     *uuid.UUID       `json:"updated_by,omitempty"`
}

func flagEnvResponseFromConfig(cfg *storage.FlagEnvironment) flagEnvironmentResponse {
	return flagEnvironmentResponse{
		FlagID:        cfg.FlagID,
		EnvironmentID: cfg.EnvironmentID,
		Enabled:       cfg.Enabled,
		Rules:         cfg.Rules,
		DefaultValue:  cfg.DefaultValue,
		Version:       cfg.Version,
		CreatedAt:     cfg.CreatedAt.UTC().Format(timeFormat),
		UpdatedAt:     cfg.UpdatedAt.UTC().Format(timeFormat),
		UpdatedBy:     cfg.UpdatedBy,
	}
}

func (s *Server) handleGetFlagEnvironment(w http.ResponseWriter, r *http.Request) {
	claims := middleware.ClaimsFromContext(r.Context())
	tenantID, err := claims.TenantUUID()
	if err != nil {
		middleware.Error(w, r, http.StatusUnauthorized, "INVALID_CREDENTIALS", "Invalid token claims.")
		return
	}

	projectSlug := r.PathValue("slug")
	flagKey := r.PathValue("key")
	envSlug := r.PathValue("envSlug")

	project, err := s.stores.Projects.GetBySlug(r.Context(), tenantID, projectSlug)
	if err != nil {
		if errors.Is(err, storage.ErrProjectNotFound) {
			middleware.Error(w, r, http.StatusNotFound, "NOT_FOUND", "Project not found.")
			return
		}
		s.logger.Error("get flag env: get project", zap.Error(err))
		middleware.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "An internal server error occurred.")
		return
	}

	flag, err := s.stores.Flags.GetByKey(r.Context(), project.ID, flagKey)
	if err != nil {
		if errors.Is(err, storage.ErrFlagNotFound) {
			middleware.Error(w, r, http.StatusNotFound, "NOT_FOUND", "Flag not found.")
			return
		}
		s.logger.Error("get flag env: get flag", zap.Error(err))
		middleware.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "An internal server error occurred.")
		return
	}

	env, err := s.stores.Environments.GetBySlug(r.Context(), project.ID, envSlug)
	if err != nil {
		if errors.Is(err, storage.ErrEnvironmentNotFound) {
			middleware.Error(w, r, http.StatusNotFound, "NOT_FOUND", "Environment not found.")
			return
		}
		s.logger.Error("get flag env: get env", zap.Error(err))
		middleware.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "An internal server error occurred.")
		return
	}

	cfg, err := s.stores.FlagEnvironments.Get(r.Context(), flag.ID, env.ID)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			middleware.Error(w, r, http.StatusNotFound, "NOT_FOUND", "Flag environment configuration not found.")
			return
		}
		s.logger.Error("get flag env", zap.Error(err))
		middleware.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "An internal server error occurred.")
		return
	}

	middleware.JSON(w, http.StatusOK, flagEnvResponseFromConfig(cfg))
}

func (s *Server) handleUpdateFlagEnvironment(w http.ResponseWriter, r *http.Request) {
	var req updateFlagEnvRequest
	if err := DecodeJSON(r, &req); err != nil {
		s.writeDecodeError(w, r, err)
		return
	}

	if len(req.Rules) > 0 {
		if err := ValidateFlagRules(req.Rules); err != nil {
			middleware.Error(w, r, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
			return
		}
	}
	if req.DefaultValue != nil && len(*req.DefaultValue) > 0 && !json.Valid(*req.DefaultValue) {
		middleware.Error(w, r, http.StatusBadRequest, "VALIDATION_ERROR", "default_value must be valid JSON.")
		return
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
	flagKey := r.PathValue("key")
	envSlug := r.PathValue("envSlug")

	project, err := s.stores.Projects.GetBySlug(r.Context(), tenantID, projectSlug)
	if err != nil {
		if errors.Is(err, storage.ErrProjectNotFound) {
			middleware.Error(w, r, http.StatusNotFound, "NOT_FOUND", "Project not found.")
			return
		}
		s.logger.Error("update flag env: get project", zap.Error(err))
		middleware.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "An internal server error occurred.")
		return
	}

	flag, err := s.stores.Flags.GetByKey(r.Context(), project.ID, flagKey)
	if err != nil {
		if errors.Is(err, storage.ErrFlagNotFound) {
			middleware.Error(w, r, http.StatusNotFound, "NOT_FOUND", "Flag not found.")
			return
		}
		s.logger.Error("update flag env: get flag", zap.Error(err))
		middleware.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "An internal server error occurred.")
		return
	}

	env, err := s.stores.Environments.GetBySlug(r.Context(), project.ID, envSlug)
	if err != nil {
		if errors.Is(err, storage.ErrEnvironmentNotFound) {
			middleware.Error(w, r, http.StatusNotFound, "NOT_FOUND", "Environment not found.")
			return
		}
		s.logger.Error("update flag env: get env", zap.Error(err))
		middleware.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "An internal server error occurred.")
		return
	}

	cfg := &storage.FlagEnvironment{
		FlagID:        flag.ID,
		EnvironmentID: env.ID,
		Enabled:       req.Enabled,
		Rules:         req.Rules,
		DefaultValue:  req.DefaultValue,
		UpdatedBy:     uuidPtr(userID),
	}

	if err := s.stores.FlagEnvironments.UpdateWithVersion(r.Context(), cfg, req.Version); err != nil {
		if errors.Is(err, storage.ErrVersionConflict) {
			middleware.Error(w, r, http.StatusConflict, "VERSION_CONFLICT", "The flag environment configuration was modified by another request. Reload and retry.")
			return
		}
		s.logger.Error("update flag env", zap.Error(err))
		middleware.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "An internal server error occurred.")
		return
	}

	if err := s.stores.AuditLogs.Insert(r.Context(), &storage.AuditLogEntry{
		TenantID:     tenantID,
		ActorID:      uuidPtr(userID),
		ActorType:    "user",
		Action:       "flag_environment.updated",
		ResourceType: "flag_environment",
		ResourceID:   nil,
		IPAddress:    requestIP(r),
		UserAgent:    stringPtr(strings.TrimSpace(r.UserAgent())),
	}); err != nil {
		s.logger.Error("update flag env: insert audit log", zap.Error(err))
	}

	middleware.JSON(w, http.StatusOK, flagEnvResponseFromConfig(cfg))
}
