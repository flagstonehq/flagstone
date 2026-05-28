package api

import (
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/thomas-vilte/flagstone/internal/storage"
	"go.uber.org/zap"
)

// writeAudit inserts an audit log entry for a successful mutation. Failures
// are logged but do not bubble up — the action already succeeded, the audit
// row is bookkeeping.
//
// resourceID may be nil for actions where the resource isn't yet (or isn't
// only) identified by a single row.
func (s *Server) writeAudit(r *http.Request, tenantID, userID uuid.UUID, action, resourceType string, resourceID *uuid.UUID) {
	if err := s.stores.AuditLogs.Insert(r.Context(), &storage.AuditLogEntry{
		TenantID:     tenantID,
		ActorID:      uuidPtr(userID),
		ActorType:    "user",
		Action:       action,
		ResourceType: resourceType,
		ResourceID:   resourceID,
		IPAddress:    requestIP(r),
		UserAgent:    stringPtr(strings.TrimSpace(r.UserAgent())),
	}); err != nil {
		s.logger.Error("audit log: "+action, zap.Error(err))
	}
}
