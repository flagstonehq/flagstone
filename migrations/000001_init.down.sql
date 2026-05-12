-- =============================================================================
-- Flagstone — Rollback initial schema
-- =============================================================================
-- Drop in reverse order of creation to respect foreign keys.
-- =============================================================================

BEGIN;

DROP TRIGGER IF EXISTS flag_environments_auto_version ON flag_environments;
DROP TRIGGER IF EXISTS audit_log_immutable ON audit_log;
DROP FUNCTION IF EXISTS increment_flag_version();
DROP FUNCTION IF EXISTS prevent_audit_mutation();

DROP TABLE IF EXISTS audit_log;
DROP TABLE IF EXISTS flag_environments;
DROP TABLE IF EXISTS flags;
DROP TABLE IF EXISTS segments;
DROP TABLE IF EXISTS api_keys;
DROP TABLE IF EXISTS environments;
DROP TABLE IF EXISTS projects;
DROP TABLE IF EXISTS tenant_members;
DROP TABLE IF EXISTS users;
DROP TABLE IF EXISTS tenants;

DROP FUNCTION IF EXISTS set_updated_at();

-- We deliberately do NOT drop citext extension — it may be used by other
-- databases on the same instance.

COMMIT;
