package auth

import "strings"

// Role represents a user's role within a tenant.
type Role string

// Standard tenant roles ordered by privilege level (highest to lowest).
const (
	RoleOwner  Role = "owner"
	RoleAdmin  Role = "admin"
	RoleMember Role = "member"
	RoleViewer Role = "viewer"
)

// ParseRole normalizes and parses a raw role string.
func ParseRole(raw string) Role {
	return Role(strings.ToLower(strings.TrimSpace(raw)))
}

// String returns the string representation of the role.
func (r Role) String() string {
	return string(r)
}

// Valid returns true if the role is a known role.
func (r Role) Valid() bool {
	switch r {
	case RoleOwner, RoleAdmin, RoleMember, RoleViewer:
		return true
	default:
		return false
	}
}

// Level returns the numeric level of the role.
func (r Role) Level() int {
	switch r {
	case RoleOwner:
		return 4
	case RoleAdmin:
		return 3
	case RoleMember:
		return 2
	case RoleViewer:
		return 1
	default:
		return 0
	}
}

// AtLeast returns true if the role level is at least the minimum role level.
func (r Role) AtLeast(minimum Role) bool {
	return r.Level() >= minimum.Level()
}
