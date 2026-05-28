-- =============================================================================
-- Flagstone — Add tenant_id to revoked_refresh_tokens
-- =============================================================================
-- The revoked_refresh_tokens table originally only stored user_id, making it
-- impossible to create a proper audit log entry (which requires tenant_id)
-- when a refresh token reuse is detected.
--
-- This migration adds tenant_id so handleRefreshReuse can record the event
-- with the correct tenant context.
-- =============================================================================

BEGIN;

ALTER TABLE revoked_refresh_tokens
    ADD COLUMN tenant_id UUID REFERENCES tenants(id) ON DELETE CASCADE;

-- Backfill tenant_id from sessions. In practice the table should be empty
-- or nearly empty, but we handle stale rows gracefully.
UPDATE revoked_refresh_tokens rrt
SET tenant_id = s.tenant_id
FROM sessions s
WHERE rrt.tenant_id IS NULL
  AND s.user_id = rrt.user_id;

-- Now that every row has a tenant_id, make it NOT NULL.
ALTER TABLE revoked_refresh_tokens
    ALTER COLUMN tenant_id SET NOT NULL;

CREATE INDEX revoked_refresh_tokens_tenant_idx
    ON revoked_refresh_tokens(tenant_id);

COMMIT;
