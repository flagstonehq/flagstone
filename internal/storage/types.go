package storage

// Strong type aliases for domain identifiers.
// Prevents accidentally swapping arguments like flagKey and environmentID.
type (
	// FlagKey is the programmatic identifier of a flag.
	FlagKey string
	// EnvironmentID identifies a deployment environment.
	EnvironmentID string
	// TenantID identifies an organization.
	TenantID string
	// ProjectID identifies a project within a tenant.
	ProjectID string
	// SegmentKey is the programmatic identifier of a segment.
	SegmentKey string
)
