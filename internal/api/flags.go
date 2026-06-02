package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/thomas-vilte/flagstone/internal/api/middleware"
	"github.com/thomas-vilte/flagstone/internal/storage"
	"go.uber.org/zap"
)

var validFlagTypes = map[string]bool{"boolean": true, "string": true, "number": true, "json": true}

type createFlagRequest struct {
	Key          string          `json:"key"`
	Name         string          `json:"name"`
	Description  *string         `json:"description,omitempty"`
	Type         string          `json:"type"`
	DefaultValue json.RawMessage `json:"default_value"`
}

type updateFlagRequest struct {
	Key          *string          `json:"key,omitempty"`
	Name         *string          `json:"name,omitempty"`
	Description  *string          `json:"description,omitempty"`
	Type         *string          `json:"type,omitempty"`
	DefaultValue *json.RawMessage `json:"default_value,omitempty"`
}

type flagResponse struct {
	ID           uuid.UUID       `json:"id"`
	ProjectID    uuid.UUID       `json:"project_id"`
	Key          string          `json:"key"`
	Name         string          `json:"name"`
	Description  *string         `json:"description,omitempty"`
	Type         string          `json:"type"`
	DefaultValue json.RawMessage `json:"default_value"`
	ArchivedAt   *time.Time      `json:"archived_at,omitempty"`
	CreatedAt    time.Time       `json:"created_at"`
	UpdatedAt    time.Time       `json:"updated_at"`
	CreatedBy    *uuid.UUID      `json:"created_by,omitempty"`
}

func flagResponseFromFlag(f *storage.Flag) flagResponse {
	return flagResponse{
		ID:           f.ID,
		ProjectID:    f.ProjectID,
		Key:          f.Key,
		Name:         f.Name,
		Description:  f.Description,
		Type:         f.Type,
		DefaultValue: f.DefaultValue,
		ArchivedAt:   f.ArchivedAt,
		CreatedAt:    f.CreatedAt,
		UpdatedAt:    f.UpdatedAt,
		CreatedBy:    f.CreatedBy,
	}
}

func (s *Server) handleCreateFlag(w http.ResponseWriter, r *http.Request) {
	var req createFlagRequest
	if err := DecodeJSON(r, &req); err != nil {
		s.writeDecodeError(w, r, err)
		return
	}

	if err := validateCreateFlag(&req); err != nil {
		middleware.Error(w, r, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
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
	project, err := s.stores.Projects.GetBySlug(r.Context(), tenantID, projectSlug)
	if err != nil {
		if errors.Is(err, storage.ErrProjectNotFound) {
			middleware.Error(w, r, http.StatusNotFound, "NOT_FOUND", "Project not found.")
			return
		}
		s.logger.Error("create flag: get project", zap.Error(err))
		middleware.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "An internal server error occurred.")
		return
	}

	var resp flagResponse
	if err := s.runInTx(r.Context(), func(txStores *storage.Stores) error {
		flag := &storage.Flag{
			ProjectID:    project.ID,
			Key:          req.Key,
			Name:         req.Name,
			Description:  req.Description,
			Type:         req.Type,
			DefaultValue: req.DefaultValue,
			CreatedBy:    uuidPtr(userID),
		}
		if err := txStores.Flags.Create(r.Context(), flag); err != nil {
			return err
		}

		envs, err := txStores.Environments.ListByProject(r.Context(), project.ID)
		if err != nil {
			return fmt.Errorf("list environments: %w", err)
		}

		for _, env := range envs {
			if err := txStores.FlagEnvironments.Upsert(r.Context(), &storage.FlagEnvironment{
				FlagID:        flag.ID,
				EnvironmentID: env.ID,
				Enabled:       false,
				Rules:         json.RawMessage("[]"),
			}); err != nil {
				return fmt.Errorf("upsert flag_env for %s: %w", env.Slug, err)
			}
		}

		if err := txStores.AuditLogs.Insert(r.Context(), &storage.AuditLogEntry{
			TenantID:     tenantID,
			ActorID:      uuidPtr(userID),
			ActorType:    "user",
			Action:       "flag.created",
			ResourceType: "flag",
			ResourceID:   uuidPtr(flag.ID),
			IPAddress:    requestIP(r),
			UserAgent:    stringPtr(strings.TrimSpace(r.UserAgent())),
		}); err != nil {
			return fmt.Errorf("insert audit log: %w", err)
		}

		resp = flagResponseFromFlag(flag)
		return nil
	}); err != nil {
		if errors.Is(err, storage.ErrDuplicateKey) {
			middleware.Error(w, r, http.StatusConflict, "DUPLICATE_KEY", "A flag with this key already exists in this project.")
			return
		}
		s.logger.Error("create flag", zap.Error(err))
		middleware.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "An internal server error occurred.")
		return
	}

	middleware.JSON(w, http.StatusCreated, resp)
}

