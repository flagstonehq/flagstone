package api

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/flagstonehq/flagstone/internal/api/middleware"
	"github.com/flagstonehq/flagstone/internal/storage"
	"go.uber.org/zap"
)

type auditEntryResponse struct {
	ID           uuid.UUID        `json:"id"`
	ActorID      *uuid.UUID       `json:"actor_id,omitempty"`
	ActorType    string           `json:"actor_type"`
	Action       string           `json:"action"`
	ResourceType string           `json:"resource_type"`
	ResourceID   *uuid.UUID       `json:"resource_id,omitempty"`
	Changes      *json.RawMessage `json:"changes,omitempty"`
	IPAddress    *string          `json:"ip_address,omitempty"`
	UserAgent    *string          `json:"user_agent,omitempty"`
	CreatedAt    string           `json:"created_at"`
}

type auditLogResponse struct {
	Entries []auditEntryResponse `json:"entries"`
	Total   int64                `json:"total"`
	Limit   int                  `json:"limit"`
	Offset  int                  `json:"offset"`
}

func (s *Server) handleAuditLog(w http.ResponseWriter, r *http.Request) {
	claims := middleware.ClaimsFromContext(r.Context())
	if claims == nil {
		middleware.Error(w, r, http.StatusUnauthorized, "INVALID_CREDENTIALS", "Authentication is required.")
		return
	}

	tenantID, err := claims.TenantUUID()
	if err != nil {
		middleware.Error(w, r, http.StatusUnauthorized, "INVALID_CREDENTIALS", "Invalid token claims.")
		return
	}
	filter := storage.AuditLogFilter{TenantID: tenantID}

	q := r.URL.Query()

	if v := trimQuery(q, "actor_id"); v != "" {
		id, err := uuid.Parse(v)
		if err != nil {
			middleware.Error(w, r, http.StatusBadRequest, "VALIDATION_ERROR", "actor_id must be a valid UUID.")
			return
		}
		filter.ActorID = &id
	}

	if v := trimQuery(q, "actor_type"); v != "" {
		filter.ActorType = &v
	}

	if v := trimQuery(q, "action"); v != "" {
		filter.Action = &v
	}

	if v := trimQuery(q, "resource_type"); v != "" {
		filter.ResourceType = &v
	}

	if v := trimQuery(q, "resource_id"); v != "" {
		id, err := uuid.Parse(v)
		if err != nil {
			middleware.Error(w, r, http.StatusBadRequest, "VALIDATION_ERROR", "resource_id must be a valid UUID.")
			return
		}
		filter.ResourceID = &id
	}

	if v := trimQuery(q, "since"); v != "" {
		t, err := time.Parse(time.RFC3339, v)
		if err != nil {
			middleware.Error(w, r, http.StatusBadRequest, "VALIDATION_ERROR", "since must be in RFC3339 format.")
			return
		}
		filter.Since = &t
	}

	if v := trimQuery(q, "until"); v != "" {
		t, err := time.Parse(time.RFC3339, v)
		if err != nil {
			middleware.Error(w, r, http.StatusBadRequest, "VALIDATION_ERROR", "until must be in RFC3339 format.")
			return
		}
		filter.Until = &t
	}

	filter.Limit = queryInt(q, "limit", 20)
	if filter.Limit < 1 {
		filter.Limit = 1
	}
	if filter.Limit > 100 {
		filter.Limit = 100
	}

	filter.Offset = queryInt(q, "offset", 0)
	if filter.Offset < 0 {
		filter.Offset = 0
	}

	page, err := s.stores.AuditLogs.Query(r.Context(), filter)
	if err != nil {
		s.logger.Error("audit: query", zap.Error(err))
		middleware.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "An internal server error occurred.")
		return
	}

	resp := auditLogResponse{
		Entries: make([]auditEntryResponse, 0, len(page.Entries)),
		Total:   page.Total,
		Limit:   page.Limit,
		Offset:  page.Offset,
	}
	for _, e := range page.Entries {
		resp.Entries = append(resp.Entries, auditEntryResponse{
			ID:           e.ID,
			ActorID:      e.ActorID,
			ActorType:    e.ActorType,
			Action:       e.Action,
			ResourceType: e.ResourceType,
			ResourceID:   e.ResourceID,
			Changes:      e.Changes,
			IPAddress:    ipPtrToString(e.IPAddress),
			UserAgent:    e.UserAgent,
			CreatedAt:    e.CreatedAt.UTC().Format(timeFormat),
		})
	}

	middleware.JSON(w, http.StatusOK, resp)
}

func trimQuery(q url.Values, key string) string {
	return strings.TrimSpace(q.Get(key))
}

func queryInt(q url.Values, key string, defaultVal int) int {
	v := strings.TrimSpace(q.Get(key))
	if v == "" {
		return defaultVal
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return defaultVal
	}
	return n
}

func ipPtrToString(ip any) *string {
	if ip == nil {
		return nil
	}
	switch v := ip.(type) {
	case *net.IP:
		if v == nil {
			return nil
		}
		s := v.String()
		return &s
	case fmt.Stringer:
		s := v.String()
		return &s
	default:
		return nil
	}
}

func (s *Server) writeAudit(r *http.Request, tenantID, userID uuid.UUID, action, resourceType string, resourceID *uuid.UUID) {
	entry := &storage.AuditLogEntry{
		TenantID:     tenantID,
		ActorID:      uuidPtr(userID),
		ActorType:    "user",
		Action:       action,
		ResourceType: resourceType,
		ResourceID:   resourceID,
		IPAddress:    requestIP(r),
		UserAgent:    stringPtr(strings.TrimSpace(r.UserAgent())),
	}
	if err := s.stores.AuditLogs.Insert(r.Context(), entry); err != nil {
		s.logger.Error("audit: insert", zap.String("action", action), zap.Error(err))
	}
}
