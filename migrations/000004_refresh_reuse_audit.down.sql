BEGIN;

DROP INDEX IF EXISTS revoked_refresh_tokens_tenant_idx;

ALTER TABLE revoked_refresh_tokens
    DROP COLUMN IF EXISTS tenant_id;

COMMIT;
