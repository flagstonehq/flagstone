package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/flagstonehq/flagstone/internal/api/middleware"
	"github.com/flagstonehq/flagstone/internal/storage"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

type createSegmentRequest struct {
	Key         string          `json:"key"`
	Name        string          `json:"name"`
	Description *string         `json:"description,omitempty"`
	Rules       json.RawMessage `json:"rules"`
}

type updateSegmentRequest struct {
	Key         *string          `json:"key,omitempty"`
	Name        *string          `json:"name,omitempty"`
	Description *string          `json:"description,omitempty"`
	Rules       *json.RawMessage `json:"rules,omitempty"`
}

type segmentResponse struct {
	ID          uuid.UUID       `json:"id"`
	ProjectID   uuid.UUID       `json:"project_id"`
	Key         string          `json:"key"`
	Name        string          `json:"name"`
	Description *string         `json:"description,omitempty"`
	Rules       json.RawMessage `json:"rules"`
	ArchivedAt  *time.Time      `json:"archived_at,omitempty"`
	CreatedAt   time.Time       `json:"created_at"`
	UpdatedAt   time.Time       `json:"updated_at"`
	CreatedBy   *uuid.UUID      `json:"created_by,omitempty"`
}

func segmentResponseFromSegment(s *storage.Segment) segmentResponse {
	return segmentResponse{
		ID:          s.ID,
		ProjectID:   s.ProjectID,
		Key:         s.Key,
		Name:        s.Name,
		Description: s.Description,
		Rules:       s.Rules,
		ArchivedAt:  s.ArchivedAt,
		CreatedAt:   s.CreatedAt,
		UpdatedAt:   s.UpdatedAt,
		CreatedBy:   s.CreatedBy,
	}
}

func validateCreateSegment(req *createSegmentRequest) error {
	req.Key = strings.TrimSpace(req.Key)
	req.Name = strings.TrimSpace(req.Name)
	if req.Description != nil {
		trimmed := strings.TrimSpace(*req.Description)
		req.Description = &trimmed
	}

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

	if len(req.Rules) > 0 {
		if err := ValidateSegmentRules(req.Rules); err != nil {
			return err
		}
	}

	return nil
}

func (s *Server) handleCreateSegment(w http.ResponseWriter, r *http.Request) {
	var req createSegmentRequest
	if err := DecodeJSON(r, &req); err != nil {
		s.writeDecodeError(w, r, err)
		return
	}

	if err := validateCreateSegment(&req); err != nil {
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
		s.logger.Error("create segment: get project", zap.Error(err))
		middleware.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "An internal server error occurred.")
		return
	}

	segment := &storage.Segment{
		ProjectID:   project.ID,
		Key:         req.Key,
		Name:        req.Name,
		Description: req.Description,
		Rules:       req.Rules,
		CreatedBy:   uuidPtr(userID),
	}

	if err := s.stores.Segments.Create(r.Context(), segment); err != nil {
		if errors.Is(err, storage.ErrDuplicateKey) {
			middleware.Error(w, r, http.StatusConflict, "DUPLICATE_KEY", "A segment with this key already exists in this project.")
			return
		}
		s.logger.Error("create segment", zap.Error(err))
		middleware.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "An internal server error occurred.")
		return
	}

	s.writeAudit(r, tenantID, userID, "segment.created", "segment", uuidPtr(segment.ID))

	middleware.JSON(w, http.StatusCreated, segmentResponseFromSegment(segment))
}

func (s *Server) handleListSegments(w http.ResponseWriter, r *http.Request) {
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
		s.logger.Error("list segments: get project", zap.Error(err))
		middleware.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "An internal server error occurred.")
		return
	}

	segments, err := s.stores.Segments.ListByProject(r.Context(), project.ID)
	if err != nil {
		s.logger.Error("list segments", zap.Error(err))
		middleware.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "An internal server error occurred.")
		return
	}

	resp := make([]segmentResponse, 0, len(segments))
	for _, seg := range segments {
		resp = append(resp, segmentResponseFromSegment(&seg))
	}

	middleware.JSON(w, http.StatusOK, resp)
}

