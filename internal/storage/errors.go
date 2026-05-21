package storage

import "errors"

// Sentinel errors returned by storage operations.
var (
	ErrNotFound            = errors.New("storage: not found")
	ErrAlreadyInitialized  = errors.New("storage: already initialized")
	ErrVersionConflict     = errors.New("storage: version conflict")
	ErrDuplicateKey        = errors.New("storage: duplicate key")
	ErrTenantNotFound      = errors.New("storage: tenant not found")
	ErrUserNotFound        = errors.New("storage: user not found")
	ErrProjectNotFound     = errors.New("storage: project not found")
	ErrEnvironmentNotFound = errors.New("storage: environment not found")
	ErrFlagNotFound        = errors.New("storage: flag not found")
	ErrFlagArchived        = errors.New("storage: flag archived")
	ErrSegmentNotFound     = errors.New("storage: segment not found")
)
