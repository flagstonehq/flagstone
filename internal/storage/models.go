package storage

import (
	"encoding/json"
	"net"
	"time"

	"github.com/google/uuid"
)

// Tenant represents an organization using Flagstone.
type Tenant struct {
	ID        uuid.UUID
	Slug      string
	Name      string
	Plan      string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// User represents a human account that can log into the dashboard.
type User struct {
	ID              uuid.UUID
	Email           string
	PasswordHash    *string
	EmailVerifiedAt *time.Time
	LastLoginAt     *time.Time
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

// TenantMember links a user to a tenant with a specific role.
type TenantMember struct {
	TenantID  uuid.UUID
	UserID    uuid.UUID
	Role      string
	CreatedAt time.Time
}

// Session stores a JWT refresh token hash for dashboard auth.
type Session struct {
	ID          uuid.UUID
	UserID      uuid.UUID
	TenantID    uuid.UUID
	RefreshHash string
	UserAgent   *string
	IPAddress   *net.IP
	ExpiresAt   time.Time
	CreatedAt   time.Time
}

// Project groups flags within a tenant.
type Project struct {
	ID        uuid.UUID
	TenantID  uuid.UUID
	Slug      string
	Name      string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// Environment represents a deployment target (dev, staging, prod) within a project.
type Environment struct {
	ID        uuid.UUID
	ProjectID uuid.UUID
	Slug      string
	Name      string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// APIKey represents machine credentials scoped to a single environment.
type APIKey struct {
	ID            uuid.UUID
	EnvironmentID uuid.UUID
	Name          string
	KeyHash       string
	KeyPrefix     string
	LastUsedAt    *time.Time
	ExpiresAt     *time.Time
	RevokedAt     *time.Time
	CreatedAt     time.Time
	CreatedBy     *uuid.UUID
}

// Flag is a feature flag definition at the project level.
type Flag struct {
	ID           uuid.UUID
	ProjectID    uuid.UUID
	Key          string
	Name         string
	Description  *string
	Type         string
	DefaultValue json.RawMessage
	ArchivedAt   *time.Time
	CreatedAt    time.Time
	UpdatedAt    time.Time
	CreatedBy    *uuid.UUID
}

// FlagEnvironment holds per-environment configuration for a flag.
type FlagEnvironment struct {
	FlagID        uuid.UUID
	EnvironmentID uuid.UUID
	Enabled       bool
	Rules         json.RawMessage
	DefaultValue  *json.RawMessage
	Version       int64
	CreatedAt     time.Time
	UpdatedAt     time.Time
	UpdatedBy     *uuid.UUID
}

// Segment is a reusable named predicate over user attributes.
type Segment struct {
	ID          uuid.UUID
	ProjectID   uuid.UUID
	Key         string
	Name        string
	Description *string
	Rules       json.RawMessage
	ArchivedAt  *time.Time
	CreatedAt   time.Time
	UpdatedAt   time.Time
	CreatedBy   *uuid.UUID
}

// LoginAttempt records a single failed login. The row is consulted to
// enforce account lockout (T19) and wiped when the user logs in successfully.
type LoginAttempt struct {
	ID          uuid.UUID
	UserID      uuid.UUID
	IPAddress   *net.IP
	UserAgent   *string
	AttemptedAt time.Time
}

// RevokedRefreshToken records a refresh-token hash that must never be
// honored again. Used to detect replayed refresh tokens (T20).
type RevokedRefreshToken struct {
	ID        uuid.UUID
	TokenHash string
	UserID    uuid.UUID
	TenantID  uuid.UUID
	RevokedAt time.Time
	ExpiresAt time.Time
}

// AuditLogEntry records an immutable system action for forensic auditing.
type AuditLogEntry struct {
	ID           uuid.UUID
	TenantID     uuid.UUID
	ActorID      *uuid.UUID
	ActorType    string
	Action       string
	ResourceType string
	ResourceID   *uuid.UUID
	Changes      *json.RawMessage
	IPAddress    *net.IP
	UserAgent    *string
	CreatedAt    time.Time
}