func (s *Server) handleGetSegment(w http.ResponseWriter, r *http.Request) {
	claims := middleware.ClaimsFromContext(r.Context())
	tenantID, err := claims.TenantUUID()
	if err != nil {
		middleware.Error(w, r, http.StatusUnauthorized, "INVALID_CREDENTIALS", "Invalid token claims.")
		return
	}

	projectSlug := r.PathValue("slug")
	segmentKey := r.PathValue("key")

	project, err := s.stores.Projects.GetBySlug(r.Context(), tenantID, projectSlug)
	if err != nil {
		if errors.Is(err, storage.ErrProjectNotFound) {
			middleware.Error(w, r, http.StatusNotFound, "NOT_FOUND", "Project not found.")
			return
		}
		s.logger.Error("get segment: get project", zap.Error(err))
		middleware.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "An internal server error occurred.")
		return
	}

	segment, err := s.stores.Segments.GetByKey(r.Context(), project.ID, segmentKey)
	if err != nil {
		if errors.Is(err, storage.ErrSegmentNotFound) {
			middleware.Error(w, r, http.StatusNotFound, "NOT_FOUND", "Segment not found.")
			return
		}
		s.logger.Error("get segment", zap.Error(err))
		middleware.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "An internal server error occurred.")
		return
	}

	middleware.JSON(w, http.StatusOK, segmentResponseFromSegment(segment))
}

func (s *Server) handleUpdateSegment(w http.ResponseWriter, r *http.Request) {
	var req updateSegmentRequest
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
	segmentKey := r.PathValue("key")

	project, err := s.stores.Projects.GetBySlug(r.Context(), tenantID, projectSlug)
	if err != nil {
		if errors.Is(err, storage.ErrProjectNotFound) {
			middleware.Error(w, r, http.StatusNotFound, "NOT_FOUND", "Project not found.")
			return
		}
		s.logger.Error("update segment: get project", zap.Error(err))
		middleware.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "An internal server error occurred.")
		return
	}

	segment, err := s.stores.Segments.GetByKey(r.Context(), project.ID, segmentKey)
	if err != nil {
		if errors.Is(err, storage.ErrSegmentNotFound) {
			middleware.Error(w, r, http.StatusNotFound, "NOT_FOUND", "Segment not found.")
			return
		}
		s.logger.Error("update segment: get segment", zap.Error(err))
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
		segment.Key = trimmed
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
		segment.Name = trimmed
	}
	if req.Description != nil {
		trimmed := strings.TrimSpace(*req.Description)
		if len(trimmed) > 2000 {
			middleware.Error(w, r, http.StatusBadRequest, "VALIDATION_ERROR", "description must not exceed 2000 characters.")
			return
		}
		if trimmed == "" {
			segment.Description = nil
		} else {
			segment.Description = &trimmed
		}
	}
	if req.Rules != nil {
		if len(*req.Rules) > 0 {
			if err := ValidateSegmentRules(*req.Rules); err != nil {
				middleware.Error(w, r, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
				return
			}
		}
		segment.Rules = *req.Rules
	}

	if err := s.stores.Segments.Update(r.Context(), segment); err != nil {
		if errors.Is(err, storage.ErrDuplicateKey) {
			middleware.Error(w, r, http.StatusConflict, "DUPLICATE_KEY", "A segment with this key already exists in this project.")
			return
		}
		if errors.Is(err, storage.ErrSegmentNotFound) {
			middleware.Error(w, r, http.StatusNotFound, "NOT_FOUND", "Segment not found.")
			return
		}
		s.logger.Error("update segment", zap.Error(err))
		middleware.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "An internal server error occurred.")
		return
	}

	s.writeAudit(r, tenantID, userID, "segment.updated", "segment", uuidPtr(segment.ID))

	middleware.JSON(w, http.StatusOK, segmentResponseFromSegment(segment))
}

func (s *Server) handleArchiveSegment(w http.ResponseWriter, r *http.Request) {
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
	segmentKey := r.PathValue("key")

	project, err := s.stores.Projects.GetBySlug(r.Context(), tenantID, projectSlug)
	if err != nil {
		if errors.Is(err, storage.ErrProjectNotFound) {
			middleware.Error(w, r, http.StatusNotFound, "NOT_FOUND", "Project not found.")
			return
		}
		s.logger.Error("archive segment: get project", zap.Error(err))
		middleware.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "An internal server error occurred.")
		return
	}

	segment, err := s.stores.Segments.GetByKey(r.Context(), project.ID, segmentKey)
	if err != nil {
		if errors.Is(err, storage.ErrSegmentNotFound) {
			middleware.Error(w, r, http.StatusNotFound, "NOT_FOUND", "Segment not found.")
			return
		}
		s.logger.Error("archive segment: get segment", zap.Error(err))
		middleware.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "An internal server error occurred.")
		return
	}

	now := time.Now().UTC()
	if err := s.stores.Segments.Archive(r.Context(), segment.ID, now); err != nil {
		if errors.Is(err, storage.ErrSegmentNotFound) {
			middleware.Error(w, r, http.StatusNotFound, "NOT_FOUND", "Segment not found.")
			return
		}
		s.logger.Error("archive segment", zap.Error(err))
		middleware.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "An internal server error occurred.")
		return
	}

	s.writeAudit(r, tenantID, userID, "segment.archived", "segment", uuidPtr(segment.ID))

	w.WriteHeader(http.StatusNoContent)
}