func (s *Server) handleListFlags(w http.ResponseWriter, r *http.Request) {
	claims := middleware.ClaimsFromContext(r.Context())
	tenantID, err := claims.TenantUUID()
	if err != nil {
		middleware.Error(w, r, http.StatusUnauthorized, "INVALID_CREDENTIALS", "Invalid token claims.")
		return
	}

	projectSlug := r.PathValue("slug")
	project, err := s.stores.Projects.GetBySlug(r.Context(), tenantID, projectSlug)
	if err != nil {
		if errors.Is(err, storage.ErrProjectNotFound) {
			middleware.Error(w, r, http.StatusNotFound, "NOT_FOUND", "Project not found.")
			return
		}
		s.logger.Error("list flags: get project", zap.Error(err))
		middleware.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "An internal server error occurred.")
		return
	}

	flags, err := s.stores.Flags.ListByProject(r.Context(), project.ID)
	if err != nil {
		s.logger.Error("list flags", zap.Error(err))
		middleware.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "An internal server error occurred.")
		return
	}

	resp := make([]flagResponse, 0, len(flags))
	for _, f := range flags {
		resp = append(resp, flagResponseFromFlag(&f))
	}

	middleware.JSON(w, http.StatusOK, resp)
}

func (s *Server) handleGetFlag(w http.ResponseWriter, r *http.Request) {
	claims := middleware.ClaimsFromContext(r.Context())
	tenantID, err := claims.TenantUUID()
	if err != nil {
		middleware.Error(w, r, http.StatusUnauthorized, "INVALID_CREDENTIALS", "Invalid token claims.")
		return
	}

	projectSlug := r.PathValue("slug")
	flagKey := r.PathValue("key")

	project, err := s.stores.Projects.GetBySlug(r.Context(), tenantID, projectSlug)
	if err != nil {
		if errors.Is(err, storage.ErrProjectNotFound) {
			middleware.Error(w, r, http.StatusNotFound, "NOT_FOUND", "Project not found.")
			return
		}
		s.logger.Error("get flag: get project", zap.Error(err))
		middleware.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "An internal server error occurred.")
		return
	}

	flag, err := s.stores.Flags.GetByKey(r.Context(), project.ID, flagKey)
	if err != nil {
		if errors.Is(err, storage.ErrFlagNotFound) {
			middleware.Error(w, r, http.StatusNotFound, "NOT_FOUND", "Flag not found.")
			return
		}
		s.logger.Error("get flag", zap.Error(err))
		middleware.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "An internal server error occurred.")
		return
	}

	middleware.JSON(w, http.StatusOK, flagResponseFromFlag(flag))
}

func (s *Server) handleUpdateFlag(w http.ResponseWriter, r *http.Request) {
	var req updateFlagRequest
	if err := DecodeJSON(r, &req); err != nil {
		s.writeDecodeError(w, r, err)
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

	project, err := s.stores.Projects.GetBySlug(r.Context(), tenantID, projectSlug)
	if err != nil {
		if errors.Is(err, storage.ErrProjectNotFound) {
			middleware.Error(w, r, http.StatusNotFound, "NOT_FOUND", "Project not found.")
			return
		}
		s.logger.Error("update flag: get project", zap.Error(err))
		middleware.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "An internal server error occurred.")
		return
	}

	flag, err := s.stores.Flags.GetByKey(r.Context(), project.ID, flagKey)
	if err != nil {
		if errors.Is(err, storage.ErrFlagNotFound) {
			middleware.Error(w, r, http.StatusNotFound, "NOT_FOUND", "Flag not found.")
			return
		}
		s.logger.Error("update flag: get flag", zap.Error(err))
		middleware.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "An internal server error occurred.")
		return
	}

	if req.Key != nil {
		trimmed := strings.TrimSpace(*req.Key)
		if trimmed == "" {
			middleware.Error(w, r, http.StatusBadRequest, "VALIDATION_ERROR", "key must not be empty.")
			return
		}
		if len(trimmed) > 128 {
			middleware.Error(w, r, http.StatusBadRequest, "VALIDATION_ERROR", "key must not exceed 128 characters.")
			return
		}
		if !flagKeyRegex.MatchString(trimmed) {
			middleware.Error(w, r, http.StatusBadRequest, "VALIDATION_ERROR", "key must match: "+flagKeyPattern)
			return
		}
		flag.Key = trimmed
	}
	if req.Name != nil {
		trimmed := strings.TrimSpace(*req.Name)
		if trimmed == "" {
			middleware.Error(w, r, http.StatusBadRequest, "VALIDATION_ERROR", "name must not be empty.")
			return
		}
		if len(trimmed) > 255 {
			middleware.Error(w, r, http.StatusBadRequest, "VALIDATION_ERROR", "name must not exceed 255 characters.")
			return
		}
		flag.Name = trimmed
	}
	if req.Description != nil {
		trimmed := strings.TrimSpace(*req.Description)
		if len(trimmed) > 2000 {
			middleware.Error(w, r, http.StatusBadRequest, "VALIDATION_ERROR", "description must not exceed 2000 characters.")
			return
		}
		if trimmed == "" {
			flag.Description = nil
		} else {
			flag.Description = &trimmed
		}
	}
	if req.Type != nil {
		trimmed := strings.TrimSpace(*req.Type)
		if !validFlagTypes[trimmed] {
			middleware.Error(w, r, http.StatusBadRequest, "VALIDATION_ERROR", "type must be one of: boolean, string, number, json.")
			return
		}
		flag.Type = trimmed
	}
	if req.DefaultValue != nil {
		if !json.Valid(*req.DefaultValue) {
			middleware.Error(w, r, http.StatusBadRequest, "VALIDATION_ERROR", "default_value must be valid JSON.")
			return
		}
		flag.DefaultValue = *req.DefaultValue
	}

	if err := s.stores.Flags.Update(r.Context(), flag); err != nil {
		if errors.Is(err, storage.ErrDuplicateKey) {
			middleware.Error(w, r, http.StatusConflict, "DUPLICATE_KEY", "A flag with this key already exists in this project.")
			return
		}
		if errors.Is(err, storage.ErrFlagNotFound) {
			middleware.Error(w, r, http.StatusNotFound, "NOT_FOUND", "Flag not found.")
			return
		}
		s.logger.Error("update flag", zap.Error(err))
		middleware.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "An internal server error occurred.")
		return
	}

	s.writeAudit(r, tenantID, userID, "flag.updated", "flag", uuidPtr(flag.ID))

	middleware.JSON(w, http.StatusOK, flagResponseFromFlag(flag))
}

