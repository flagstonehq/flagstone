package storage

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// FlagEnvironmentConfig is the flattened view returned by the bulk environment query.
type FlagEnvironmentConfig struct {
	FlagID                  uuid.UUID
	EnvironmentID           uuid.UUID
	FlagKey                 string
	FlagName                string
	FlagType                string
	FlagDefaultValue        json.RawMessage
	Enabled                 bool
	Rules                   json.RawMessage
	EnvironmentDefaultValue *json.RawMessage
	Version                 int64
	UpdatedAt               time.Time
	UpdatedBy               *uuid.UUID
}

// AuditLogFilter defines the supported filters for querying tenant audit logs.
type AuditLogFilter struct {
	TenantID     uuid.UUID
	ActorID      *uuid.UUID
	ActorType    *string
	Action       *string
	ResourceType *string
	ResourceID   *uuid.UUID
	Since        *time.Time
	Until        *time.Time
	Limit        int
	Offset       int
}

// AuditLogPage is a paginated audit log result set.
type AuditLogPage struct {
	Entries []AuditLogEntry
	Total   int64
	Limit   int
	Offset  int
}

func rawMessagePtrFromBytes(v []byte) *json.RawMessage {
	if v == nil {
		return nil
	}

	raw := json.RawMessage(append([]byte(nil), v...))
	return &raw
}
