package api

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/flagstonehq/flagstone/internal/api/middleware"
	"github.com/flagstonehq/flagstone/internal/storage"
	"go.uber.org/zap"
)

type createProjectRequest struct {
	Slug string `json:"slug"`
	Name string `json:"name"`
}

type updateProjectRequest struct {
	Slug *string `json:"slug,omitempty"`
	Name *string `json:"name,omitempty"`
}

type projectResponse struct {
	ID        uuid.UUID `json:"id"`
	Slug      string    `json:"slug"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (s *Server) handleCreateProject(w http.ResponseWriter, r *http.Request) {
	var req createProjectRequest
	if err := DecodeJSON(r, &req); err != nil {
		s.writeDecodeError(w, r, err)
		return
	}

	if err := validateCreateProject(&req); err != nil {
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

	var resp projectResponse
	if err := s.runInTx(r.Context(), func(txStores *storage.Stores) error {
		project := &storage.Project{
			ID:       uuid.New(),
			TenantID: tenantID,
			Slug:     req.Slug,
			Name:     req.Name,
		}
		if err := txStores.Projects.Create(r.Context(), project); err != nil {
			return err
		}

		for _, env := range []struct{ slug, name string }{
			{"development", "Development"},
			{"staging", "Staging"},
			{"production", "Production"},
		} {
			if err := txStores.Environments.Create(r.Context(), &storage.Environment{
				ProjectID: project.ID,
				Slug:      env.slug,
				Name:      env.name,
			}); err != nil {
				return fmt.Errorf("create environment %s: %w", env.slug, err)
			}
		}

		if err := txStores.AuditLogs.Insert(r.Context(), &storage.AuditLogEntry{
			TenantID:     tenantID,
			ActorID:      uuidPtr(userID),
			ActorType:    "user",
			Action:       "project.created",
			ResourceType: "project",
			ResourceID:   uuidPtr(project.ID),
			IPAddress:    requestIP(r),
			UserAgent:    stringPtr(strings.TrimSpace(r.UserAgent())),
		}); err != nil {
			return fmt.Errorf("insert audit log: %w", err)
		}

		resp = projectResponse{
			ID:        project.ID,
			Slug:      project.Slug,
			Name:      project.Name,
			CreatedAt: project.CreatedAt,
			UpdatedAt: project.UpdatedAt,
		}
		return nil
	}); err != nil {
		if errors.Is(err, storage.ErrDuplicateKey) {
			middleware.Error(w, r, http.StatusConflict, "DUPLICATE_SLUG", "A project with this slug already exists.")
			return
		}
		s.logger.Error("create project", zap.Error(err))
		middleware.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "An internal server error occurred.")
		return
	}

	middleware.JSON(w, http.StatusCreated, resp)
}

func (s *Server) handleListProjects(w http.ResponseWriter, r *http.Request) {
	claims := middleware.ClaimsFromContext(r.Context())
	tenantID, err := claims.TenantUUID()
	if err != nil {
		middleware.Error(w, r, http.StatusUnauthorized, "INVALID_CREDENTIALS", "Invalid token claims.")
		return
	}

	projects, err := s.stores.Projects.ListByTenant(r.Context(), tenantID)
	if err != nil {
		s.logger.Error("list projects", zap.Error(err))
		middleware.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "An internal server error occurred.")
		return
	}

	resp := make([]projectResponse, 0, len(projects))
	for _, p := range projects {
		resp = append(resp, projectResponse{
			ID:        p.ID,
			Slug:      p.Slug,
			Name:      p.Name,
			CreatedAt: p.CreatedAt,
			UpdatedAt: p.UpdatedAt,
		})
	}

	middleware.JSON(w, http.StatusOK, resp)
}

func (s *Server) handleGetProject(w http.ResponseWriter, r *http.Request) {
	claims := middleware.ClaimsFromContext(r.Context())
	tenantID, err := claims.TenantUUID()
	if err != nil {
		middleware.Error(w, r, http.StatusUnauthorized, "INVALID_CREDENTIALS", "Invalid token claims.")
		return
	}

	slug := r.PathValue("slug")
	project, err := s.stores.Projects.GetBySlug(r.Context(), tenantID, slug)
	if err != nil {
		if errors.Is(err, storage.ErrProjectNotFound) {
			middleware.Error(w, r, http.StatusNotFound, "NOT_FOUND", "Project not found.")
			return
		}
		s.logger.Error("get project", zap.Error(err))
		middleware.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "An internal server error occurred.")
		return
	}

	middleware.JSON(w, http.StatusOK, projectResponse{
		ID:        project.ID,
		Slug:      project.Slug,
		Name:      project.Name,
		CreatedAt: project.CreatedAt,
		UpdatedAt: project.UpdatedAt,
	})
}

func (s *Server) handleUpdateProject(w http.ResponseWriter, r *http.Request) {
	var req updateProjectRequest
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

	slug := r.PathValue("slug")
	project, err := s.stores.Projects.GetBySlug(r.Context(), tenantID, slug)
	if err != nil {
		if errors.Is(err, storage.ErrProjectNotFound) {
			middleware.Error(w, r, http.StatusNotFound, "NOT_FOUND", "Project not found.")
			return
		}
		s.logger.Error("update project: get", zap.Error(err))
		middleware.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "An internal server error occurred.")
		return
	}

	if req.Slug != nil {
		trimmed := strings.TrimSpace(*req.Slug)
		if trimmed == "" {
			middleware.Error(w, r, http.StatusBadRequest, "VALIDATION_ERROR", "slug must not be empty.")
			return
		}
		if len(trimmed) > 64 {
			middleware.Error(w, r, http.StatusBadRequest, "VALIDATION_ERROR", "slug must not exceed 64 characters.")
			return
		}
		if !slugRegex.MatchString(trimmed) {
			middleware.Error(w, r, http.StatusBadRequest, "VALIDATION_ERROR", "slug must match: "+slugPattern)
			return
		}
		project.Slug = trimmed
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
		project.Name = trimmed
	}

	if err := s.stores.Projects.Update(r.Context(), project); err != nil {
		if errors.Is(err, storage.ErrDuplicateKey) {
			middleware.Error(w, r, http.StatusConflict, "DUPLICATE_SLUG", "A project with this slug already exists.")
			return
		}
		if errors.Is(err, storage.ErrProjectNotFound) {
			middleware.Error(w, r, http.StatusNotFound, "NOT_FOUND", "Project not found.")
			return
		}
		s.logger.Error("update project", zap.Error(err))
		middleware.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "An internal server error occurred.")
		return
	}

	s.writeAudit(r, tenantID, userID, "project.updated", "project", uuidPtr(project.ID))

	middleware.JSON(w, http.StatusOK, projectResponse{
		ID:        project.ID,
		Slug:      project.Slug,
		Name:      project.Name,
		CreatedAt: project.CreatedAt,
		UpdatedAt: project.UpdatedAt,
	})
}

func validateCreateProject(req *createProjectRequest) error {
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
