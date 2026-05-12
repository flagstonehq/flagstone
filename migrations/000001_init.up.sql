-- =============================================================================
-- Flagstone — Initial schema
-- =============================================================================
-- This migration creates the full set of tables needed for a multi-tenant
-- feature flag system. Design rationale lives in DESIGN.md at the repo root.
--
-- Conventions:
--   * UUIDs for all public-facing primary keys (no enumeration attacks).
--   * TIMESTAMPTZ everywhere — never store naive timestamps.
--   * Soft-delete via *_at columns (archived_at / revoked_at) — never hard delete.
--   * snake_case for tables and columns.
--   * Foreign keys with explicit ON DELETE behavior (CASCADE for ownership,
--     SET NULL for soft references like "created_by").
-- =============================================================================

BEGIN;

-- -----------------------------------------------------------------------------
-- Extensions
-- -----------------------------------------------------------------------------
-- citext:   case-insensitive text (used for emails — avoids LOWER() on every query)
-- Note: gen_random_uuid() is built-in since PostgreSQL 13 — no need for pgcrypto.
CREATE EXTENSION IF NOT EXISTS citext;


-- -----------------------------------------------------------------------------
-- Shared trigger: keep updated_at in sync on UPDATE
-- -----------------------------------------------------------------------------
CREATE OR REPLACE FUNCTION set_updated_at() RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;


-- =============================================================================
-- Tenancy & identity
-- =============================================================================

-- A tenant is an organization paying for / hosting Flagstone.
-- Even in single-tenant deploys we keep this table because it makes the
-- multi-tenant migration trivial when a real customer shows up.
CREATE TABLE tenants (
    id         UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    slug       VARCHAR(64)  NOT NULL UNIQUE,    -- URL-safe identifier
    name       VARCHAR(255) NOT NULL,
    plan       VARCHAR(32)  NOT NULL DEFAULT 'free',
    created_at TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ  NOT NULL DEFAULT NOW(),

    CONSTRAINT tenants_plan_valid CHECK (plan IN ('free', 'team', 'enterprise')),
    CONSTRAINT tenants_slug_format CHECK (slug ~ '^[a-z0-9]([a-z0-9-]*[a-z0-9])?$')
);
CREATE TRIGGER tenants_set_updated_at
    BEFORE UPDATE ON tenants
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();


-- Humans who can log into the dashboard. Distinct from API keys (machines).
CREATE TABLE users (
    id                UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    email             CITEXT       NOT NULL UNIQUE,
    password_hash     VARCHAR(255),                  -- nullable for SSO/OAuth users
    email_verified_at TIMESTAMPTZ,
    last_login_at     TIMESTAMPTZ,
    created_at        TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at        TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);
CREATE TRIGGER users_set_updated_at
    BEFORE UPDATE ON users
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();


-- Many-to-many between users and tenants, with a role per membership.
-- A single user can belong to multiple tenants (consultant, agency dev, etc).
CREATE TABLE tenant_members (
    tenant_id  UUID         NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    user_id    UUID         NOT NULL REFERENCES users(id)   ON DELETE CASCADE,
    role       VARCHAR(32)  NOT NULL DEFAULT 'member',
    created_at TIMESTAMPTZ  NOT NULL DEFAULT NOW(),

    PRIMARY KEY (tenant_id, user_id),
    CONSTRAINT tenant_members_role_valid
        CHECK (role IN ('owner', 'admin', 'member', 'viewer'))
);
CREATE INDEX tenant_members_user_idx ON tenant_members(user_id);


-- =============================================================================
-- Project & environment hierarchy
-- =============================================================================
-- A tenant can have many projects (e.g. "mobile-app", "web-dashboard").
-- Each project has its own environments (dev, staging, prod). This mirrors
-- how teams actually organize their software and avoids accidental cross-env
-- flag changes.
-- =============================================================================

CREATE TABLE projects (
    id         UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id  UUID         NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    slug       VARCHAR(64)  NOT NULL,
    name       VARCHAR(255) NOT NULL,
    created_at TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ  NOT NULL DEFAULT NOW(),

    UNIQUE (tenant_id, slug),
    CONSTRAINT projects_slug_format CHECK (slug ~ '^[a-z0-9]([a-z0-9-]*[a-z0-9])?$')
);
CREATE TRIGGER projects_set_updated_at
    BEFORE UPDATE ON projects
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();


