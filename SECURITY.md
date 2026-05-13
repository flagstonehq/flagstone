# Flagstone — Security Model

> This document describes Flagstone's authentication, authorization, cryptographic choices, and threat model. It's written for two audiences: developers implementing the system, and auditors evaluating it.

---

## Table of Contents

1. [Authentication](#authentication)
2. [Authorization (RBAC)](#authorization-rbac)
3. [Cryptographic Choices](#cryptographic-choices)
4. [Input Validation](#input-validation)
5. [Rate Limiting](#rate-limiting)
6. [Infrastructure Security](#infrastructure-security)
7. [Threat Model](#threat-model)
8. [Compliance Considerations](#compliance-considerations)
9. [Incident Response Checklist](#incident-response-checklist)
10. [Security Roadmap](#security-roadmap)

---

## Authentication

Flagstone has two authentication paths for two types of callers.

### Path 1: API Keys (SDKs / machines)

API keys are the primary authentication method for SDKs and automated systems. They are scoped to a single environment — if the production key leaks, dev and staging are unaffected.

#### Key lifecycle

```
Creation:
  1. Admin requests new key via dashboard (POST /api/v1/api-keys)
  2. Server generates 32 bytes from crypto/rand
  3. Server encodes as base62 with prefix: fs_live_<random> or fs_test_<random>
  4. Server stores SHA-256(full_key) as key_hash and first 12 chars as key_prefix
  5. Server returns the full key to the admin ONCE
  6. Admin stores it in their application's environment variables

Authentication (every SDK request):
  1. SDK sends: Authorization: Bearer fs_live_a3b9d2c8e4f1...
  2. Server computes SHA-256 of the bearer token
  3. Server queries: SELECT * FROM api_keys WHERE key_hash = $1 AND revoked_at IS NULL
  4. Server checks expires_at (if set)
  5. Server updates last_used_at (async, non-blocking)
  6. Server resolves environment_id → all subsequent queries scoped to that environment

Revocation:
  1. Admin revokes key via dashboard (DELETE /api/v1/api-keys/:id)
  2. Server sets revoked_at = NOW()
  3. Key is immediately rejected on next request
  4. Key remains in DB for audit trail (soft delete)
```

#### Key format

```
fs_live_a3b9d2c8e4f1g5h7j8k9m2n4p6q8r1s3t5
│  │    └── 32 bytes of crypto/rand, base62 encoded
│  └── environment hint: "live" (production) or "test" (dev/staging)
└── product prefix: "fs" = Flagstone
```

The environment hint is informational only — the actual environment binding comes from the database FK. This prevents an attacker from changing `test` to `live` in the key string to gain production access.

#### Why the key is never stored

```
What we store:          What the admin has:
┌──────────────┐       ┌──────────────────────────┐
│ key_hash:    │       │ fs_live_a3b9d2c8e4f1...  │
│ SHA-256(key) │       │ (the full key)            │
│              │       │                           │
│ key_prefix:  │       │ Stored in their env vars, │
│ fs_live_a3b9 │       │ not in our database       │
└──────────────┘       └──────────────────────────┘
```

If the database is compromised, an attacker gets SHA-256 hashes. With 32 bytes of random input (256 bits of entropy), brute-forcing is computationally infeasible — it would take longer than the age of the universe.

### Path 2: JWT Sessions (dashboard users)

Dashboard users authenticate with email + password and receive JWT tokens.

#### Login flow

```
POST /api/v1/auth/login
{
  "email": "admin@company.com",
  "password": "correct-horse-battery-staple"
}

Server:
  1. Lookup user by email (case-insensitive via CITEXT)
  2. Verify password with bcrypt.CompareHashAndPassword()
  3. Check email_verified_at is not NULL (if email verification is enabled)
  4. Generate access token (JWT, 15 min TTL)
  5. Generate refresh token (opaque, 7 day TTL)
  6. Store SHA-256(refresh_token) in sessions table
  7. Update last_login_at on user
  8. Audit log: auth.login

Response:
{
  "access_token": "eyJhbGciOiJIUzI1NiIs...",
  "token_type": "Bearer",
  "expires_in": 900
}
+ Set-Cookie: refresh_token=<opaque>; HttpOnly; Secure; SameSite=Strict; Path=/api/v1/auth
```

#### Access token (JWT) structure

```json
{
  "header": {
    "alg": "HS256",
    "typ": "JWT"
  },
  "payload": {
    "sub": "user-uuid",
    "tid": "tenant-uuid",
    "role": "admin",
    "iat": 1700000000,
    "exp": 1700000900,
    "iss": "flagstone"
  }
}
```

| Claim | Purpose |
|---|---|
| `sub` | User ID — who is making the request |
| `tid` | Tenant ID — which organization's data to access |
| `role` | Role within this tenant — determines permissions |
| `exp` | Expiration — 15 minutes from issue |
| `iss` | Issuer — always "flagstone" (prevents token confusion) |

**Why 15 minutes?** Short-lived tokens limit the damage window if a token is stolen. The refresh flow is transparent to the user (the frontend handles it automatically).

#### Token refresh flow

```
POST /api/v1/auth/refresh
Cookie: refresh_token=<opaque>

Server:
  1. Extract refresh token from httpOnly cookie
  2. Compute SHA-256(refresh_token)
  3. Lookup in sessions table by refresh_hash
  4. Verify not expired
  5. Delete old session row (token rotation)
  6. Create new session with new refresh token
  7. Issue new access token (JWT)
  8. Audit log: auth.refresh

Response:
  New access_token + new refresh_token cookie
```

**Token rotation**: every refresh invalidates the old refresh token and issues a new one. If an attacker steals a refresh token and the legitimate user also refreshes, one of them will get an "invalid token" error — signaling the compromise.

#### Logout

```
POST /api/v1/auth/logout
Authorization: Bearer <access_token>
Cookie: refresh_token=<opaque>

Server:
  1. Delete session row matching refresh_hash
  2. Clear refresh_token cookie
  3. Audit log: auth.logout
```

The access token remains valid until its 15-minute expiration (JWTs are stateless). For immediate access token invalidation, a token blocklist in Redis can be added later — but for 15 minutes of exposure, the tradeoff is acceptable.

---

## Authorization (RBAC)

### Role hierarchy

```
owner > admin > member > viewer
```

Each user has a role per tenant (via `tenant_members` table). A user can belong to multiple tenants with different roles.

### Permission matrix

| Action | viewer | member | admin | owner |
|---|---|---|---|---|
| **Flags** | | | | |
| List/read flags | Yes | Yes | Yes | Yes |
| Create flag | - | Yes | Yes | Yes |
| Update flag definition | - | Yes | Yes | Yes |
| Update flag per-env config | - | Yes | Yes | Yes |
| Archive flag | - | - | Yes | Yes |
| **Segments** | | | | |
| List/read segments | Yes | Yes | Yes | Yes |
| Create/update segments | - | Yes | Yes | Yes |
| Archive segment | - | - | Yes | Yes |
| **Environments** | | | | |
| List environments | Yes | Yes | Yes | Yes |
| Create environment | - | - | Yes | Yes |
| Delete environment | - | - | - | Yes |
| **API Keys** | | | | |
| List keys (prefix only) | - | Yes | Yes | Yes |
| Create key | - | - | Yes | Yes |
| Revoke key | - | - | Yes | Yes |
| **Members** | | | | |
| List members | Yes | Yes | Yes | Yes |
| Invite member | - | - | Yes | Yes |
| Change member role | - | - | Yes (not to owner) | Yes |
| Remove member | - | - | Yes (not owner) | Yes |
| **Audit Log** | | | | |
| Read audit log | Yes | Yes | Yes | Yes |
| **Tenant** | | | | |
| Update tenant settings | - | - | - | Yes |
| Delete tenant | - | - | - | Yes |

### Implementation

Authorization is enforced in HTTP middleware, after authentication:

```go
// RequireRole returns middleware that ensures the authenticated user
// has at least the specified role within their active tenant.
func RequireRole(minimum Role) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            claims := ClaimsFromContext(r.Context())
            if claims.Role.Level() < minimum.Level() {
                http.Error(w, "insufficient permissions", http.StatusForbidden)
                return
            }
            next.ServeHTTP(w, r)
        })
    }
}

// Route registration:
mux.Handle("POST /api/v1/flags", RequireRole(RoleMember)(createFlagHandler))
mux.Handle("DELETE /api/v1/flags/{key}", RequireRole(RoleAdmin)(archiveFlagHandler))
mux.Handle("POST /api/v1/api-keys", RequireRole(RoleAdmin)(createAPIKeyHandler))
```

### Cross-tenant isolation

Every database query includes `tenant_id` in the WHERE clause. This is enforced at the storage layer — handlers never construct raw SQL. The tenant_id comes from the authenticated JWT or the resolved API key, never from user input.

```go
// Storage layer — tenant_id is always injected from auth context
func (s *Store) ListFlags(ctx context.Context, tenantID, projectID uuid.UUID) ([]Flag, error) {
    return s.query(ctx, `
        SELECT f.* FROM flags f
        JOIN projects p ON f.project_id = p.id
        WHERE p.tenant_id = $1 AND f.project_id = $2 AND f.archived_at IS NULL
    `, tenantID, projectID)
}
```

---

## Cryptographic Choices

| What | Algorithm | Why |
|---|---|---|
| API key hashing | SHA-256 | Keys are high-entropy (32 bytes random). Fast hash is appropriate — no brute-force risk. We hash on every request; bcrypt would add ~100ms. |
| Password hashing | bcrypt (cost=12) | Passwords are human-chosen, potentially weak. bcrypt's work factor (~250ms) makes brute-force impractical. |
| JWT signing | HS256 (HMAC-SHA256) | Single-issuer system — symmetric signing is simpler than RSA/ECDSA. The signing key never leaves the server. |
| Refresh tokens | 32 bytes crypto/rand | Opaque tokens, stored as SHA-256 hash in DB. Same security model as API keys. |
| API key generation | crypto/rand | Go's `crypto/rand` reads from `/dev/urandom` on Linux. CSPRNG, suitable for security-critical random values. |

### Why NOT argon2id for passwords?

Argon2id is technically superior to bcrypt (memory-hard, resistant to GPU attacks). However:

- bcrypt is battle-tested, widely understood, and available in Go's standard `x/crypto` library.
- The security difference is marginal for our threat model (we're not protecting nuclear codes).
- If needed, migrating from bcrypt to argon2id is straightforward: verify with bcrypt, re-hash with argon2id on successful login.

### JWT signing key management

The JWT secret is loaded from the `JWT_SECRET` environment variable. Requirements:

- Minimum 32 bytes (256 bits) of entropy
- Generated with `openssl rand -hex 32`
- Never committed to version control
- Rotated via: set new secret → old tokens expire naturally (15 min) → done

For key rotation with zero downtime, a future enhancement could support a list of signing keys (verify with any, sign with newest).

---

## Input Validation

### JSONB rule validation

Rules stored in `flag_environments.rules` and `segments.rules` are JSONB — the database doesn't validate their structure. The application MUST validate before storing:

| Check | Limit | Why |
|---|---|---|
| Max nesting depth | 10 levels | Prevents stack overflow during recursive evaluation |
| Max conditions per rule | 50 | Prevents DoS via complex rules that take too long to evaluate |
| Max rules per flag | 100 | Prevents unbounded growth |
| Max JSONB size | 64 KB | Prevents memory exhaustion |
| Operator whitelist | `eq`, `neq`, `gt`, `gte`, `lt`, `lte`, `contains`, `starts_with`, `ends_with`, `in`, `not_in`, `matches`, `exists`, `not_exists`, `segment` | Prevents injection of unknown operators |
| Attribute name format | `^[a-zA-Z_][a-zA-Z0-9_.]{0,63}$` | Prevents weird attribute names |
| Segment reference validation | Must exist in same project | Prevents dangling references (soft check — segment could be archived later) |

### API input validation

| Field | Validation |
|---|---|
| Flag key | `^[a-z0-9][a-z0-9_-]*[a-z0-9]$`, max 128 chars |
| Flag name | Max 255 chars, non-empty |
| Email | Valid RFC 5322 format (via `net/mail`) |
| Password | Minimum 8 chars, max 72 chars (bcrypt limit) |
| Slug | `^[a-z0-9]([a-z0-9-]*[a-z0-9])?$`, max 64 chars |
| Description | Max 2000 chars |
| Pagination `limit` | 1-100, default 20 |
| Pagination `offset` | >= 0 |
| UUID parameters | Valid UUID v4 format |

### SQL injection prevention

All database queries use parameterized statements (`$1`, `$2`, etc.). No string concatenation of user input into SQL. This is enforced architecturally: the storage layer accepts Go types, never raw strings that could contain SQL.

---

## Rate Limiting

### Strategy: token bucket per IP (in-process)

Rate limiting is implemented in-process using a token bucket algorithm per client IP. No external dependencies (Redis) needed for v1.

| Endpoint | Limit | Burst | Why |
|---|---|---|---|
| `POST /auth/login` | 5/min per IP | 5 | Prevent credential brute-force |
| `POST /auth/refresh` | 10/min per IP | 10 | Prevent refresh token abuse |
| `POST /evaluate/*` | 1000/min per API key | 100 | Prevent evaluation abuse |
| `GET /stream` | 10 connections per API key | 10 | Prevent SSE connection exhaustion |
| All other endpoints | 60/min per JWT | 20 | General protection |

### Response on limit exceeded

```
HTTP 429 Too Many Requests
Retry-After: 30
Content-Type: application/json

{
  "error": {
    "code": "RATE_LIMITED",
    "message": "Too many requests. Retry after 30 seconds.",
    "retry_after": 30
  }
}
```

### Future: distributed rate limiting

When running multiple server instances, rate limiting needs to be coordinated. Options:
1. **Redis-backed token bucket** (adds ~1ms latency per request)
2. **Approximate local rate limiting** with periodic sync — each instance tracks locally and syncs counts to Redis every few seconds. Good enough for rate limiting (exact counts don't matter).

---

## Infrastructure Security

### Network architecture

```
Internet
    │
    ▼
┌─────────────────────────────────┐
│ Security Group: app-sg          │
│ ┌─────────────────────────────┐ │
│ │ EC2 (public subnet)         │ │
│ │  - Port 80/443: 0.0.0.0/0  │ │
│ │  - Port 22: YOUR_IP/32     │ │
│ │  - IMDSv2 required         │ │
│ │  - IAM role (no keys)      │ │
│ │  - EBS encrypted           │ │
│ └─────────────┬───────────────┘ │
└───────────────┼─────────────────┘
                │ Port 5432 only
                ▼
┌─────────────────────────────────┐
│ Security Group: db-sg           │
│ ┌─────────────────────────────┐ │
│ │ RDS (private subnet)        │ │
│ │  - Only app-sg can connect  │ │
│ │  - No public access         │ │
│ │  - No outbound internet     │ │
│ │  - Storage encrypted        │ │
│ │  - Automated backups (7d)   │ │
│ └─────────────────────────────┘ │
└─────────────────────────────────┘
```

### SSH hardening

- `ssh_allowed_cidr` has **no default value** — Terraform refuses to run until the operator explicitly sets their IP. The previous default of `0.0.0.0/0` was removed as a security fix.
- Terraform validates the CIDR is not `0.0.0.0/0` even if someone tries to set it explicitly.
- AWS Session Manager (SSM) is configured as a backup — SSH-less access if port 22 is locked.

### Secrets management

**Current state** (acceptable for solo dev):
- `db_password` passed via Terraform variable (marked `sensitive`)
- `JWT_SECRET` in `.env` file (gitignored)
- API keys stored as hashes in DB

**Future state** (when there are 2+ developers or production customers):
- AWS Secrets Manager for all secrets
- Terraform data source reads from Secrets Manager instead of variables
- Automatic rotation for DB password

---

## Threat Model

### Assets to protect

| Asset | Sensitivity | Impact if compromised |
|---|---|---|
| Flag configurations | Medium | Incorrect feature exposure, but no data breach |
| User passwords | High | Account takeover |
| API keys | High | Unauthorized flag reads for that environment |
| JWT signing secret | Critical | Forge any user session, full tenant access |
| Database credentials | Critical | Full data access |
| Audit log | Medium | Loss of forensic evidence |
| Customer evaluation data | Low | User IDs and attributes visible in traces |

### Threat matrix

| # | Threat | Attack vector | Mitigation | Status |
|---|---|---|---|---|
| T1 | **Database leak** → API keys exposed | SQL injection, backup theft, insider | API keys stored as SHA-256 hashes. High-entropy input makes brute-force infeasible. | Mitigated |
| T2 | **Database leak** → passwords exposed | Same as T1 | Passwords hashed with bcrypt (cost=12). Work factor makes brute-force impractical. | Mitigated |
| T3 | **Database leak** → JWT secret exposed | Same as T1 | JWT secret is NOT in the database — it's in environment variables. DB leak doesn't compromise it. | Mitigated |
| T4 | **SSRF** → steal IAM credentials | Application bug allowing arbitrary HTTP requests | IMDSv2 required — SSRF can't read metadata without a token. | Mitigated |
| T5 | **SSH brute force** | Direct connection to port 22 | SG restricted to operator IP. No default `0.0.0.0/0`. SSM as backup. | Mitigated |
| T6 | **Direct RDS attack** | Port scan, direct connection | Private subnet, SG only allows app-sg on 5432. Not publicly accessible. | Mitigated |
| T7 | **DDoS on API** | Volumetric attack, resource exhaustion | In-process rate limiting (token bucket per IP). | Planned (M1) |
| T8 | **Cross-tenant data access** | Manipulated tenant_id in requests | tenant_id comes from authenticated JWT/API key, never from user input. All queries include tenant_id. | Mitigated |
| T9 | **Audit log tampering** | Direct DB access, SQL injection | Database trigger raises exception on UPDATE/DELETE of audit_log. | Mitigated |
| T10 | **Malicious JSONB rules** | Admin injects deeply nested / huge rules | Application validates: max depth 10, max size 64KB, operator whitelist. | Planned (M1) |
| T11 | **Credential brute force** | Repeated login attempts | Rate limit: 5 login attempts/min per IP. bcrypt work factor provides additional protection. | Planned (M1) |
| T12 | **JWT token theft** | XSS, network interception | Access tokens in memory (not localStorage). Refresh tokens in httpOnly/Secure/SameSite cookies. HTTPS only in production. 15 min access token TTL limits exposure. | Planned (M1) |
| T13 | **Refresh token replay** | Steal refresh token from cookie | Token rotation: each refresh invalidates the old token. If both attacker and user try to refresh, one gets an error — signaling compromise. | Planned (M1) |
| T14 | **Enumeration attacks** | Guess resource IDs via sequential numbers | UUIDs for all public-facing IDs. Error messages don't distinguish "not found" from "not authorized" for inaccessible resources. | Mitigated |
| T15 | **Stale flag evaluation** | Cache returns old value | Acceptable by design. TTL 30s as worst case. Redis pub/sub for fast invalidation. SDKs fallback to cached value if server is unreachable. | Accepted risk |
| T16 | **API key in logs/URLs** | Key appears in access logs or query strings | Keys are sent in Authorization header (not URL). Server never logs full key values — only key_prefix. | Mitigated |
| T17 | **CORS bypass** | Cross-origin requests from malicious sites | CORS whitelist configured per deployment. Dashboard and API on same origin when possible. | Planned (M2) |

### Accepted risks

| Risk | Justification | Revisit when |
|---|---|---|
| JWT not immediately revocable | 15 min TTL limits exposure. Adding a Redis blocklist adds complexity for marginal gain. | A paying customer requires immediate revocation. |
| No WAF | AWS WAF costs $5/month minimum. Rate limiting covers the basic cases. | Public-facing production deployment with untrusted traffic. |
| Single-AZ database | 7-day automated backups. ~30 min recovery. No SLA yet. | Paying customer with uptime SLA. |
| No encryption in transit (internal) | EC2 to RDS is within VPC, same region. Attack requires VPC compromise which implies game over already. | Multi-region or multi-VPC deployment. |

---

## Compliance Considerations

### SOC2 Type II readiness

| Control | Implementation | Gap |
|---|---|---|
| Access control | RBAC with 4 roles, JWT + API key auth | None |
| Audit logging | Append-only, DB-enforced immutability | None |
| Data encryption at rest | EBS + RDS encrypted | None |
| Data encryption in transit | HTTPS (TLS termination at Caddy/ALB) | Planned (M2) |
| Password policy | bcrypt, minimum 8 chars | Could add complexity requirements |
| Session management | JWT 15 min + refresh token rotation | None |
| Change management | All flag changes in audit log with actor, timestamp, diff | None |
| Incident response | Planned | Need formal runbook |

### GDPR considerations

- User emails stored in `users` table (PII).
- User IDs and attributes may appear in evaluation traces (passed by SDKs).
- Audit log references user IDs (who changed what).
- **Right to erasure**: Can anonymize user record (set email to hash, clear password_hash). Audit log entries retain the user_id but the mapping to PII is broken. This preserves the audit trail while complying with erasure requests.
- **Data retention**: Audit log is append-only. Partitioning by month allows archival of old data while maintaining recent data.

---

## Incident Response Checklist

### API key compromised

1. Immediately revoke the key: `DELETE /api/v1/api-keys/:id`
2. Check audit log for unauthorized actions during the exposure window
3. Create new key for the affected environment
4. Update the key in all systems that use it
5. Review how the key was leaked (logs, code repo, etc.)

### JWT signing secret compromised

1. Generate new secret: `openssl rand -hex 32`
2. Update `JWT_SECRET` environment variable
3. Restart all server instances
4. All existing JWTs immediately become invalid (users must re-login)
5. All refresh tokens become invalid (tied to the old signing key)
6. Investigate how the secret was exposed

### Database credentials compromised

1. Rotate RDS master password via AWS Console or CLI
2. Update Terraform variables and application config
3. Restart application
4. Review CloudTrail and RDS audit logs for unauthorized access
5. Consider enabling RDS IAM authentication as a replacement

### Suspected unauthorized access

1. Check audit log: `GET /api/v1/audit?actor_id=<suspect>&since=<time>`
2. Check API key `last_used_at` timestamps
3. Review CloudWatch logs for unusual request patterns
4. If confirmed: revoke all sessions for the user, reset password, revoke API keys
5. Document findings for post-mortem

---

## Security Roadmap

### Milestone 1 (MVP) — implemented with core features

- [x] API key authentication (SHA-256 hashed)
- [x] Audit log with DB-enforced immutability
- [x] Optimistic concurrency control (auto-increment version)
- [x] Input validation on slugs and keys (DB constraints)
- [ ] JWT authentication for dashboard
- [ ] bcrypt password hashing
- [ ] Rate limiting (in-process token bucket)
- [ ] JSONB rule validation (depth, size, operator whitelist)
- [ ] RBAC middleware

### Milestone 2 — production hardening

- [ ] HTTPS / TLS termination (Caddy or ALB)
- [ ] CORS configuration
- [ ] CSRF protection (if using cookie-based auth)
- [ ] Security headers (CSP, HSTS, X-Frame-Options)
- [ ] Dependency vulnerability scanning (Dependabot / govulncheck)
- [ ] API key expiration enforcement

### Milestone 3 — enterprise features

- [ ] SSO / OAuth2 integration (Google, GitHub, SAML)
- [ ] IP allowlisting per tenant
- [ ] AWS Secrets Manager integration
- [ ] Audit log export (S3, SIEM integration)
- [ ] Penetration test

---

## Reporting Vulnerabilities

If you find a security vulnerability, please do NOT open a public GitHub issue. Instead, email **security@flagstone.dev** (to be set up) with:

1. Description of the vulnerability
2. Steps to reproduce
3. Potential impact
4. Suggested fix (if any)

We aim to acknowledge reports within 48 hours and provide a fix within 7 days for critical issues.
