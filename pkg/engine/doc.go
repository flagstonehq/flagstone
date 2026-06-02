// Package engine is a pure rule evaluator for Flagstone. It computes flag
// values from a snapshot of flags, segments and a per-request context, with
// no I/O and no dependency on storage, HTTP or auth. The same package is
// reused by the server (internal/api) and the Go SDK (pkg/sdk).
//
// Stable API: breaking changes bump the major version of the module.
package engine