CREATE TABLE environments (
    id         UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id UUID         NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    slug       VARCHAR(64)  NOT NULL,
    name       VARCHAR(255) NOT NULL,
    created_at TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ  NOT NULL DEFAULT NOW(),

    UNIQUE (project_id, slug),
    CONSTRAINT environments_slug_format CHECK (slug ~ '^[a-z0-9]([a-z0-9-]*[a-z0-9])?$')
);
CREATE TRIGGER environments_set_updated_at
    BEFORE UPDATE ON environments
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();


-- =============================================================================
-- API keys (machine credentials for SDKs)
-- =============================================================================
-- Keys are scoped to ONE environment. If the prod key leaks, dev/staging
-- configs are still safe.
--
-- We never store the raw key:
--   * key_hash:   sha256 of the full key, used for O(1) lookup
--   * key_prefix: first ~8 chars (e.g. "fs_live_a3b9"), shown in the UI so
--                 admins can identify which key is which without revealing it
-- =============================================================================

CREATE TABLE api_keys (
    id             UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    environment_id UUID         NOT NULL REFERENCES environments(id) ON DELETE CASCADE,
    name           VARCHAR(255) NOT NULL,
    key_hash       CHAR(64)     NOT NULL UNIQUE,
    key_prefix     VARCHAR(16)  NOT NULL,
    last_used_at   TIMESTAMPTZ,
    expires_at     TIMESTAMPTZ,
    revoked_at     TIMESTAMPTZ,
    created_at     TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    created_by     UUID         REFERENCES users(id) ON DELETE SET NULL
);

-- Partial index: most lookups filter active keys, so we exclude revoked ones
CREATE INDEX api_keys_active_idx
    ON api_keys(environment_id)
    WHERE revoked_at IS NULL;


-- =============================================================================
-- Segments (reusable user groups)
-- =============================================================================
-- A segment is a named predicate over user attributes, e.g. "premium users
-- in Argentina". Reusable across many flags so you don't repeat the rule.
--
-- The rules column is JSONB: a tree of conditions like
--   { "all": [
--       { "attribute": "country", "op": "eq", "value": "AR" },
--       { "attribute": "plan",    "op": "eq", "value": "premium" }
--   ]}
-- See DESIGN.md for why JSONB instead of normalized rule tables.
-- =============================================================================

CREATE TABLE segments (
    id          UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id  UUID         NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    key         VARCHAR(128) NOT NULL,                       -- programmatic id
    name        VARCHAR(255) NOT NULL,                       -- human label
    description TEXT,
    rules       JSONB        NOT NULL DEFAULT '[]'::jsonb,
    archived_at TIMESTAMPTZ,                                 -- soft delete
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    created_by  UUID         REFERENCES users(id) ON DELETE SET NULL,

    UNIQUE (project_id, key),
    CONSTRAINT segments_key_format CHECK (key ~ '^[a-z0-9][a-z0-9_-]*[a-z0-9]$')
);
CREATE TRIGGER segments_set_updated_at
    BEFORE UPDATE ON segments
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();


-- =============================================================================
-- Flags
-- =============================================================================
-- A flag is the *definition*: name, type, default value. Per-environment
-- behavior lives in flag_environments below — separating these means you can
-- define a flag once and configure it differently per env.
-- =============================================================================

CREATE TABLE flags (
    id            UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id    UUID         NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    key           VARCHAR(128) NOT NULL,
    name          VARCHAR(255) NOT NULL,
    description   TEXT,
    type          VARCHAR(16)  NOT NULL DEFAULT 'boolean',
    default_value JSONB        NOT NULL DEFAULT 'false'::jsonb,
    archived_at   TIMESTAMPTZ,
    created_at    TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    created_by    UUID         REFERENCES users(id) ON DELETE SET NULL,

    UNIQUE (project_id, key),
    CONSTRAINT flags_type_valid CHECK (type IN ('boolean', 'string', 'number', 'json')),
    CONSTRAINT flags_key_format CHECK (key ~ '^[a-z0-9][a-z0-9_-]*[a-z0-9]$')
);
CREATE TRIGGER flags_set_updated_at
    BEFORE UPDATE ON flags
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();


