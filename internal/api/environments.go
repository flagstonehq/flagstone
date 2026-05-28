package api

import (
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

type createEnvironmentRequest struct {
	Slug string `json:"slug"`
	Name string `json:"name"`
}

type environmentResponse struct {
	ID        uuid.UUID `json:"id"`
	ProjectID uuid.UUID `json:"project_id"`
	Slug      string    `json:"slug"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (s *Server) handleCreateEnvironment(w http.ResponseWriter, r *http.Request) {
	var req createEnvironmentRequest
	if err := DecodeJSON(r, &req); err != nil {
		s.writeDecodeError(w, r, err)
		return
	}

	if err := validateCreateEnvironment(&req); err != nil {
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
		s.logger.Error("create environment: get project", zap.Error(err))
		middleware.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "An internal server error occurred.")
		return
	}

	var resp environmentResponse
	if err := s.runInTx(r.Context(), func(txStores *storage.Stores) error {
		env := &storage.Environment{
			ProjectID: project.ID,
			Slug:      req.Slug,
			Name:      req.Name,
		}
		if err := txStores.Environments.Create(r.Context(), env); err != nil {
			return err
		}

		if err := txStores.AuditLogs.Insert(r.Context(), &storage.AuditLogEntry{
			TenantID:     tenantID,
			ActorID:      uuidPtr(userID),
			ActorType:    "user",
			Action:       "environment.created",
			ResourceType: "environment",
			ResourceID:   uuidPtr(env.ID),
			IPAddress:    requestIP(r),
			UserAgent:    stringPtr(strings.TrimSpace(r.UserAgent())),
		}); err != nil {
			return fmt.Errorf("insert audit log: %w", err)
		}

		resp = environmentResponse{
			ID:        env.ID,
			ProjectID: env.ProjectID,
			Slug:      env.Slug,
			Name:      env.Name,
			CreatedAt: env.CreatedAt,
			UpdatedAt: env.UpdatedAt,
		}
		return nil
	}); err != nil {
		if errors.Is(err, storage.ErrDuplicateKey) {
			middleware.Error(w, r, http.StatusConflict, "DUPLICATE_SLUG", "An environment with this slug already exists in this project.")
			return
		}
		s.logger.Error("create environment", zap.Error(err))
		middleware.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "An internal server error occurred.")
		return
	}

	middleware.JSON(w, http.StatusCreated, resp)
}

func (s *Server) handleListEnvironments(w http.ResponseWriter, r *http.Request) {
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
		s.logger.Error("list environments: get project", zap.Error(err))
		middleware.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "An internal server error occurred.")
		return
	}

	environments, err := s.stores.Environments.ListByProject(r.Context(), project.ID)
	if err != nil {
		s.logger.Error("list environments", zap.Error(err))
		middleware.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "An internal server error occurred.")
		return
	}

	resp := make([]environmentResponse, 0, len(environments))
	for _, env := range environments {
		resp = append(resp, environmentResponse{
			ID:        env.ID,
			ProjectID: env.ProjectID,
			Slug:      env.Slug,
			Name:      env.Name,
			CreatedAt: env.CreatedAt,
			UpdatedAt: env.UpdatedAt,
		})
	}

	middleware.JSON(w, http.StatusOK, resp)
}

// handleDeleteEnvironment hard-deletes an environment. Cascades drop all
// flag_environments and api_keys for it. Restricted to Owner because the
// blast radius — every API key in this env stops working, every flag's
// per-env config evaporates — is wider than a normal admin operation.
func (s *Server) handleDeleteEnvironment(w http.ResponseWriter, r *http.Request) {
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
		s.logger.Error("delete environment: get project", zap.Error(err))
		middleware.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "An internal server error occurred.")
		return
	}

	env, err := s.stores.Environments.GetBySlug(r.Context(), project.ID, envSlug)
	if err != nil {
		if errors.Is(err, storage.ErrEnvironmentNotFound) {
			middleware.Error(w, r, http.StatusNotFound, "NOT_FOUND", "Environment not found.")
			return
		}
		s.logger.Error("delete environment: get env", zap.Error(err))
		middleware.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "An internal server error occurred.")
		return
	}

	if err := s.stores.Environments.DeleteByID(r.Context(), env.ID); err != nil {
		if errors.Is(err, storage.ErrEnvironmentNotFound) {
			middleware.Error(w, r, http.StatusNotFound, "NOT_FOUND", "Environment not found.")
			return
		}
		s.logger.Error("delete environment", zap.Error(err))
		middleware.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "An internal server error occurred.")
		return
	}

	s.writeAudit(r, tenantID, userID, "environment.deleted", "environment", uuidPtr(env.ID))

	w.WriteHeader(http.StatusNoContent)
}

func validateCreateEnvironment(req *createEnvironmentRequest) error {
	req.Slug = strings.TrimSpace(req.Slug)
	req.Name = strings.TrimSpace(req.Name)

	if req.Slug == "" {
		return fmt.Errorf("slug is required")
	}
	if len(req.Slug) > 64 {
		return fmt.Errorf("slug must not exceed 64 characters")
	}
	if !slugRegex.MatchString(req.Slug) {
		return fmt.Errorf("slug must match: %s", slugPattern)
	}

	if req.Name == "" {
		return fmt.Errorf("name is required")
	}
	if len(req.Name) > 255 {
		return fmt.Errorf("name must not exceed 255 characters")
	}

	return nil
}