func (s *Server) handleArchiveFlag(w http.ResponseWriter, r *http.Request) {
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

	project, err := s.stores.Projects.GetBySlug(r.Context(), tenantID, projectSlug)
	if err != nil {
		if errors.Is(err, storage.ErrProjectNotFound) {
			middleware.Error(w, r, http.StatusNotFound, "NOT_FOUND", "Project not found.")
			return
		}
		s.logger.Error("archive flag: get project", zap.Error(err))
		middleware.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "An internal server error occurred.")
		return
	}

	flag, err := s.stores.Flags.GetByKey(r.Context(), project.ID, flagKey)
	if err != nil {
		if errors.Is(err, storage.ErrFlagNotFound) {
			middleware.Error(w, r, http.StatusNotFound, "NOT_FOUND", "Flag not found.")
			return
		}
		s.logger.Error("archive flag: get flag", zap.Error(err))
		middleware.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "An internal server error occurred.")
		return
	}

	now := time.Now().UTC()
	if err := s.stores.Flags.Archive(r.Context(), flag.ID, now); err != nil {
		if errors.Is(err, storage.ErrFlagNotFound) {
			middleware.Error(w, r, http.StatusNotFound, "NOT_FOUND", "Flag not found.")
			return
		}
		s.logger.Error("archive flag", zap.Error(err))
		middleware.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "An internal server error occurred.")
		return
	}

	if err := s.stores.AuditLogs.Insert(r.Context(), &storage.AuditLogEntry{
		TenantID:     tenantID,
		ActorID:      uuidPtr(userID),
		ActorType:    "user",
		Action:       "flag.archived",
		ResourceType: "flag",
		ResourceID:   uuidPtr(flag.ID),
		IPAddress:    requestIP(r),
		UserAgent:    stringPtr(strings.TrimSpace(r.UserAgent())),
	}); err != nil {
		s.logger.Error("archive flag: insert audit log", zap.Error(err))
	}

	w.WriteHeader(http.StatusNoContent)
}

func validateCreateFlag(req *createFlagRequest) error {
	req.Key = strings.TrimSpace(req.Key)
	req.Name = strings.TrimSpace(req.Name)
	if req.Description != nil {
		trimmed := strings.TrimSpace(*req.Description)
		req.Description = &trimmed
	}
	req.Type = strings.TrimSpace(req.Type)

	if req.Key == "" {
		return fmt.Errorf("key is required")
	}
	if len(req.Key) > 128 {
		return fmt.Errorf("key must not exceed 128 characters")
	}
	if !flagKeyRegex.MatchString(req.Key) {
		return fmt.Errorf("key must match: %s", flagKeyPattern)
	}

	if req.Name == "" {
		return fmt.Errorf("name is required")
	}
	if len(req.Name) > 255 {
		return fmt.Errorf("name must not exceed 255 characters")
	}

	if req.Description != nil && len(*req.Description) > 2000 {
		return fmt.Errorf("description must not exceed 2000 characters")
	}

	if !validFlagTypes[req.Type] {
		return fmt.Errorf("type must be one of: boolean, string, number, json")
	}

	if len(req.DefaultValue) > 0 {
		if !json.Valid(req.DefaultValue) {
			return fmt.Errorf("default_value must be valid JSON")
		}
	} else {
		switch req.Type {
		case "boolean":
			req.DefaultValue = json.RawMessage(`false`)
		case "number":
			req.DefaultValue = json.RawMessage(`0`)
		case "string":
			req.DefaultValue = json.RawMessage(`""`)
		default:
			req.DefaultValue = json.RawMessage(`null`)
		}
	}

	return nil
}