-- Per-environment configuration of a flag. This is the row read on every
-- evaluation, so it's the hottest table.
--
-- The `version` column is an optimistic concurrency token: clients pass the
-- expected version on UPDATE; if it doesn't match, the write is rejected.
-- This prevents the classic "two admins toggle the same flag at once" bug.
CREATE TABLE flag_environments (
    flag_id        UUID         NOT NULL REFERENCES flags(id) ON DELETE CASCADE,
    environment_id UUID         NOT NULL REFERENCES environments(id) ON DELETE CASCADE,

    enabled        BOOLEAN      NOT NULL DEFAULT FALSE,    -- master kill switch
    rules          JSONB        NOT NULL DEFAULT '[]'::jsonb,
    default_value  JSONB,                                  -- override flag default for this env
    rollout        JSONB,                                  -- e.g. { "percentage": 25, "seed": "..." }

    version        BIGINT       NOT NULL DEFAULT 1,
    created_at     TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at     TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_by     UUID         REFERENCES users(id) ON DELETE SET NULL,

    PRIMARY KEY (flag_id, environment_id)
);
CREATE TRIGGER flag_environments_set_updated_at
    BEFORE UPDATE ON flag_environments
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- For listing all flag configs in an environment (dashboard, SDK bootstrap)
CREATE INDEX flag_environments_by_env_idx
    ON flag_environments(environment_id);


-- =============================================================================
-- Audit log (append-only)
-- =============================================================================
-- Every mutation in the system is recorded here. Application code MUST only
-- INSERT into this table — never UPDATE or DELETE. We deliberately don't add
-- ON UPDATE / ON DELETE triggers because the absence of a "way to mutate"
-- documents the intent.
--
-- changes is a JSONB diff: { "before": {...}, "after": {...} }
-- =============================================================================

CREATE TABLE audit_log (
    id            UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id     UUID         NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    actor_id      UUID         REFERENCES users(id) ON DELETE SET NULL,
    actor_type    VARCHAR(32)  NOT NULL DEFAULT 'user',
    action        VARCHAR(64)  NOT NULL,         -- e.g. flag.created, flag.updated, key.revoked
    resource_type VARCHAR(32)  NOT NULL,         -- e.g. flag, segment, environment
    resource_id   UUID,
    changes       JSONB,
    ip_address    INET,
    user_agent    TEXT,
    created_at    TIMESTAMPTZ  NOT NULL DEFAULT NOW(),

    CONSTRAINT audit_log_actor_type_valid
        CHECK (actor_type IN ('user', 'api_key', 'system'))
);

-- Hot path: "show me the audit log for my tenant, newest first"
CREATE INDEX audit_log_tenant_recent_idx
    ON audit_log(tenant_id, created_at DESC);

-- "What happened to this specific flag?"
CREATE INDEX audit_log_resource_idx
    ON audit_log(resource_type, resource_id)
    WHERE resource_id IS NOT NULL;


-- -----------------------------------------------------------------------------
-- Immutability guard: audit_log is append-only
-- -----------------------------------------------------------------------------
-- The comment in the table definition says "never UPDATE or DELETE", but
-- comments don't stop bugs or rogue queries. This trigger enforces it at the
-- DB level — any UPDATE or DELETE on audit_log raises an exception.
-- Required for SOC2/ISO 27001 compliance audits.
-- -----------------------------------------------------------------------------
CREATE OR REPLACE FUNCTION prevent_audit_mutation() RETURNS TRIGGER AS $$
BEGIN
    RAISE EXCEPTION 'audit_log is append-only: % operations are forbidden', TG_OP;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER audit_log_immutable
    BEFORE UPDATE OR DELETE ON audit_log
    FOR EACH ROW EXECUTE FUNCTION prevent_audit_mutation();


-- -----------------------------------------------------------------------------
-- Auto-increment version on flag_environments UPDATE
-- -----------------------------------------------------------------------------
-- Optimistic concurrency control depends on `version` being incremented on
-- every write. If application code forgets to bump it, the guarantee breaks.
-- This trigger makes it automatic and tamper-proof: the app passes the
-- expected version in a WHERE clause, and the trigger handles the increment.
-- -----------------------------------------------------------------------------
CREATE OR REPLACE FUNCTION increment_flag_version() RETURNS TRIGGER AS $$
BEGIN
    NEW.version = OLD.version + 1;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER flag_environments_auto_version
    BEFORE UPDATE ON flag_environments
    FOR EACH ROW EXECUTE FUNCTION increment_flag_version();


COMMIT;
