BEGIN;

DROP TABLE IF EXISTS tenant_invitations;
DROP TABLE IF EXISTS password_reset_tokens;
DROP TABLE IF EXISTS email_verification_tokens;

COMMIT;
