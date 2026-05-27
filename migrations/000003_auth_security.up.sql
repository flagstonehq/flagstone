-- =============================================================================
-- Flagstone — Auth security: account lockout + refresh token reuse detection
-- =============================================================================
-- Two tables backing the hardening described in SECURITY.md threats T19/T20:
--
--   1. login_attempts
--      Records every failed login. handleLogin counts rows for a given user
--      in the past 15 minutes; if the count crosses the threshold (5), the
--      account is locked and we return 423 instead of running bcrypt. The
--      lockout check sits BEFORE bcrypt so an attacker cannot weaponize the
--      verifier's CPU cost.
--
--   2. revoked_refresh_tokens
--      Every refresh hash we rotate or invalidate is recorded here. When a
--      client presents a refresh token whose hash is in this table, we treat
--      it as a replay (T20: refresh token reuse) and kill every session for
--      the user — both the legitimate one and the attacker's.
--
-- Both tables hold short-lived rows. A background cleanup job is expected to
-- delete expired entries periodically; until that runs, the table sizes are
-- bounded by traffic × retention window.
-- =============================================================================

BEGIN;

-- -----------------------------------------------------------------------------
-- login_attempts — failed-login bookkeeping for account lockout (T19)
-- -----------------------------------------------------------------------------
-- We only record FAILED attempts. A successful login wipes the user's rows.
-- The (user_id, attempted_at DESC) index supports the hot-path count query.
CREATE TABLE login_attempts (
    id           UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id      UUID         NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    ip_address   INET,
    user_agent   TEXT,
    attempted_at TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);
CREATE INDEX login_attempts_user_attempted_idx ON login_attempts(user_id, attempted_at DESC);
CREATE INDEX login_attempts_attempted_at_idx   ON login_attempts(attempted_at);


-- -----------------------------------------------------------------------------
-- revoked_refresh_tokens — replayed-refresh detection (T20)
-- -----------------------------------------------------------------------------
-- We store the SHA-256 hash of every refresh token we've rotated out (or that
-- was burned by logout). On refresh, handleRefresh checks this table first; a
-- hit means the same token was presented twice and we kill every session for
-- the user.
--
-- expires_at copies the original session's expiry, since after that point any
-- session bearing this hash would already be gone — the row exists only to
-- catch replays within the original token's TTL.
CREATE TABLE revoked_refresh_tokens (
    id          UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    token_hash  CHAR(64)     NOT NULL UNIQUE,
    user_id     UUID         NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    revoked_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    expires_at  TIMESTAMPTZ  NOT NULL
);
CREATE INDEX revoked_refresh_tokens_user_idx       ON revoked_refresh_tokens(user_id);
CREATE INDEX revoked_refresh_tokens_expires_at_idx ON revoked_refresh_tokens(expires_at);

COMMIT;
