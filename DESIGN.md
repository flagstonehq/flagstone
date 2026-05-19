# Flagstone — Design Decisions

> This document justifies every design decision in the project. The goal is that 6 months from now, when you (or someone else) asks "why did we do X this way?", the answer is here.

---

## Table of Contents

1. [Guiding Principles](#guiding-principles)
2. [Tech Stack](#tech-stack)
3. [Database Design](#database-design)
   - [Initial Schema (Migration 000001)](#initial-schema-migration-000001)
   - [Zero-downtime Migration Strategy](#zero-downtime-migration-strategy)
4. [Rule Evaluation Engine](#rule-evaluation-engine)
   - [Rules Schema Specification](#rules-schema-specification)
   - [Evaluation Flow (complete)](#evaluation-flow-complete)
   - [Error Policy](#error-policy-resilience-over-correctness)
   - [Rollout Configuration Spec](#rollout-configuration-spec)
   - [Eager loading: avoiding N+1 queries](#eager-loading-avoiding-n1-queries)
5. [Authentication & Authorization](#authentication--authorization)
   - [Onboarding & Bootstrap Flow](#onboarding--bootstrap-flow)
6. [Caching & Propagation](#caching--propagation)
   - [SDK Bootstrap Race Condition](#sdk-bootstrap-race-condition)
7. [API Design](#api-design)
   - [Evaluation API Contracts](#evaluation-api-contracts)
   - [Health Check Design](#health-check-design)
   - [SSE Event Format](#sse-event-format)
   - [SSE Reconnection & Backoff](#sse-reconnection--backoff)
8. [Observability](#observability)
9. [Infrastructure (AWS)](#infrastructure-aws)
10. [Cost Strategy](#cost-strategy)
11. [Monetization Model](#monetization-model)
12. [Competitive Landscape](#competitive-landscape)
13. [Implementation Conventions](#implementation-conventions)
    - [Logging](#logging)
    - [Database connection pool sizing](#database-connection-pool-sizing)
    - [Plan enforcement (tenant quotas)](#plan-enforcement-tenant-quotas)
14. [Deferred Decisions](#deferred-decisions)
15. [References](#references)

---

## Architecture Diagrams

> Full visual reference: [`diagrams/flagstone-overview.svg`](./diagrams/flagstone-overview.svg)

| Diagram | Maps to section |
|---------|----------------|
| Flagstone · Diagrama Entidad-Relación | [Database Design](#database-design) |
| Diagrama 4: Autenticación Dual (API Keys + JWT) | [Authentication & Authorization](#authentication--authorization) |
| Diagrama 5: Flujo de evaluación de flags (SDK Runtime) | [Rule Evaluation Engine](#rule-evaluation-engine) |
| Diagrama 6: Publicación de cambios | [Caching & Propagation](#caching--propagation) |
| Diagrama 7: Arquitectura AWS | [Infrastructure (AWS)](#infrastructure-aws) |
| Diagrama 8: API Design + Health Check | [API Design](#api-design) |
| Diagrama 9: Observabilidad — OTel + Prometheus + Grafana | [Observability](#observability) |
| FLAGSTONE — Flujo completo DESIGN.md | Full SDK → API → Redis → Postgres → OTel sequence |

---

## Guiding Principles

Before specific decisions, the principles that guide everything:

**1. Boring tech wins.** At every fork we choose the technology most people already know. Postgres instead of something exotic. Go instead of the latest trendy framework. When something breaks at 3 AM, there are 50,000 StackOverflow posts ready to help.

**2. Optimize for readability, not performance.** Until a benchmark says otherwise. Feature flags are an I/O-bound service (network, database), not CPU-bound. A rule evaluated with a map lookup vs an optimized switch has the same perceptible latency.

**3. Zero paid infrastructure until there's traction.** Every infra decision starts from "can we do this for free?" and we only scale when a real user justifies it.

**4. Design for multi-tenant from day one.** Even though we start solo, every model has `tenant_id`. Migrating to multi-tenant later is a nightmare — doing it from the start costs almost nothing.

**5. Append-only by default.** Soft-delete (`archived_at`, `revoked_at`) instead of DELETE. The audit log is never modified — enforced at the database level with triggers, not just by convention. Reconstructing past state is more valuable than saving storage.

**6. Security is not optional.** Auth, hashing, RBAC, rate limiting, and input validation are core features, not "we'll add it later" items. Every API endpoint is authenticated from day one.

---

## Tech Stack

### Why Go

- **Cheap concurrency**: one SSE connection per client is one goroutine (~8KB stack) — we scale to thousands without effort.
- **Single binary**: deploy = `scp` + `systemctl restart`. No runtimes, no OS dependencies.
- **Free cross-compile**: `GOOS=linux GOARCH=arm64 go build` and it's ready for Graviton.
- **Consistent performance**: modern GC, predictable latencies. Matters when your SLO is "p99 < 5ms".
- **Standard library is excellent**: `net/http`, `log/slog`, `crypto/sha256`, `encoding/json` — minimal external dependencies needed for the core.

### Why Postgres and not another database

- **JSONB** lets us store rules (arbitrary trees) without normalizing into 5 tables. Still queryable when needed.
- **Real transactions**: atomic changes to flags + audit log in a single commit.
- **CITEXT** for emails (case-insensitive) without writing `LOWER()` everywhere.
- **pg_stat_statements**: when something is slow, we know which query it is.
- **Any enterprise customer already has it**: "you need Postgres" is an easy sell. "You need MongoDB and ScyllaDB" is not.

### Why Redis

Redis fills two roles:

1. **Distributed cache** between server instances (when we scale beyond one).
2. **Pub/sub** so a change in one instance propagates to others, which in turn push to their connected SSE clients.

For v1, a single instance + Redis on the same box is enough. When we scale, Redis moves to ElastiCache or a separate service.

### Why REST first, gRPC later

The original design proposed REST + gRPC from day one. After analysis, **gRPC adds complexity that doesn't pay off in the MVP**:

- Dependency on Protocol Buffers (code generation tooling)
- `protoc`, `buf`, `grpc-gateway` toolchain
- Not curl-able (harder to debug)

**Decision**: MVP is REST-only. gRPC is Milestone 3+, when there are SDKs in multiple languages that benefit from typed Protobuf contracts. The internal architecture keeps the transport layer thin so adding gRPC later is mechanical, not architectural.

### Why OpenTelemetry as a first-class citizen

This is our key market differentiator. Every flag evaluation emits:

- A **span** with attributes (`flag.key`, `flag.value`, `user.id`, `rule.matched`).
- **Metrics** in Prometheus format: counters per flag, latency histograms.
- **Structured logs** correlated with the trace.

LaunchDarkly, Flagsmith, and Unleash have integrations with observability systems. We _are_ a native piece of the OTel stack from day one. For teams already living in Grafana/Honeycomb/Datadog, this is a qualitative change.

### Why SSE over WebSockets

Feature flags are **unidirectional**: the server pushes changes to clients. SSE is the right tool:

- Works over standard HTTP (passes through proxies, load balancers, CDNs)
- Automatic reconnection built into the browser/client spec
- Simpler server implementation (just `text/event-stream` responses)
- WebSockets would be overkill (bidirectional not needed)
- Polling wastes resources and has minimum latency of 1 poll interval

Each SSE connection in Go is a goroutine (~8KB stack). With 10,000 connected SDKs = ~80MB of memory. Completely manageable.

---

## Database Design

> ![Entity-Relationship Diagram](./diagrams/diagram-erd.png)

### Hierarchy: Tenant → Project → Environment → Flag

Four levels might seem like a lot, but it mirrors how real teams organize their software:

```
Tenant: "My Company"
  └─ Project: "Mobile App"
       ├─ Environment: "dev"
       ├─ Environment: "staging"
       └─ Environment: "prod"
  └─ Project: "Web Dashboard"
       ├─ Environment: "dev"
       └─ Environment: "prod"
```

A flag is defined at the **Project** level (`new-checkout`) and configured differently per **Environment** (100% in dev, 5% in prod). This separation is implemented in `flags` (definition) and `flag_environments` (per-environment configuration).

**Performance consideration**: This 4-level hierarchy means every flag evaluation needs to resolve `API key → environment → flag_environment → flag → project → tenant`. That's potentially 3-4 JOINs or lookups. This is mitigated by the multi-level cache (see [Caching](#caching--propagation)), but the hot path should pre-load the full config into memory keyed by `environment_id:flag_key`.

### Why UUIDs instead of bigserial

- **No enumeration attacks**: nobody can infer how many customers we have by requesting `/api/flags/1`, `/api/flags/2`...
- **Client-side generation**: useful for offline-first SDKs in the future.
- **Mergeable**: if we ever sync data between instances, no collisions.

The cost is ~16 bytes per PK vs ~8, and larger indexes. For our volume, irrelevant.

### Why JSONB for rules

Targeting rules are arbitrary trees:

```json
{
  "all": [
    { "attribute": "country", "op": "eq", "value": "AR" },
    { "any": [
      { "attribute": "plan", "op": "eq", "value": "premium" },
      { "attribute": "is_admin", "op": "eq", "value": true }
    ]}
  ]
}
```

Modeling this as normalized tables means:
- A `rule_groups` table (any/all)
- A `rule_conditions` table (attribute, op, value)
- A `rule_groups_rule_groups` table for nesting
- Recursive joins to reconstruct the tree

With JSONB it's one column. We validate it in code (not in the DB) on input. We evaluate it in code. Much simpler, and the DB doesn't even need to understand the structure.

**Tradeoff: no referential integrity for segment references.** Rules can reference segments by key. If a segment is deleted, the rules become broken. This MUST be handled at the application level — the rule engine must gracefully handle "segment not found" instead of crashing.

**"What if we need to search all flags that use the `country` attribute?"** Postgres supports GIN indexes on JSONB. If that use case appears, we add one. YAGNI until then.

### Why `version` in `flag_environments`

Optimistic concurrency control. Without it:

```
T1: Admin A reads flag (rollout=10%)
T2: Admin B reads flag (rollout=10%)
T3: Admin A writes rollout=25%
T4: Admin B writes rollout=50%   ← silently overwrites A's change
```

With `version`:

```
T3: Admin A writes rollout=25%, version becomes 2 (was 1) ✓
T4: Admin B writes rollout=50%, expected_version=1 ✗ ERROR: version mismatch
```

**The version is auto-incremented by a database trigger**, so application code can't accidentally forget to bump it. The app's UPDATE query includes `WHERE version = $expected` and checks `rows_affected = 0` for conflicts.

### Why audit_log is append-only (and enforced)

The audit log is the source of truth for "what happened?". If it can be modified, it's no longer trustworthy. A company with compliance requirements (SOC2, ISO 27001) will audit this — and auditors will specifically ask if the log is immutable.

**Enforcement**: A database trigger raises an exception on any UPDATE or DELETE attempt on the `audit_log` table. This is stronger than a code convention — it survives bugs, rogue queries, and direct DB access.

At code level: only INSERT, never UPDATE or DELETE. At DB level: if it grows too large, we partition by month (Postgres 16 does this well with declarative partitioning). We move old partitions to cheaper storage. But we never delete.

### Why hash + prefix for API keys

When a user creates an API key, we show `fs_live_a3b9d2c8e4f1...` ONCE. Then we only store:

- `key_hash`: SHA-256 of the full key (for validating requests)
- `key_prefix`: first ~12 characters (`fs_live_a3b9`) so the admin can identify "ah, this is the prod key for the backend"

If the database leaks, the keys can't be recovered. The user regenerates them.

**Why SHA-256 and not bcrypt?** API keys are random, high-entropy strings (not human-chosen passwords). SHA-256 is appropriate for this: it's fast (intentionally — we hash on every request) and the input space is too large to brute-force. User passwords use bcrypt/argon2id — see [Authentication](#authentication--authorization).

### Schema protections (added post-review)

The following protections were added after a security review:

| Protection | Implementation | Why |
|---|---|---|
| Audit log immutability | `BEFORE UPDATE OR DELETE` trigger raises exception | SOC2/ISO 27001 compliance |
| Version auto-increment | `BEFORE UPDATE` trigger on `flag_environments` | Prevents OCC bypass from code bugs |
| Slug regex fix | `'^[a-z0-9]([a-z0-9-]*[a-z0-9])?$'` | Now accepts single-char slugs like `a` |
| pgcrypto removal | Dropped `CREATE EXTENSION pgcrypto` | Unnecessary since Postgres 13+ |
| `created_at` on `flag_environments` | Added `TIMESTAMPTZ NOT NULL DEFAULT NOW()` | Consistency; know when config was first created |

### Initial Schema (Migration 000001)

The first migration (`migrations/000001_initial.up.sql`) creates all core tables. Summary:

| Table | Purpose | Key columns |
|---|---|---|
| `tenants` | Top-level isolation unit | `id`, `slug`, `name` |
| `users` | Human accounts (dashboard) | `id`, `email` (CITEXT), `password_hash` |
| `tenant_members` | User-to-tenant membership + role | `user_id`, `tenant_id`, `role` |
| `sessions` | JWT refresh token storage | `user_id`, `tenant_id`, `refresh_hash`, `expires_at` |
| `projects` | Grouping of flags within a tenant | `id`, `tenant_id`, `slug`, `name` |
| `environments` | dev/staging/prod per project | `id`, `project_id`, `slug`, `name` |
| `api_keys` | Machine auth, scoped to environment | `key_hash`, `key_prefix`, `environment_id`, `revoked_at` |
| `flags` | Flag definition (project-level) | `id`, `project_id`, `key`, `type`, `archived_at` |
| `flag_environments` | Per-env flag config + rules | `flag_id`, `environment_id`, `enabled`, `rules` (JSONB), `version` |
| `segments` | Reusable user groups | `id`, `project_id`, `key`, `conditions` (JSONB) |
| `audit_log` | Append-only change history | `id`, `tenant_id`, `actor_id`, `action`, `payload` (JSONB) |

Database-level protections in the same migration:
- `BEFORE UPDATE OR DELETE` trigger on `audit_log` → raises exception (immutability)
- `BEFORE UPDATE` trigger on `flag_environments` → auto-increments `version`
- Partial index on `api_keys WHERE revoked_at IS NULL` → fast auth lookups
- GIN index on `flag_environments(rules)` → future JSONB queries if needed

### Zero-downtime Migration Strategy

All migrations follow the **expand-contract pattern** to avoid table locks during rolling deploys:

| Phase | Operation | Safe? |
|---|---|---|
| **Expand** | Add nullable column, add table, add index `CONCURRENTLY` | Yes — old code ignores new additions |
| **Migrate** | Backfill data in batches | Yes — background, no locks |
| **Contract** | Drop old column/table, add `NOT NULL` constraint | Yes — only after old code is fully deployed |

**Rules enforced on every migration**:
1. Never rename a column or table in a single step — add new, backfill, drop old.
2. Never add a `NOT NULL` column without a `DEFAULT` in the same migration.
3. Always use `CREATE INDEX CONCURRENTLY` (non-blocking). `golang-migrate` must run this outside a transaction — use the `-- migrate: no-transaction` annotation.
4. Migrations that scan > 1M rows must run as a separate backfill job, not inline.

**Multi-instance concurrency**: `golang-migrate` uses a Postgres advisory lock (`pg_advisory_lock`) so only one instance runs migrations at a time. Others wait and detect the schema is already up-to-date when the lock is released.

**Recommended rolling deploy sequence**:
```
1. Run migration (expand phase) — new structure exists, old code still works
2. Deploy new application code — reads and writes new structure
3. Run contract migration — drops old structure once no old code is running
```

---

## Rule Evaluation Engine

> ![Flag Evaluation Flow](./diagrams/diagram-flow-eval.png)
>
> For the complete SDK → API → Redis → Postgres → OTel sequence, see [`diagrams/flagstone-overview.svg`](./diagrams/flagstone-overview.svg).

This is the algorithmically rich part of the system. Designed in layers:

```
Input: (flag_key, environment_id, user_context)
   ↓
1. In-memory cache lookup (process-local)
   ↓ miss
2. Load flag config from Redis
   ↓ miss
3. Load from Postgres
   ↓
4. Evaluate rules in order (first match wins)
   ↓
5. If no rule matches → default value
   ↓
6. If match has rollout → consistent hashing
   ↓
Output: (value, reason, rule_matched_index)
```

### Data structures used

**Rule tree (N-ary tree)**: Rules are trees of boolean expressions with `all` (AND), `any` (OR), and `not` nodes. Evaluation is a DFS tree traversal with short-circuit evaluation (stop at first `false` for AND, first `true` for OR). Complexity: O(n) where n = number of nodes. For typical rules (5-20 conditions), this is nanoseconds.

**In-memory cache (hash map with TTL)**: A thread-safe `map[string]*FlagConfig` protected by `sync.RWMutex`. Key format: `env_id:flag_key`. Multiple concurrent readers (evaluations), exclusive writer (cache invalidation). TTL: 30 seconds as fallback.

**Consistent hashing for rollouts (FNV-1a + modular arithmetic)**: Deterministic bucket assignment using `hash(flag_key + ":" + user_id) % 100`. Same user always gets the same result. Including `flag_key` prevents the same users from always being in the lucky percentage across all flags.

### Why rule order matters

Rules evaluate **in order** and the first match wins. This lets you express precedences:

```
Rule 1: "If admin → always true"
Rule 2: "If in segment beta-testers → true"
Rule 3: "Rollout 10% of the rest → true"
Default: false
```

If rules had no order and were evaluated as OR, we couldn't guarantee admins ALWAYS see the feature, because the 10% rollout could give them `false`.

### Consistent hashing for rollouts

When we say "10% of users", the problem is: which 10%? And more importantly: **the same 10% every time?**

Solution: hash of `(flag_key + user_id)`, modulo 100. If the result is `< 10`, the user is in the group. Since the hash is deterministic, the same user is always in or always out.

```go
func inRollout(flagKey, userID string, percentage int) bool {
    h := fnv.New32a()
    h.Write([]byte(flagKey + ":" + userID))
    bucket := h.Sum32() % 100
    return bucket < uint32(percentage)
}
```

**Key property**: when you increase from 10% to 25%, users who were already in the 10% stay in. Only new users are added to the group. This is because `bucket < 10` is a subset of `bucket < 25`.

### Why FNV and not MurmurHash or xxHash

For rollouts we don't need cryptographic resistance or perfect distribution. We need it to be fast and reproducible. FNV is in Go's standard library and is sufficient. If benchmarks ever show it's the bottleneck (unlikely), we swap it.

### Rules Schema Specification

The `rules` column in `flag_environments` is a JSONB array. Each element is a **rule object** evaluated in order (first match wins). The formal spec:

```jsonc
// flag_environments.rules :: Rule[]
[
  {
    "conditions": <ConditionNode>,   // required — the boolean expression tree
    "rollout": <RolloutConfig>,       // optional — if absent, match = 100% on
    "value": true                     // optional — override value on match (default: true)
  }
]
```

#### ConditionNode types

A `ConditionNode` is one of four shapes:

```jsonc
// 1. Leaf — single attribute comparison
{
  "attribute": "country",       // string, required — key in user context
  "op": "eq",                   // string, required — operator (see table below)
  "value": "AR"                 // any, required — comparison target
}

// 2. AND — all children must match
{
  "all": [ <ConditionNode>, <ConditionNode>, ... ]   // min 1 child
}

// 3. OR — at least one child must match
{
  "any": [ <ConditionNode>, <ConditionNode>, ... ]   // min 1 child
}

// 4. NOT — single child, result inverted
{
  "not": <ConditionNode>
}
```

#### Supported operators

| Operator | Type | Description | Example value |
|---|---|---|---|
| `eq` | any | Exact equality (type-coerced) | `"AR"`, `true`, `42` |
| `neq` | any | Not equal | `"AR"` |
| `gt` | number | Greater than | `18` |
| `gte` | number | Greater than or equal | `18` |
| `lt` | number | Less than | `100` |
| `lte` | number | Less than or equal | `100` |
| `in` | array | Value is in the list | `["AR", "BR", "CL"]` |
| `not_in` | array | Value is not in the list | `["CN", "RU"]` |
| `contains` | string | Substring match | `"premium"` |
| `starts_with` | string | Prefix match | `"user_"` |
| `ends_with` | string | Suffix match | `"@company.com"` |
| `matches` | string | Regex match (RE2 syntax) | `"^v[0-9]+$"` |
| `exists` | - | Attribute is present in context | `true` (ignored) |
| `not_exists` | - | Attribute is absent from context | `true` (ignored) |
| `segment` | string | User matches named segment | `"beta-testers"` |

**Type coercion rules**: The engine compares as the type of `value` in the rule. If the user context attribute cannot be coerced (e.g., `"abc"` vs numeric `gt`), the condition evaluates to `false` — never errors.

**`segment` operator**: The `value` is a segment key. The engine looks up the segment in a **pre-loaded map** passed in by the caller (see [Eager loading: avoiding N+1 queries](#eager-loading-avoiding-n1-queries)) and evaluates its condition tree recursively in-memory. The engine never performs I/O — that is the responsibility of the storage layer. Circular references (segment A → B → A) are detected by maintaining a visited set during traversal; if a cycle is found, the condition evaluates to `false` and a warning is logged.

**`matches` operator**: Uses Go's `regexp.MatchString` (RE2 syntax). The pattern is compiled once and cached. Invalid regex patterns cause the condition to evaluate to `false` + error log (never panic).

#### Validation rules (enforced on write via API)

1. `conditions` must be a valid `ConditionNode` (recursively)
2. Maximum tree depth: 10 levels (prevents abuse)
3. Maximum total nodes per rule: 50 (prevents combinatorial explosion)
4. `op` must be one of the supported operators
5. `segment` references must exist at write time (warning, not error — segments can be deleted later)
6. `rollout.percentage` must be 0-100 if present
7. `rules` array maximum length: 100 rules per flag-environment

### Evaluation Flow (complete)

The full evaluation path from SDK request to response, with every decision point:

```
SDK Request: POST /api/v1/evaluate/flags/:key
  Headers: Authorization: Bearer fs_live_...
  Body: { "context": { "user_id": "u123", "country": "AR", "plan": "premium" } }

Step 1: AUTH
  ├─ Extract API key from Authorization header
  ├─ SHA-256(key) → lookup in api_keys table
  ├─ Fail? → 401 Unauthorized (generic message, no key details)
  ├─ Revoked/expired? → 401 Unauthorized
  └─ Success → extract environment_id, tenant_id from key record

Step 2: RESOLVE FLAG
  ├─ key = "env_id:flag_key"
  ├─ Check in-memory cache → hit? use it (if not expired)
  ├─ Check Redis cache → hit? use it, populate in-memory
  ├─ Query Postgres: flag + flag_environment + rules
  ├─ Flag not found? → return { value: false, reason: "FLAG_NOT_FOUND" }
  ├─ Flag archived? → return { value: false, reason: "FLAG_ARCHIVED" }
  └─ Populate both caches, continue

Step 3: CHECK ENABLED
  ├─ flag_environment.enabled = false?
  │   → return { value: false, reason: "DISABLED" }
  └─ enabled = true → continue to rules

Step 4: EVALUATE RULES (in order, first match wins)
  For each rule in flag_environment.rules:
  │
  ├─ Evaluate rule.conditions (recursive tree walk)
  │   ├─ Leaf node: compare context[attribute] with value using op
  │   │   ├─ Attribute missing from context?
  │   │   │   ├─ op = "exists" → false
  │   │   │   ├─ op = "not_exists" → true
  │   │   │   └─ any other op → false (missing = no match)
  │   │   ├─ Type mismatch? → false (never error)
  │   │   └─ op = "segment"? → load segment, evaluate recursively
  │   ├─ "all" node: short-circuit AND (stop at first false)
  │   ├─ "any" node: short-circuit OR (stop at first true)
  │   └─ "not" node: invert child result
  │
  ├─ Conditions matched?
  │   ├─ No → continue to next rule
  │   └─ Yes → check rollout
  │       ├─ No rollout config? → return { value: rule.value, reason: "RULE_MATCH", rule_index: i }
  │       └─ Has rollout?
  │           ├─ No user_id in context? → rollout treated as NOT matched (skip rule), warning metric
  │           ├─ hash(flag_key + ":" + user_id) % 100 < percentage?
  │           │   ├─ Yes → return { value: rule.value, reason: "RULE_MATCH", rule_index: i }
  │           │   └─ No → continue to next rule (NOT default — next rule gets a chance)
  │           └─ (rollout miss falls through to next rule)
  │
  └─ No rules matched → return { value: flag_environment.default_value ?? flag.default_value ?? false, reason: "DEFAULT" }

Step 5: EMIT TELEMETRY (non-blocking, after response)
  ├─ OTel span: flag.key, flag.value, flag.reason, user.id, environment
  ├─ Prometheus counter: flagstone_evaluations_total{flag, env, result}
  └─ Prometheus histogram: flagstone_evaluation_duration_seconds
```

**Critical behavior: rollout miss falls through.** When a rule's conditions match but the user is outside the rollout percentage, evaluation continues to the next rule. This allows stacking:

```jsonc
// Rule 1: 100% for admins (no rollout = always)
// Rule 2: 50% rollout for everyone
// If admin is outside the 50%, Rule 1 catches them first.
// If non-admin is outside the 50%, they fall to default (false).
```

**No `user_id` in context for rollout**: if the SDK doesn't send a `user_id` and a rule has a rollout, the engine treats the rollout as **not matched** — the rule is skipped and evaluation continues to the next rule (or falls through to default). The metric `engine_warnings_total{type="rollout_without_user_id"}` is incremented and a warning is logged with the flag key.

Why not use `rand.Intn(100)`? It's non-deterministic — the same evaluation would flicker between true/false on consecutive requests, causing visible UI churn (features appearing and disappearing as the user navigates). Returning a deterministic "not in rollout" is worse for the developer's intent (less coverage than they wanted) but better for the end user (no flicker), and the warning metric makes the misconfiguration loud. The SDK MUST send `user_id` for rollouts to take effect; this is the contract.

Why not hash IP + User-Agent as a session-ish fallback? IP changes (mobile networks, VPN), UA changes (browser updates), and this would introduce privacy concerns (storing PII in evaluation context). It's the SDK's job — not the server's — to persist an anonymous session ID (cookie, localStorage) if the app has no logged-in user. A future SDK option (`WithAnonymousID`) can automate this.

### Error Policy: Resilience over Correctness

**Core principle: the evaluation engine never returns an error to the SDK. It always returns a value.**

This is a deliberate design choice. Feature flags control live production behavior. If the engine returns an error, the SDK has to decide what to do — and most developers will handle it poorly (crash, log and ignore, default to a random value). Instead, we degrade gracefully:

| Failure scenario | Engine behavior | Telemetry |
|---|---|---|
| Flag not found | Return `false` + reason `FLAG_NOT_FOUND` | Warning log, counter increment |
| Flag archived | Return `false` + reason `FLAG_ARCHIVED` | Info log |
| Flag disabled | Return `false` + reason `DISABLED` | Normal (expected path) |
| Malformed rule JSON | Skip rule, continue to next | Error log + `engine_errors_total{type="malformed_rule"}` |
| Unknown operator | Condition → `false`, continue | Error log + `engine_errors_total{type="unknown_op"}` |
| Segment not found | Condition → `false`, continue | Warning log + `engine_errors_total{type="segment_missing"}` |
| Segment circular ref | Condition → `false`, continue | Error log + `engine_errors_total{type="segment_cycle"}` |
| Regex compile fail | Condition → `false`, continue | Error log + `engine_errors_total{type="bad_regex"}` |
| Type coercion fail | Condition → `false`, continue | Debug log (high volume, low severity) |
| Redis down | Fall through to Postgres | Warning log, `redis_fallback_total` counter |
| Postgres down | Return cached value if available, else `false` | Critical log + alert |
| Panic in eval | Recover, return `false` + reason `INTERNAL_ERROR` | Critical log + stack trace |

**Default value resolution order**: `flag_environment.default_value` (per-env override) → `flag.default_value` (project-level default) → `false` (hardcoded fallback). Both columns exist in the schema from day one so the engine always reads them — no code change needed when multivariate variants are added.

**Panic recovery**: Every evaluation runs inside a `defer func() { recover() }()`. A bug in rule evaluation must never crash the process. The recovered panic is logged with full stack trace and the evaluation returns `false`.

### Rollout Configuration Spec

The `rollout` field in a rule object controls gradual rollout. The formal spec:

```jsonc
// Rule.rollout :: RolloutConfig | null
{
  "percentage": 25,              // required — 0-100, integer
  "seed": "experiment-q4-2024"   // optional — override hash seed (default: flag_key)
}
```

#### Fields

**`percentage`** (integer, 0-100): The percentage of users who pass the rollout gate. The engine computes `hash(seed + ":" + user_id) % 100 < percentage`.

- `0` = nobody passes (equivalent to no rule match)
- `100` = everyone passes (equivalent to no rollout config)
- Values outside 0-100 are rejected at write time by the API

**`seed`** (string, optional): Overrides the default hash seed (`flag_key`). Use cases:

1. **Correlated rollouts**: Two flags with the same seed will include the same users at the same percentage. Useful for features that must ship together.
2. **Decorrelated rollouts**: Two flags with different seeds will have independent user selection. This is the default behavior (each flag uses its own key as seed).
3. **Re-rolling**: If a rollout at 10% had bad results and you want a different 10%, change the seed. The hash picks a different bucket of users.

#### Hash function detail

```go
func inRollout(seed, userID string, percentage int) bool {
    if percentage >= 100 {
        return true
    }
    if percentage <= 0 {
        return false
    }
    h := fnv.New32a()
    h.Write([]byte(seed + ":" + userID))
    bucket := h.Sum32() % 100
    return bucket < uint32(percentage)
}
```

The `seed` defaults to `flag_key` if not specified in the rollout config.

#### Monotonic rollout guarantee

When increasing percentage from N to M (where M > N): all users who were in the N% group remain in the M% group. This is because the hash is deterministic and `bucket < N` is a strict subset of `bucket < M`.

When **decreasing** percentage: users are removed from the group. This is expected behavior but should be documented clearly in the dashboard UI ("reducing rollout will remove users from the feature").

#### Full rule example with rollout

```json
[
  {
    "conditions": { "all": [
      { "attribute": "country", "op": "in", "value": ["AR", "BR", "CL"] },
      { "attribute": "plan", "op": "neq", "value": "free" }
    ]},
    "rollout": { "percentage": 25 },
    "value": true
  },
  {
    "conditions": { "attribute": "is_internal", "op": "eq", "value": true },
    "value": true
  }
]
```

This reads: "25% of paid users in LATAM get the feature. All internal users get it regardless."

### Eager loading: avoiding N+1 queries

The engine never performs I/O. All data needed for an evaluation must be loaded by the storage layer **in batch** before the engine runs. This prevents the classic N+1 pattern.

#### Bulk flag evaluation: a single JOIN

For `POST /api/v1/evaluate/flags` (all flags for an environment), one Postgres query returns everything:

```sql
SELECT f.id, f.key, f.type, f.default_value AS flag_default,
       fe.enabled, fe.rules, fe.default_value AS env_default, fe.version
FROM flags f
JOIN flag_environments fe ON fe.flag_id = f.id
WHERE fe.environment_id = $1
  AND f.archived_at IS NULL
```

One query, regardless of whether there are 5 or 500 flags. The result is then evaluated entirely in memory.

#### Segment resolution: preload the project's segments

Rules can reference segments by key. If the engine were to load segments on-demand during evaluation, a flag with 5 segment references would cause 5 extra queries per evaluation — the N+1 nightmare.

**Solution: preload all segments for the project in one query**, into a `map[SegmentKey]*Segment` passed to the engine as part of the evaluation input.

```sql
SELECT id, key, rules
FROM segments
WHERE project_id = $1 AND archived_at IS NULL
```

Why load **all** project segments rather than only referenced ones? Two reasons:

1. **Simplicity**: collecting referenced segment keys from JSONB rules requires walking the tree before querying. Loading all is one fewer round of pre-processing.
2. **Transitive references**: segments can reference other segments (A → B → C). The naive `WHERE key = ANY($keys)` only returns directly-referenced segments, missing transitive ones. A loop with multiple query rounds or a recursive CTE would handle this, but loading the full project is simpler and the data is tiny (typically <100 segments × 1 KB = 100 KB).

If a project ever grows to thousands of segments, we revisit and switch to BFS-based loading of only the referenced subgraph.

#### Cached at the bundle level, not per-flag

The cache key is `env_id` (not `env_id:flag_key`). The cached value is the full evaluation bundle:

```go
type EnvironmentBundle struct {
    Flags    []*FlagConfig
    Segments map[SegmentKey]*Segment
    LoadedAt time.Time
}
```

This way, both single-flag and bulk evaluations share the same cache entry. Cache invalidation purges the entire environment's bundle when any flag or segment in it changes — pessimistic but simple, and the next request reloads everything in one query.

#### Engine interface (pure, no I/O)

```go
type EvaluateRequest struct {
    Flag     *FlagConfig
    Segments map[SegmentKey]*Segment  // preloaded by caller
    Context  map[string]any            // user attributes
}

type Engine interface {
    Evaluate(ctx context.Context, req EvaluateRequest) EvaluateResult
    EvaluateAll(ctx context.Context, flags []*FlagConfig, segments map[SegmentKey]*Segment, ctxAttrs map[string]any) map[FlagKey]EvaluateResult
}
```

The engine is deterministic, pure, and trivially unit-testable: no DB mock needed.

---

## Authentication & Authorization

> ![Dual Auth Flow](./diagrams/diagram-auth.png)

### Dual auth model

Flagstone has two distinct types of callers with different security requirements:

| Caller | Auth method | Token lifecycle | Scope |
|---|---|---|---|
| **SDKs / machines** | API Key (in `Authorization` header) | Long-lived, manually rotated | Single environment |
| **Dashboard users** | Email + password → JWT | Short-lived access + refresh | Tenant-scoped, role-based |

### API Key authentication (SDKs)

```
SDK request:
  Authorization: Bearer fs_live_a3b9d2c8e4f1...
                        ↓
Server:
  1. SHA-256(key) → lookup key_hash in DB
  2. Check revoked_at IS NULL
  3. Check expires_at IS NULL OR expires_at > NOW()
  4. Resolve environment_id from the key
  5. All subsequent queries scoped to that environment
```

**Why SHA-256?** API keys are 32+ bytes of random data. They're not human-chosen passwords vulnerable to dictionary attacks. SHA-256 is appropriate because:
- Input entropy is high enough that brute-force is infeasible
- We hash on every request — bcrypt's intentional slowness would add ~100ms per evaluation
- The partial index on `api_keys WHERE revoked_at IS NULL` keeps lookups fast

**Key format**: `fs_{environment}_{random}` where:
- `fs_` = Flagstone prefix (identifies the key type in logs/configs)
- `{environment}` = `live` or `test` (human-readable hint, NOT relied upon for security)
- `{random}` = 32 bytes of `crypto/rand`, base62 encoded

### Dashboard authentication (humans)

```
Login flow:
  1. POST /api/v1/auth/login { email, password }
  2. Server verifies email exists
  3. bcrypt.CompareHashAndPassword(stored_hash, password)
  4. Generate JWT access token (15 min TTL)
  5. Generate opaque refresh token (7 day TTL, stored in DB)
  6. Return { access_token, refresh_token }
  7. Client stores access_token in memory, refresh_token in httpOnly cookie

Token refresh:
  1. POST /api/v1/auth/refresh (refresh_token from cookie)
  2. Server validates refresh token exists and is not expired
  3. Rotate: delete old refresh token, issue new pair
  4. Return new { access_token, refresh_token }
```

**Why bcrypt for passwords but SHA-256 for API keys?** Passwords are human-chosen and potentially weak. Bcrypt's work factor (cost=12, ~250ms) makes brute-force attacks impractical. API keys are machine-generated with full entropy — no need for the slow hash.

**Why JWTs?** Stateless verification — the server doesn't need to hit the DB on every request. The JWT contains: `user_id`, `tenant_id`, `role`, `exp`. Signed with HS256 using a server secret.

**Why refresh tokens in DB?** To support:
- Revocation (logout invalidates the refresh token)
- Session listing ("you're logged in from 3 devices")
- Token rotation (prevents refresh token replay attacks)

### Role-Based Access Control (RBAC)

The `tenant_members` table assigns a role per user per tenant:

| Role | Flags | Environments | API Keys | Members | Billing |
|---|---|---|---|---|---|
| `viewer` | Read | Read | - | - | - |
| `member` | Read/Write | Read | Read | - | - |
| `admin` | Full | Full | Full | Manage | - |
| `owner` | Full | Full | Full | Full | Full |

Authorization is checked in middleware after authentication:

```go
// Middleware chain for a protected endpoint:
// 1. AuthN: extract + validate JWT → set user_id, tenant_id, role in context
// 2. AuthZ: check role >= required_role for this endpoint
// 3. Handler: business logic, already scoped to the authenticated tenant
```

A user can belong to multiple tenants (consultant, agency dev). The JWT specifies which tenant is "active" for this session.

### Session table

JWT refresh tokens are stored in the `sessions` table (created by migration `000001_init.up.sql`):

```sql
CREATE TABLE sessions (
    id            UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id       UUID         NOT NULL REFERENCES users(id)   ON DELETE CASCADE,
    tenant_id     UUID         NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    refresh_hash  CHAR(64)     NOT NULL UNIQUE,  -- SHA-256 of refresh token
    user_agent    TEXT,
    ip_address    INET,
    expires_at    TIMESTAMPTZ  NOT NULL,
    created_at    TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX sessions_user_idx        ON sessions(user_id);
CREATE INDEX sessions_expires_at_idx  ON sessions(expires_at);
```

Why `sessions` lives in the initial migration: the `POST /api/v1/setup` endpoint (Milestone 1) returns an access token + refresh token, so the table must exist from day one. Dashboard auth in general (login/refresh/logout) is also Milestone 1 because flag CRUD endpoints need RBAC-protected JWT auth from the first endpoint that's not API-key-authenticated.

A background goroutine periodically deletes expired sessions: `DELETE FROM sessions WHERE expires_at < NOW()`. The `sessions_expires_at_idx` index makes this efficient.

### Onboarding & Bootstrap Flow

A fresh self-hosted Flagstone installation needs a one-time setup before any tenant exists.

#### Bootstrap endpoint

```
POST /api/v1/setup
(no auth — only works when zero tenants exist)

Body:
{
  "tenant_name": "Acme Corp",
  "admin_email": "admin@acme.com",
  "admin_password": "..."
}

Response 201 Created:
{
  "tenant_id": "...",
  "user_id": "...",
  "access_token": "...",
  "refresh_token": "..."
}

Response 409 Conflict (any tenant already exists):
{
  "error": { "code": "ALREADY_INITIALIZED", "message": "This instance is already set up." }
}
```

`/api/v1/setup` is the only unauthenticated write endpoint in the system. It returns `409` if any tenant exists, preventing unauthorized bootstrap on a running installation.

#### Project & environment creation

After bootstrap, the owner creates the first project. Projects automatically receive three default environments (`development`, `staging`, `production`) — created by application code on `POST /api/v1/projects`, not by a DB trigger, to keep the schema simple.

```
POST /api/v1/projects
Authorization: Bearer <access_token>

Body: { "name": "My App", "slug": "my-app" }

Response 201:
{
  "id": "...",
  "slug": "my-app",
  "environments": [
    { "id": "...", "slug": "development" },
    { "id": "...", "slug": "staging" },
    { "id": "...", "slug": "production" }
  ]
}
```

#### Full first-run sequence

```
1. POST /api/v1/setup        → creates tenant + owner user + returns session
2. POST /api/v1/projects     → creates first project + default environments
3. POST /api/v1/api-keys     → creates SDK key scoped to "production" environment
4. SDK configured with key   → ready to evaluate flags
```

Total steps from zero to first evaluation: 4 API calls.

---

## Caching & Propagation

> ![Change Propagation](./diagrams/diagram-pub.png)

### Three-level cache

```
1. In-memory cache (Go process)     ← O(1) lookup, ~microseconds
   ↓ miss
2. Redis (distributed cache)         ← O(1) lookup, ~1ms network
   ↓ miss
3. PostgreSQL (source of truth)      ← Query, ~5-10ms
```

### Cache invalidation

The classic problem: when an admin changes a flag, how long until it propagates?

Strategy:

1. UPDATE in Postgres (within a transaction that also INSERTs the audit log)
2. PUBLISH on Redis pub/sub channel `flag_changes`
3. Each subscribed server receives the message and purges its local cache for that flag
4. Each SDK connected via SSE receives the change event

Total time: typically <100ms. If Redis pub/sub fails, caches have a 30-second TTL as fallback — worst case, changes take 30 seconds.

**Stale reads are acceptable.** A flag that takes 2 seconds to propagate doesn't break anything. A flag that returns inconsistent results (sometimes yes, sometimes no for the same user in the same second) does break things. We aim for eventual consistency with low latency, not strong consistency.

**Publish-after-commit**: the Redis `PUBLISH` for cache invalidation must happen only after the Postgres transaction successfully commits. Publishing before commit means subscribers attempt to fetch a flag version that doesn't exist yet (the write may still be in-flight or may roll back). Required pattern:

```go
tx, _ := pool.BeginTx(ctx, pgx.TxOptions{})
// UPDATE flag_environments ... INSERT INTO audit_log ...
if err := tx.Commit(ctx); err != nil {
    return err
}
// Only publish after successful commit — never before
redis.Publish(ctx, "flag_changes", event)
```

If `PUBLISH` fails after a successful commit, the TTL-based eviction (30 seconds) is the fallback — not a correctness problem.

**Cache warming: lazy by design.** The server does NOT pre-load all flags on startup. The first evaluation for each flag pays the full Postgres latency; subsequent calls hit in-memory cache. This keeps startup time predictable and avoids a write storm on Postgres when multiple instances restart simultaneously. The `readyz` `"cache": "cold"` status communicates this to the load balancer during warm-up.

### In-memory cache structure

```go
type CacheEntry struct {
    Config    *FlagConfig
    ExpiresAt time.Time
}

type FlagCache struct {
    mu      sync.RWMutex
    entries map[string]*CacheEntry  // key: "env_id:flag_key"
    ttl     time.Duration           // 30 seconds default
}
```

**Thread safety**: `sync.RWMutex` allows multiple concurrent readers (flag evaluations) but exclusive writes (cache invalidation). Alternative: `sync.Map` for read-heavy workloads (which is exactly this case — reads vastly outnumber writes).

**Eviction**: A background goroutine runs every `ttl/2` and removes expired entries. Lazy eviction on read is also checked (belt and suspenders).

**Size bound**: the cache is capped at `max_entries` (default: 10,000 flag-environment pairs). When the limit is reached, the least-recently-used entry is evicted. Without a bound, servers with many environments and flags could exhaust heap memory over time. For typical deployments (50 flags × 5 environments = 250 entries) this limit is never reached.

**Cache stampede prevention (`singleflight`)**: when an in-memory cache entry expires for a hot flag, many concurrent goroutines may simultaneously attempt to fetch from Redis/Postgres — the thundering herd problem. `golang.org/x/sync/singleflight` deduplicates: only one goroutine executes the lookup, the rest wait and share the result.

```go
var sf singleflight.Group

func (c *memoryCachedStore) GetFlagConfig(ctx context.Context, envID EnvironmentID, key FlagKey) (*FlagConfig, error) {
    cacheKey := string(envID) + ":" + string(key)
    if entry, ok := c.get(cacheKey); ok {
        return entry, nil
    }
    v, err, _ := sf.Do(cacheKey, func() (any, error) {
        return c.next.GetFlagConfig(ctx, envID, key)
    })
    if err != nil {
        return nil, err
    }
    cfg := v.(*FlagConfig)
    c.set(cacheKey, cfg)
    return cfg, nil
}
```

### SSE fan-out

```go
type SSEHub struct {
    clients    map[string]chan Event  // client_id → channel
    register   chan Client
    unregister chan Client
    broadcast  chan Event
    mu         sync.RWMutex
}
```

Each connected SSE client has its own Go channel backed by a goroutine that writes to the HTTP response. When a change arrives, the hub iterates over all clients and sends the event to each channel. Graceful shutdown drains all client connections.

### SDK Bootstrap Race Condition

The naive SDK startup sequence has an unavoidable race window:

```
T=0ms:   SDK → POST /evaluate/flags   (snapshot of all flag values)
T=50ms:  Flag "checkout-v2" changes on server  ← MISSED
T=100ms: SDK → GET /stream            (starts receiving SSE events)
```

The change at T=50ms is never delivered: the bulk eval happened before it, and the SSE connection opened after. The SDK has stale state indefinitely.

**Solution: Last-Event-ID + server-side event log**

Each SSE event carries a monotonically increasing integer ID. The server maintains a short-lived event log in Redis (capped list, TTL 30 seconds, keyed by environment). On connect or reconnect, the SDK sends `Last-Event-ID: <id>` and the server replays all events after that ID.

**Corrected startup sequence**:
```
1. SDK → GET /stream?last_event_id=0   (open SSE first — server starts buffering)
2. SDK → POST /evaluate/flags           (bulk eval)
3. Server replays any events since ID 0 over the already-open SSE connection
4. SDK applies replayed events on top of the bulk eval snapshot
```

By opening the SSE connection before the bulk fetch, the SDK guarantees no events fall in the gap.

**Fallback (event log expired)**: If `last_event_id` points to an event older than 30 seconds (or is absent), the server emits a `resync` event instructing the SDK to re-fetch all flags via `POST /evaluate/flags`.

**Event log Redis structure**:
```
Key:   sse:events:{environment_id}   (Redis list, capped at 500 entries via LTRIM)
Value: JSON-encoded SSE event
TTL:   30 seconds (refreshed on every new event)
```

---

## API Design

> ![API Design & Health Check](./diagrams/diagram-design-api.png)

### URL structure

```
/api/v1/                          # Versioned API root
├── setup/
│   └── POST   /                   # One-shot bootstrap (no auth; 409 if already initialized)
├── projects/
│   ├── GET    /                   # List projects in tenant
│   ├── POST   /                   # Create project (auto-creates default environments)
│   ├── GET    /:slug              # Get project details
│   └── PUT    /:slug              # Update project name/slug
├── auth/
│   ├── POST   login              # Email + password → JWT
│   ├── POST   refresh            # Refresh token → new JWT pair
│   └── POST   logout             # Invalidate refresh token
├── flags/
│   ├── GET    /                   # List flags (for dashboard)
│   ├── POST   /                   # Create flag
│   ├── GET    /:key               # Get flag details
│   ├── PUT    /:key               # Update flag definition
│   ├── DELETE /:key               # Soft-delete (archive) flag
│   └── PUT    /:key/environments/:env  # Update per-env config
├── segments/
│   ├── GET    /                   # List segments
│   ├── POST   /                   # Create segment
│   ├── GET    /:key               # Get segment
│   ├── PUT    /:key               # Update segment
│   └── DELETE /:key               # Archive segment
├── environments/
│   ├── GET    /                   # List environments
│   └── POST   /                   # Create environment
├── api-keys/
│   ├── GET    /                   # List keys (prefix only)
│   ├── POST   /                   # Create key (returns full key ONCE)
│   └── DELETE /:id                # Revoke key
├── audit/
│   └── GET    /                   # Query audit log
└── evaluate/
    ├── POST   /flags/:key         # Evaluate single flag (SDK)
    └── POST   /flags              # Evaluate all flags (SDK bootstrap)

/healthz                           # Health check (unauthenticated)
/stream                            # SSE endpoint (API key auth)
```

### Auth scoping

- **SDK endpoints** (`/evaluate/*`, `/stream`): API Key auth → scoped to single environment
- **Dashboard endpoints** (everything else): JWT auth → scoped to tenant, role-checked
- **Health check**: unauthenticated

### Error format

```json
{
  "error": {
    "code": "FLAG_NOT_FOUND",
    "message": "Flag 'checkout-v2' does not exist in this project",
    "request_id": "req_abc123"
  }
}
```

Consistent error codes allow SDKs to handle specific failures programmatically.

### Evaluation API Contracts

These are the most critical endpoints — they're in the hot path of every SDK request. Specified here so SDK authors and the engine implementation share the same contract.

#### Single flag evaluation

```
POST /api/v1/evaluate/flags/:key
Authorization: Bearer fs_live_...
Content-Type: application/json

Request body:
{
  "context": {                    // required — user/request attributes
    "user_id": "u_abc123",        // recommended — needed for consistent rollouts
    "country": "AR",              // arbitrary key-value pairs
    "plan": "premium",
    "is_admin": true,
    "app_version": "2.4.1"
  }
}

Success response (200 OK):
{
  "key": "new-checkout",
  "value": true,                  // boolean (MVP); will become any type with variants
  "reason": "RULE_MATCH",         // enum: see table below
  "rule_index": 0,                // which rule matched (-1 if none)
  "request_id": "req_d4f8a2"     // for debugging, correlates with OTel trace
}

Error responses:
  401 Unauthorized — invalid/revoked/expired API key
  {
    "error": {
      "code": "UNAUTHORIZED",
      "message": "Invalid or expired API key",
      "request_id": "req_e5c1b3"
    }
  }
```

Note: there is no 404 for flag not found. A missing flag returns `200 OK` with `value: false` and `reason: "FLAG_NOT_FOUND"`. This is intentional — see [Error Policy](#error-policy-resilience-over-correctness). SDKs should never crash because a flag doesn't exist yet.

#### Bulk evaluation (all flags)

Used by SDKs at startup to bootstrap their local cache with all flag values for the authenticated environment.

```
POST /api/v1/evaluate/flags
Authorization: Bearer fs_live_...
Content-Type: application/json

Request body:
{
  "context": {
    "user_id": "u_abc123",
    "country": "AR",
    "plan": "premium"
  }
}

Success response (200 OK):
{
  "flags": {
    "new-checkout": {
      "value": true,
      "reason": "RULE_MATCH",
      "rule_index": 0
    },
    "dark-mode": {
      "value": false,
      "reason": "DISABLED",
      "rule_index": -1
    },
    "beta-dashboard": {
      "value": false,
      "reason": "DEFAULT",
      "rule_index": -1
    }
  },
  "environment": "production",
  "evaluated_at": "2024-11-15T03:22:41Z",
  "request_id": "req_f7a2c4"
}
```

**Performance note**: Bulk evaluation loads ALL flags for the environment in a single Postgres query (or cache hit), then evaluates each against the provided context. This is O(F × R) where F = number of flags and R = average rules per flag. For typical deployments (50 flags, 5 rules each), this completes in <10ms.

**Scale consideration**: For environments with > 500 flags, bulk evaluation may take 50–100ms. At that scale, consider fetching only enabled flags (`enabled = true`) or a filtered key set. A `?keys=flag-a,flag-b` filter parameter is in scope for Milestone 3 but not required for MVP.

**SDK bootstrap flow** (race-condition-safe — see [SDK Bootstrap Race Condition](#sdk-bootstrap-race-condition)):
1. SDK opens SSE connection with `Last-Event-ID: 0` → server starts buffering events
2. SDK calls `POST /evaluate/flags` → gets all current values
3. Server replays any events that arrived during step 2 over the open connection
4. On subsequent SSE events → re-fetch the changed flag via `POST /evaluate/flags/:key`

#### Evaluation reason enum

| Reason | Meaning |
|---|---|
| `RULE_MATCH` | A rule's conditions matched (and rollout passed, if applicable) |
| `DEFAULT` | No rule matched; returned the default value |
| `DISABLED` | Flag exists but `enabled = false` in this environment |
| `FLAG_NOT_FOUND` | Flag key doesn't exist in this project |
| `FLAG_ARCHIVED` | Flag was soft-deleted |
| `INTERNAL_ERROR` | Engine panicked and recovered (should never happen) |

#### Context requirements

The `context` object is a flat `map[string]any`. Rules reference attributes by key. Constraints:

- **Maximum keys**: 100 (reject with 400 if exceeded — prevents abuse)
- **Maximum key length**: 128 characters
- **Maximum string value length**: 1024 characters
- **Allowed value types**: string, number (float64), boolean, array of strings
- **`user_id`**: not required, but mandatory for any flag whose rules use rollout. Without `user_id`, rollouts are treated as not-matched (the rule is skipped); a warning metric is emitted so the misconfiguration surfaces in dashboards.
- **Reserved keys**: none currently, but keys starting with `_fs_` are reserved for future engine use

### Health Check Design

Two health endpoints with different purposes:

#### Liveness: `/healthz`

```
GET /healthz

Response (200 OK):
{
  "status": "ok",
  "version": "0.1.0",
  "uptime_seconds": 3847
}
```

**Purpose**: "Is the process alive?" Used by container orchestrators (ECS, k8s) to decide whether to restart the process. This endpoint does NOT check dependencies — if Postgres is down but the process is alive, liveness still returns 200.

**Auth**: None. Must be callable by load balancers and orchestrators without credentials.

**Behavior**: Always returns 200 if the HTTP server is accepting connections. The only way this fails is if the process is dead or the listener is closed.

#### Readiness: `/readyz`

```
GET /readyz

Response when healthy (200 OK):
{
  "status": "ready",
  "checks": {
    "postgres": { "status": "up", "latency_ms": 2 },
    "redis": { "status": "up", "latency_ms": 1 },
    "cache": { "status": "warm", "entries": 142 }
  }
}

Response when degraded (503 Service Unavailable):
{
  "status": "not_ready",
  "checks": {
    "postgres": { "status": "down", "error": "connection refused" },
    "redis": { "status": "up", "latency_ms": 1 },
    "cache": { "status": "cold", "entries": 0 }
  }
}
```

**Purpose**: "Can this instance serve traffic?" Used by load balancers to remove unhealthy instances from the pool. Checks:

1. **Postgres**: `SELECT 1` with 2-second timeout. If this fails, the instance cannot evaluate flags that aren't cached.
2. **Redis**: `PING` with 1-second timeout. If this fails, the instance can still serve from Postgres but with degraded latency.
3. **Cache**: Reports whether the in-memory cache has been populated. A cold cache after startup is expected (warm-up period).

**Readiness logic**:
- Postgres UP → 200 (Redis and cache are nice-to-have)
- Postgres DOWN → 503 (cannot guarantee correct evaluations)

**Auth**: None. Same reasoning as liveness.

**Why two endpoints instead of one?** A single `/health` endpoint conflates two questions. If the process is alive but Postgres is down, we want the orchestrator to keep the process running (don't restart — it might recover) but remove it from the load balancer (don't send traffic). Two endpoints give the operator precise control.

### SSE Event Format

The `/stream` endpoint emits `text/event-stream` responses. Three event types are defined:

#### `flag_change` — a flag's configuration changed

```
id: 42
event: flag_change
data: {"flag_key":"checkout-v2","environment_id":"env_abc123","change":"updated","version":5}
```

| Field | Type | Description |
|---|---|---|
| `flag_key` | string | The flag's human-readable key |
| `environment_id` | UUID | Which environment changed |
| `change` | enum | `updated` \| `archived` \| `enabled` \| `disabled` |
| `version` | int | New `flag_environments.version` (for OCC on re-fetch) |

The payload does **not** include the new rule set. Rationale: sending full rule payloads over SSE means every connected SDK receives data it may not need. The SDK re-evaluates the flag by calling `POST /evaluate/flags/:key` after receiving the event.

Exception: `enabled` and `disabled` changes imply a known new value (`false` for disabled, restored for enabled) and can be applied locally without a re-fetch.

#### `resync` — SDK must re-fetch all flags

```
id: 43
event: resync
data: {"reason":"event_log_expired"}
```

Sent when a reconnecting SDK requests events older than the Redis event log's TTL (30s). The SDK must discard its local cache and re-bootstrap via `POST /evaluate/flags`.

#### `heartbeat` — keepalive

```
event: heartbeat
data: {"timestamp":"2024-11-15T03:22:41Z"}
```

Sent every 30 seconds. SDKs reset their reconnect timer on each heartbeat. If no heartbeat arrives within 90 seconds, the SDK treats the connection as dead and reconnects.

### SSE Reconnection & Backoff

SDKs must implement exponential backoff with jitter to avoid thundering herd on server restarts:

| Parameter | Value |
|---|---|
| Initial delay | 1 second |
| Backoff multiplier | 2× |
| Max delay | 60 seconds |
| Jitter | ±20% of current delay |
| Max reconnect attempts | Unlimited (flag evaluation must keep working) |

On each reconnect, the SDK sends `Last-Event-ID: <last_received_id>` in the HTTP request header. If the server has the events in its Redis log, it replays them. If not, it sends a `resync` event (see above).

**Goroutine model (Go SDK)**: Two goroutines — one owns the SSE connection and reconnect loop, one serves the user-facing `IsEnabled` / `GetVariant` API from a thread-safe local cache. The user-facing goroutine is never blocked by reconnection.

---

## Observability

> ![Observability Stack](./diagrams/diagram-obs.png)

### What we instrument

Every flag evaluation emits:

```go
// Span attributes on every evaluation
span.SetAttributes(
    attribute.String("flag.key", flagKey),
    attribute.String("flag.type", flag.Type),
    attribute.Bool("flag.enabled", config.Enabled),
    attribute.String("flag.value", resultValue),
    attribute.String("flag.reason", reason),      // "rule", "default", "disabled"
    attribute.Int("flag.rule_index", ruleIndex),
    attribute.String("user.id", userID),
    attribute.String("environment", envSlug),
)
```

### Prometheus metrics

| Metric | Type | Description |
|---|---|---|
| `flagstone_evaluations_total` | Counter | Total evaluations (labels: flag, env, result) |
| `flagstone_evaluation_duration_seconds` | Histogram | Evaluation latency |
| `flagstone_cache_hits_total` | Counter | Cache hit rate (labels: level) |
| `flagstone_sse_connections` | Gauge | Active SSE connections |
| `flagstone_api_requests_total` | Counter | API requests (labels: method, path, status) |

### Pre-built Grafana dashboard

The project will include a JSON dashboard for Grafana that shows:
- Flag evaluation rates and latency
- Cache hit ratios
- Active SSE connections
- API error rates
- Audit log activity

---

## Infrastructure (AWS)

> ![AWS Architecture](./diagrams/diagram-aws.png)

### Why AWS and not Hetzner / Fly / Railway

Long-term, Hetzner is 5x cheaper. But:

- The $300 AWS credits give 6+ months free even after the free tier.
- Learning AWS is **billable skill**. Having Terraform + RDS + EC2 + IAM on your CV is worth real money in freelance.
- Enterprise customers ask for "deploy in my AWS account". If we already know the terrain, sales are easier.
- When we leave the free tier, we evaluate. Hetzner is still there.

### Why t4g.small (Graviton ARM)

- **Free until December 2026**: 750 hours/month, enough to run 24/7.
- **2 vCPU, 2GB RAM**: enough for Go + Redis + a test workload.
- **ARM Graviton is ~20% cheaper post-free-tier than equivalent x86**.
- **Go cross-compiles to ARM without changes**: `GOARCH=arm64 go build`.

### Network layout

```
VPC (10.0.0.0/16)
  ├── Public subnets  (10.0.1.0/24, 10.0.2.0/24)  → Internet Gateway
  │     EC2 lives here. Reachable from internet for HTTP/HTTPS.
  │
  └── Private subnets (10.0.10.0/24, 10.0.11.0/24)
        RDS lives here. Never reachable from internet.
```

**No NAT Gateway**: costs ~$32/month (no free tier). RDS in private subnets without NAT means the DB has no outbound internet — a feature, not a bug.

**Two AZs**: RDS DB Subnet Groups require at least two subnets in different AZs even for single-AZ deployment. No extra cost.

### Security measures

| Measure | Implementation |
|---|---|
| DB network isolation | Private subnets, SG allows only app SG on port 5432 |
| SSH restriction | `ssh_allowed_cidr` has NO default — forces explicit IP config |
| SSRF protection | IMDSv2 required (`http_tokens = "required"`) |
| Encryption at rest | EBS + RDS `storage_encrypted = true` |
| No hardcoded credentials | IAM instance profile with role, no access keys |
| Audit trail | CloudTrail + audit_log table |
| Session Manager backup | SSM policy attached — SSH-less access if port 22 is locked |

### Why single-AZ for RDS

Multi-AZ doubles the cost (~$26/month vs $13/month post-free-tier) for a standby replica. For v1:

- We have automated backups (7 days retention).
- If the AZ goes down, we restore in another AZ — implies ~30 minutes downtime.
- For a service without SLAs yet, that's acceptable.

When a paying customer requires 99.99% uptime, we enable Multi-AZ with a toggle in `terraform.tfvars`.

---

## Cost Strategy

### Months 1-12 (full free tier)

| Resource | Usage | Cost |
|---|---|---|
| EC2 t4g.small | 24/7 | $0 (free until Dec 2026) |
| RDS db.t3.micro | 24/7 | $0 (12 months free) |
| EBS 20GB gp3 | continuous | $0 (within 30GB free) |
| RDS storage 20GB | continuous | $0 (within 20GB free) |
| Outbound transfer | < 100GB/month | $0 |
| CloudWatch | basic metrics | $0 |
| **Total** | | **$0/month** |

### Month 13+ (post RDS free tier, t4g still free)

| Resource | Approximate cost |
|---|---|
| EC2 t4g.small | $0 (until Dec 2026) |
| RDS db.t3.micro | ~$13/month |
| EBS + storage | ~$5/month |
| Transfer | ~$0-5/month |
| **Total** | **~$20/month** |

With $300 credits: **15 extra months at $20/month**. Conservatively, **24+ months running free**.

### Month 24+ (post-credits, post-Graviton trial)

If the project is still active:

- **If it has traction** (paying customers, 500+ stars, active contributors): stay on AWS, cost justified by MRR.
- **If it's just portfolio**: migrate to Hetzner (~$5-8/month for everything) or Fly.io free tier.

### Mandatory alarms from day one

Configure in CloudWatch:

- **Billing alarm at $5**: early warning.
- **Billing alarm at $20**: red alert, investigate immediately.
- **AWS Budgets**: email notification + block at 80% of monthly budget.

---

## Monetization Model

> Strategy, pricing, and execution phases live in **[BUSINESS.md](./BUSINESS.md)**. This section covers the architectural implications.

Flagstone is **dual-distribution**: the same codebase runs as both a fully open-source self-hosted server and as our managed Cloud offering. There is **no fork**, no enterprise-only feature flags, no closed-source addons. The only difference between the two is operational and configuration.

### Self-hosted (free, MIT license)

The user runs `docker compose up` (or Terraform → AWS / Kubernetes / their own infra) and gets the full server.

- All features available, no artificial limits
- All API endpoints, all SDK functionality
- All security and observability features
- MIT license: fork it, modify it, embed it
- No phone-home, no telemetry, no license check

Configuration default: `FLAGSTONE_DEFAULT_PLAN=enterprise` — new tenants get unlimited everything. The `tenants.plan` column exists but quotas resolve to "no limit" for the `enterprise` plan.

Suitable for: solo devs, internal tooling teams, companies with strict data residency requirements, anyone who wants full control.

### Flagstone Cloud (managed by us, paid tiers)

We operate a multi-tenant deployment of the same code. Customers create an account at `cloud.flagstone.dev`, never touch a server, never run a migration. Bring credit card.

Configuration: `FLAGSTONE_DEFAULT_PLAN=free` — new tenants land on the free plan and explicitly upgrade.

**Pricing tiers** (illustrative — final numbers set at launch):

| Tier | Price | What you get |
|---|---|---|
| **Free** | $0 | 1 project, 10 flags, 100k evals/month, community support |
| **Starter** | $19/mo | 5 projects, 100 flags, 5M evals/month, email support |
| **Pro** | $79/mo | Unlimited projects/flags, 50M evals/month, priority support, webhooks |
| **Enterprise** | Custom | SSO, SLA 99.9%, dedicated support, on-prem option |

Plan limits are enforced at the storage layer (see [Plan enforcement](#plan-enforcement-tenant-quotas)). Billing integration via Stripe with subscription webhooks updating `tenants.plan`.

### Why this model works

**The same codebase serves both.** This is the GitLab / Sentry / Plausible / Cal.com model — proven to work for OSS infrastructure tools.

| Concern | Why it's fine |
|---|---|
| "Won't self-hosters cannibalize Cloud customers?" | Different audience. Self-hosters spend engineering time; Cloud customers spend money. Most teams pick the one that matches their constraint (time vs money). |
| "What's the moat?" | Operating a reliable SaaS is non-trivial: monitoring, backups, on-call, security patches, SLAs. We charge for that operational expertise, not the bits. |
| "Why open source the core?" | Trust + adoption. Feature flags are critical infrastructure. Companies are wary of vendor lock-in. Open core eliminates that concern and 10×s the addressable audience. |
| "Will OSS slow down Cloud development?" | No — the same features ship to both. Cloud only adds: Stripe integration, multi-tenant operational tooling (per-tenant metrics dashboards, billing), and SSO/SAML (which can be a paid plugin even in self-hosted, optional). |

### What is NOT closed-source

Everything the docs describe is in the open repository:
- Rule engine, evaluation algorithm, consistent hashing
- All SDKs
- Database schema, migrations
- OpenTelemetry integration
- RBAC, JWT, API key auth
- The dashboard web UI
- Terraform modules

### What IS closed-source (Cloud-only operational code)

A small `cmd/flagstone-cloud/` overlay (not in the OSS repo) contains:
- Stripe webhooks → plan updates
- Cloud-specific admin endpoints (cross-tenant search, support tools)
- Billing email templates
- Onboarding flows for the marketing site

These are not feature flags features — they're SaaS operations. Self-hosters don't need them.

### Migration path: self-hosted → Cloud (or back)

Both deployments use the same schema. Migration is a `pg_dump` from one and `pg_restore` to the other. This is a real selling point: customers aren't locked in. They can prototype self-hosted and move to Cloud when they scale, or vice versa.

---

## Competitive Landscape

### Direct competitors

| | Flagstone | **Flipt** | Unleash | Flagsmith | LaunchDarkly |
|---|---|---|---|---|---|
| Language | Go | Go | TypeScript | Python | ? |
| Config model | API + DB | **Git-native (v2)** | API + DB | API + DB | API |
| OTel native | Yes | Yes | No | No | Partial |
| Self-hosted | Yes | Yes | Yes | Yes (limited) | No |
| Stars | 0 (new) | **4,800+** | 13,500+ | 6,300+ | N/A |
| Commits | 1 | **5,000+** | 10,000+ | 8,000+ | N/A |
| Maturity | Pre-alpha | Production | Production | Production | Production |

### Flipt is the primary competitor

Flipt is the closest competitor: same language (Go), same philosophy (self-hosted, OTel-native), but with 5,000+ commits head start. Key differences:

1. **Flipt v2 moved to Git-native configuration** — flags are defined in YAML files in a Git repo. This adds complexity for teams that want a simple API-first approach.
2. **Flagstone stays API-first** with a traditional database backend. Create a flag via API, change it via dashboard, see the effect in seconds.
3. **Flagstone targets simplicity over features** — Flipt has years of features we won't match. We win on ease of deployment and learning curve.

### Honest assessment

Flagstone is NOT going to replace Flipt for teams that already use it. Our market is:
- Teams that haven't adopted feature flags yet (greenfield)
- Teams that find Flipt's Git-native model too complex
- Teams that want the simplest possible self-hosted solution
- Solo devs / small teams that want one binary + Postgres

### OpenFeature (CNCF standard)

[OpenFeature](https://openfeature.dev/) is a CNCF sandbox project defining a standard API for feature flag evaluation. Supporting it is near-mandatory for any new feature flag tool — it means SDKs can use a standard interface and swap providers.

**Decision**: Implement an OpenFeature Go provider in Milestone 3. This is a significant competitive advantage — it means existing OpenFeature users can adopt Flagstone without changing their application code.

---

## Implementation Conventions

Code-level decisions that apply across the entire codebase. Every contributor follows these so the code stays consistent.

### Logging

**Library: `go.uber.org/zap`** (not stdlib `log/slog`). Reasoning: every flag evaluation may log warnings or errors, and at high QPS the per-call allocation cost of `slog` adds up. Zap's typed-field API (`zap.String`, `zap.Int`) avoids interface boxing and is measurably faster. For a feature-flag server where logging happens in the hot path, this matters.

Use the **non-sugared** logger (`*zap.Logger`, not `*zap.SugaredLogger`) — sugaring trades performance for ergonomics, exactly the opposite of why we chose zap.

#### Levels

| Level | When to use | Example |
|---|---|---|
| `Debug` | Per-evaluation traces, dropped in production by default | `"rule matched", zap.Int("rule_index", 2)` |
| `Info` | Lifecycle events, mutations | `"flag created", zap.String("flag_key", k)` |
| `Warn` | Recoverable misconfigurations | `"rollout without user_id"` |
| `Error` | Operations that failed but didn't crash | `"redis publish failed, falling back to TTL"` |
| `Fatal` | Startup failures only | `"cannot connect to database"` (uses `os.Exit(1)`) |

Never use `Panic` — panic recovery middleware exists for unexpected crashes, not for things we should log.

#### Mandatory structured fields

Every log line in a request-handling code path MUST include (via context):

```go
logger.Info("flag updated",
    zap.String("request_id", reqID),
    zap.String("tenant_id", tenantID.String()),
    zap.String("user_id", userID.String()),
    zap.String("flag_key", string(flagKey)),
)
```

The middleware injects a request-scoped logger into the context. Handlers retrieve it via `log.FromContext(ctx)`. No global logger inside handlers — that loses correlation.

#### PII handling in logs

- **Never log full emails.** Log the domain only (`@company.com`) or a hash if needed for correlation.
- **Never log password fields.** Even in error paths.
- **Never log raw user context attributes** in production — they may contain PII the customer doesn't want in our logs. Log only the keys that matched (`zap.Strings("matched_attributes", []string{"country", "plan"})`) and counts.
- **API keys**: only the key prefix (`fs_live_a3b9`), never the full key.
- **JWT tokens**: never log, even truncated.

In development (`FLAGSTONE_ENV=development`), debug-level logging may include more detail. In production, the PII rules above are non-negotiable.

#### Sampling at high QPS

At >1k evaluations/sec, even Info-level logging at one log per eval becomes noisy. Zap's built-in sampler keeps the first N logs per second per level and drops the rest:

```go
core := zapcore.NewSamplerWithOptions(baseCore, time.Second, 100, 100)
```

This is enabled by default in production config. Errors and warnings are always logged in full — only Info/Debug are sampled.

### Error handling

**Wrapping convention**: `fmt.Errorf("package.Operation: %w", err)`. The prefix names the package and operation — errors are self-documenting in logs without needing a stack trace:

```
storage.GetFlag: sql: no rows in result set
engine.Evaluate: storage.GetFlag: connection refused
api.handleEvaluate: engine.Evaluate: storage.GetFlag: context deadline exceeded
```

**Sentinel errors** for conditions callers need to distinguish with `errors.Is()`:

```go
var (
    ErrFlagNotFound        = errors.New("flag not found")
    ErrFlagArchived        = errors.New("flag archived")
    ErrEnvironmentNotFound = errors.New("environment not found")
    ErrVersionConflict     = errors.New("version conflict") // OCC
)
```

HTTP handlers map domain errors to status codes via `errors.Is()`. Anything that isn't a known sentinel becomes a 500.

### Dependency injection

**No framework** (Wire, Uber fx, etc.). The dependency graph fits in ~20 lines of `main.go`, assembled by hand with explicit constructors:

```go
pool     := storage.NewPool(cfg.DatabaseURL)
pgStore  := storage.NewPostgresStore(pool)
redStore := storage.NewRedisCachedStore(pgStore, redisClient, cfg.FlagCacheTTL)
memStore := storage.NewMemoryCachedStore(redStore, cfg.FlagCacheTTL)
engine   := engine.New(memStore)
server   := api.NewServer(engine, cfg)
```

A DI framework adds its own dependency, code generation, and a learning curve without meaningful benefit at this scale. Revisit if the graph exceeds ~40 nodes.

### Decorator pattern for the cache layers

Each cache layer is a decorator wrapping the next. All three implement the same interface:

```go
type FlagStore interface {
    GetFlagConfig(ctx context.Context, envID EnvironmentID, key FlagKey) (*FlagConfig, error)
    ListFlagConfigs(ctx context.Context, envID EnvironmentID) ([]*FlagConfig, error)
}

// Concrete implementations (innermost → outermost):
// 1. postgresStore        — source of truth
// 2. redisCachedStore     — wraps postgresStore, adds Redis cache
// 3. memoryCachedStore    — wraps redisCachedStore, adds in-process cache
```

The engine only knows `FlagStore`. Unit tests pass a `fakeFlagStore` returning in-memory fixtures. Integration tests use `postgresStore` directly. Each layer is independently testable.

### Database connection pool sizing

`pgxpool.New` defaults to **`max(4, GOMAXPROCS) = ~4-8 connections`** which is far too low for a feature-flag server doing hundreds of evaluations per second. The pool needs explicit sizing.

#### Sizing formula

```
MaxConns_per_instance = min(
    RDS_max_connections × 0.8 / N_instances,
    target_qps × p99_query_seconds × safety_factor
)
```

Concrete numbers for the default AWS setup (`db.t3.micro`, RDS `max_connections = 87`, 1 app instance):

| Variable | Value | Source |
|---|---|---|
| RDS `max_connections` | 87 | `db.t3.micro` formula: `LEAST({DBInstanceClassMemory/9531392}, 5000)` |
| Reserved for other clients | 20% (~17) | Migrations, admin, monitoring |
| Available for app | 70 | 87 − 17 |
| Target QPS per instance | 1000 | Modest target |
| p99 query time | 5 ms | From SLO |
| Safety factor | 2× | Bursts, retries |
| Calculated max | 10 | 1000 × 0.005 × 2 |
| **Configured `MaxConns`** | **25** | Headroom above calculated, well below RDS limit |
| `MinConns` | 5 | Warm pool, avoids cold-connect on startup |

#### Configuration

```go
poolCfg, _ := pgxpool.ParseConfig(cfg.DatabaseURL)
poolCfg.MaxConns        = 25
poolCfg.MinConns        = 5
poolCfg.MaxConnLifetime = 1 * time.Hour    // recycle to pick up RDS failovers
poolCfg.MaxConnIdleTime = 5 * time.Minute  // release idle conns
poolCfg.HealthCheckPeriod = 30 * time.Second
pool, _ := pgxpool.NewWithConfig(ctx, poolCfg)
```

These values live in `config.go` as env vars (`DB_MAX_CONNS`, `DB_MIN_CONNS`, etc.) with the above as defaults.

#### Pool exhaustion monitoring

Expose `pool.Stat()` metrics to Prometheus:

```go
flagstone_db_pool_acquired_connections   // currently checked out
flagstone_db_pool_idle_connections       // available in pool
flagstone_db_pool_total_connections      // total open
flagstone_db_pool_acquire_wait_seconds   // histogram of how long Acquire() blocked
```

If `acquire_wait_seconds` p99 exceeds 10ms, the pool is undersized for the workload.

### Plan enforcement (tenant quotas)

The `tenants.plan` column already exists with check constraint `IN ('free', 'team', 'enterprise')` — but no code reads it. Plan enforcement is what turns this into a real SaaS-capable system.

#### Plan limits

| Resource | `free` | `team` | `enterprise` |
|---|---|---|---|
| Projects | 1 | 5 | unlimited |
| Flags per project | 10 | 100 | unlimited |
| Environments per project | 3 (the defaults) | 5 | unlimited |
| Segments per project | 5 | 50 | unlimited |
| API keys per environment | 2 | 10 | unlimited |
| Team members | 1 | 10 | unlimited |
| Evaluations per month | 100,000 | 5,000,000 | contractual |
| SSE concurrent connections per env | 10 | 100 | unlimited |
| Audit log retention | 7 days | 90 days | 2 years |
| Webhooks | - | 3 per project | unlimited |
| SSO (SAML/OIDC) | - | - | Yes |
| SLA | none | none | 99.9% |

Limits are stored as a hardcoded `map[string]PlanLimits` keyed by plan name. They live in `internal/auth/plans.go` so they're easy to find and adjust without a migration.

#### Enforcement layer

Plan limits are enforced in the **storage layer**, not the handlers. Reason: any code path that creates a flag/segment/key must enforce, and centralizing it prevents bypasses.

```go
func (s *flagStore) Create(ctx context.Context, f *Flag) error {
    plan := plans.For(ctx.Tenant())
    count, _ := s.countFlags(ctx, f.ProjectID)
    if count >= plan.MaxFlagsPerProject {
        return ErrPlanLimitExceeded{
            Resource: "flags",
            Limit:    plan.MaxFlagsPerProject,
            Plan:     plan.Name,
        }
    }
    // ... INSERT
}
```

#### HTTP response

`ErrPlanLimitExceeded` maps to **`402 Payment Required`** (not 403). The body includes the plan info so the client can prompt the user to upgrade:

```json
{
  "error": {
    "code": "PLAN_LIMIT_EXCEEDED",
    "message": "Free plan is limited to 10 flags per project. Upgrade to Team for up to 100.",
    "details": {
      "resource": "flags",
      "current": 10,
      "limit": 10,
      "plan": "free",
      "upgrade_url": "https://flagstone.dev/pricing"
    }
  }
}
```

Why 402 and not 403? 402 specifically signals "this is a paid-tier restriction, not a permissions issue" — the user has the right role, they just don't have the right plan.

#### Self-hosted: plans default to enterprise

Self-hosted deployments don't have plan restrictions by default. The env var `FLAGSTONE_DEFAULT_PLAN=enterprise` (default in self-hosted builds) makes all tenants unlimited. The hosted Cloud deployment sets `FLAGSTONE_DEFAULT_PLAN=free` so new tenants get the free tier and upgrade explicitly.

This is the only difference between self-hosted and Cloud builds — same code, one env var.

### Startup retry, not request retry

The DB connection retries with exponential backoff on startup — Postgres takes a few seconds to be ready in Docker. Requests do not retry.

```go
func connectWithRetry(ctx context.Context, url string) (*pgxpool.Pool, error) {
    backoff := time.Second
    for i := range 5 {
        pool, err := pgxpool.New(ctx, url)
        if err == nil {
            return pool, nil
        }
        slog.Warn("db not ready, retrying", "attempt", i+1, "backoff", backoff)
        time.Sleep(backoff)
        backoff *= 2
    }
    return nil, fmt.Errorf("storage: failed to connect after 5 attempts")
}
```

Per-request retries are intentionally absent. A failing query has a defined fallback (next cache level → `false`). Retrying adds latency; the fallback is faster and always correct.

### Middleware stack order

The order is load-bearing — a request ID must exist before auth logs it; rate limiting must happen after auth to limit by API key, not by anonymous IP:

```
panic recovery → request ID → structured log → auth → rate limit → handler
```

```go
mux.Handle("POST /api/v1/evaluate/flags/{key}", Chain(
    handlers.Evaluate,
    middleware.RecoverPanic,   // outermost: catches everything below
    middleware.RequestID,      // generates req ID before any logging
    middleware.Logger(logger), // logs with req ID attached
    middleware.APIKeyAuth(keyStore),
    middleware.RateLimit(cfg.EvaluateRateLimit), // limits per authenticated key
))
```

`Chain` applies middlewares right-to-left so the first listed wraps outermost.

### SDK configuration: functional options

The SDK is a public API imported by third parties. Functional options let new options be added without breaking existing callers:

```go
type Option func(*sdkConfig)

func WithCacheTTL(d time.Duration) Option  { return func(c *sdkConfig) { c.cacheTTL = d } }
func WithLogger(l *zap.Logger) Option      { return func(c *sdkConfig) { c.logger = l } }
func WithHTTPClient(h *http.Client) Option { return func(c *sdkConfig) { c.httpClient = h } }

func New(serverURL, apiKey string, opts ...Option) *Client
```

Internal packages use plain config structs — functional options only pay off for public APIs that evolve over time.

### Value types for critical IDs

Prevent the class of bug where `flagKey` and `environmentID` are swapped in a function call — the compiler catches it:

```go
type FlagKey       string
type EnvironmentID string
type TenantID      string
```

Used in all `FlagStore` and `Engine` interface signatures. Not used inside HTTP handler internals where path values from `r.PathValue()` are immediately converted at the boundary.

### Table-driven tests for the engine

The engine has many cases — operators, tree shapes, rollout edge cases, missing attributes, type coercion. Table-driven tests are mandatory:

```go
var evaluateTests = []struct {
    name    string
    rules   string
    context map[string]any
    want    bool
    reason  string
}{
    {"no rules → default",     "[]",   nil,                            false, "DEFAULT"},
    {"eq match",  `[{"conditions":{"attribute":"plan","op":"eq","value":"premium"},"value":true}]`, map[string]any{"plan":"premium"}, true,  "RULE_MATCH"},
    {"eq no match",            `...`,  map[string]any{"plan":"free"},  false, "DEFAULT"},
    {"missing attribute → false", `...`, map[string]any{},            false, "DEFAULT"},
}
for _, tt := range evaluateTests {
    t.Run(tt.name, func(t *testing.T) { ... })
}
```

Engine tests are pure unit tests — no DB, no Redis, no HTTP. Everything is in `internal/engine/evaluate_test.go`.

---

## Deferred Decisions

Honest list of things we know we'll need eventually, but don't implement now:

| Decision | When to revisit | Estimated cost |
|---|---|---|
| Terraform S3 backend | When there are 2+ developers | ~$1/month |
| Multi-AZ RDS | When a customer requires it contractually | +$13/month |
| Application Load Balancer | When we need multiple instances or WAF | +$16/month |
| CDN (CloudFront) | When serving geographically distributed content | Variable |
| Secrets Manager | When there are 3+ secrets or 2+ developers | ~$0.40/secret/month |
| Auto Scaling | When one instance is insufficient | Variable |
| gRPC API | Milestone 3, when multi-language SDKs benefit from Protobuf | Zero (infra) |
| Rate limiting | Milestone 2 — needed before public deployment | Zero (in-process) |
| CORS configuration | When dashboard SPA or client-side SDKs exist | Zero |
| CSRF protection | When dashboard uses cookie-based sessions | Zero |
| Backups cross-region | When compliance requires it | Variable |
| VPC Flow Logs | When there's a network mystery to debug | Storage cost |
| Multivariate flags (variants) | Milestone 2 — when boolean on/off is insufficient | Zero (schema + engine change) |
| Scheduled flags | Milestone 2 — `activate_at`/`deactivate_at` on `flag_environments` + background goroutine | Zero |
| Flag dependencies | Never — resolve in client code. Complexity doesn't justify the coupling. | N/A |
| Circuit breaker | Never — the manual fallback chain (Redis → Postgres → in-memory → `false`) already provides graceful degradation without a state machine. Redis client reconnects automatically. | Zero |
| DI framework (Wire/fx) | Never at this scale — the dependency graph fits in ~20 lines of `main.go`. See [Implementation Conventions](#implementation-conventions). | Zero |
| OpenFeature Go provider | Milestone 3 — after core is stable | Zero |

The principle: **each infra feature pays its cost only when there's a concrete user or requirement justifying it**. Until then, simple beats "prepared for the future".

---

## References

- [Feature Toggles (Martin Fowler)](https://martinfowler.com/articles/feature-toggles.html)
- [Postgres JSONB Performance](https://www.postgresql.org/docs/current/datatype-json.html)
- [The Twelve-Factor App](https://12factor.net/)
- [AWS Well-Architected Framework](https://aws.amazon.com/architecture/well-architected/)
- [Designing Data-Intensive Applications (Martin Kleppmann)](https://dataintensive.net/)
- [OpenFeature Specification](https://openfeature.dev/specification/)
- [Flipt Documentation](https://www.flipt.io/docs)
- [OWASP Authentication Cheat Sheet](https://cheatsheetseries.owasp.org/cheatsheets/Authentication_Cheat_Sheet.html)
- [JWT Best Practices (RFC 8725)](https://datatracker.ietf.org/doc/html/rfc8725)
