-- =============================================================================
-- Flagstone — Email flow tokens
-- =============================================================================
-- Tables backing three user-facing email flows:
--   1. Email verification (signup → click link → email_verified_at set)
--   2. Password reset       (forgot password → click link → set new password)
--   3. Tenant invitations   (admin invites → click link → join tenant)
--
-- Conventions:
--   * Tokens are opaque, 32 bytes from crypto/rand, base62 encoded.
--   * We store SHA-256(token), never the raw token. The raw token is in the
--     email link only, never logged or persisted.
--   * Each row is single-use: rotation = delete-then-insert.
--   * All tokens expire (TTL varies per flow). A background goroutine deletes
--     expired rows daily.
-- =============================================================================

BEGIN;

-- -----------------------------------------------------------------------------
-- Email verification tokens
-- -----------------------------------------------------------------------------
-- Sent when a user signs up (or changes their email later). Clicking the link
-- sets users.email_verified_at = NOW() and deletes the token.
CREATE TABLE email_verification_tokens (
    id          UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     UUID         NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash  CHAR(64)     NOT NULL UNIQUE,           -- SHA-256 of raw token
    expires_at  TIMESTAMPTZ  NOT NULL,                  -- 24h TTL typically
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);
CREATE INDEX email_verification_tokens_user_idx       ON email_verification_tokens(user_id);
CREATE INDEX email_verification_tokens_expires_at_idx ON email_verification_tokens(expires_at);


-- -----------------------------------------------------------------------------
-- Password reset tokens
-- -----------------------------------------------------------------------------
-- Issued on POST /api/v1/auth/forgot-password. Short TTL (1 hour) to limit
-- the exposure window. Single-use: used or expired → deleted.
CREATE TABLE password_reset_tokens (
    id          UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     UUID         NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash  CHAR(64)     NOT NULL UNIQUE,
    expires_at  TIMESTAMPTZ  NOT NULL,                  -- 1h TTL
    ip_address  INET,                                   -- for audit
    user_agent  TEXT,
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);
CREATE INDEX password_reset_tokens_user_idx       ON password_reset_tokens(user_id);
CREATE INDEX password_reset_tokens_expires_at_idx ON password_reset_tokens(expires_at);


-- -----------------------------------------------------------------------------
-- Tenant invitations
-- -----------------------------------------------------------------------------
-- An admin invites a person by email. The invitee may not have a user account
-- yet (so user_id is nullable). When they accept:
--   * if they have an account → tenant_members row created with the offered role
--   * if not → they sign up first, then the invite is accepted automatically
--     based on the matching email
CREATE TABLE tenant_invitations (
    id          UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id   UUID         NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    email       CITEXT       NOT NULL,                  -- invitee's email
    role        VARCHAR(32)  NOT NULL DEFAULT 'member',
    token_hash  CHAR(64)     NOT NULL UNIQUE,
    invited_by  UUID         REFERENCES users(id) ON DELETE SET NULL,
    expires_at  TIMESTAMPTZ  NOT NULL,                  -- 7d TTL
    accepted_at TIMESTAMPTZ,                            -- NULL = still pending
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW(),

    CONSTRAINT tenant_invitations_role_valid
        CHECK (role IN ('admin', 'member', 'viewer')),  -- can't invite as owner
    -- Prevent duplicate pending invites for the same (tenant, email)
    UNIQUE (tenant_id, email, accepted_at)
);
CREATE INDEX tenant_invitations_tenant_idx     ON tenant_invitations(tenant_id);
CREATE INDEX tenant_invitations_email_idx      ON tenant_invitations(email) WHERE accepted_at IS NULL;
CREATE INDEX tenant_invitations_expires_at_idx ON tenant_invitations(expires_at);

COMMIT;
