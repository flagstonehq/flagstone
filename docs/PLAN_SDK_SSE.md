# Plan: Demoable App + Go SDK with SSE

> **Status:** Phases 1 and 2 **DONE** (validated end-to-end with Docker). Phase 3 **PROPOSED** (not yet approved, not yet implemented).
> **Scope:** Phase 1 (demoable app) ✅ + Phase 2 (SSE backend + Go SDK) ✅ + Phase 3 (SDK parity with competitors) ⏳.
> **Estimated effort:** Phases 1+2 ~2 days (done). Phase 3.0–3.5 ~11.5 days (proposed).
> **Revision note:** Phase 3 reworked after a code-grounded review — 3.0a is now a single-writer background model (fixes the confirmed race *and* eliminates hot-path network blocking); 3.2 introduces a mandatory-default, no-error eval API and is moved ahead of 3.1; `DataStore` traffics in raw `[]byte` (was an un-constructable struct alias) and drops in priority. See "What's still missing" for the post-3.5 capability roadmap (analytics events, variants, `sdktest`, codegen, FlagTracker, OpenFeature).

---

## Context and rationale

A full inventory of the repository surfaced these facts, which dictate the order of work:

- **`internal/engine/` is 100% pure Go**: zero DB/HTTP imports, only `zap` for logging. It is trivially SDK-reusable.
- **The backend already exposes** `POST /api/v1/evaluate/flags/{key}` and `POST /api/v1/evaluate/flags`, gated by API-key auth. There is no `/stream` endpoint yet.
- **SSE is designed in `DESIGN.md`** (lines 1029-1387) but never implemented: `internal/streaming/` is an empty directory with a `.gitkeep`.
- **There is no seed data**: `admin@acme.com` / `password123` are referenced in 8 Playwright specs and in `setup_test.go`, but **they are never provisioned anywhere**. The only bootstrap path is `POST /api/v1/setup` with arbitrary data.
- **`docker-compose.yml` only starts Postgres and Redis**. A developer must run `make run` and `npm run dev` in two separate terminals.
- **The Go SDK example in the README does not compile**: it imports `github.com/flagstonehq/flagstone/pkg/sdk`, which is an empty directory.
- **`flag_environments.version` already exists** with an auto-increment trigger at `migrations/000001_init.up.sql:338-347`. Optimistic concurrency control is already wired; SSE will simply expose it.
- **DESIGN.md does not specify a `segment_change` event**, but the SDK needs one to invalidate cached segments when their rules change. We extend the design with justification.

**Why we start with Phase 1**: without seed data and without the `api` service in `docker-compose.yml`, it is impossible to test the app as a user. Without testing it as a user, we cannot know whether the UX is sound before investing in SDKs. This is the order of least regret.

---

## Implementation log (Phases 1 and 2)

**Phase 1.1 — `cmd/seed/main.go`**: idempotent seed binary that calls `GET /api/v1/setup/status`, posts setup, logs in, creates `my-app` project with `production` + `development` envs, 2 API keys, flags `new-checkout` and `dark-mode`, segments `beta-users` and `premium-customers`. Built and verified. **Note**: it is a separate binary, not a `NewServer` call, so it does not start the HTTP server.

**Phase 1.2 — `docker-compose.yml`**: added `api`, `web`, and `seed` services. `api` runs migrations on boot, `web` waits for `/readyz`, `seed` is profile-scoped.

**Phase 1.3 — demo path validation**: all 7 verification steps pass. Login → projects → flags → segments → apikeys → audit all return 200. No audit 500 bug encountered.

**Phase 1.4 — README quickstart**: replaced "Local Development" with the 5-line `docker compose up` flow.

**Phase 2.4 — `git mv internal/engine → pkg/engine`**: zero friction, no API changes. `pkg/engine/doc.go` stability note added.

**Phase 2.1 + 2.3 — SSE hub + handler**: `internal/streaming/{event,hub,handler}.go` implemented. Hub has register/unregister/publish/broadcast/persistToRedis. Handler serves SSE with replay, 30s heartbeat, `Last-Event-ID` reconnection. Mounted in `internal/api/server.go` behind `AuthAPIKey`. **Bug fixed during integration**: `Logger` middleware wrapped `ResponseWriter` in `statusRecorder` which embedded `http.ResponseWriter` (an interface) but did not implement `http.Flusher`, so SSE returned 500 "streaming unsupported". Added explicit `Flush()` method on `statusRecorder`.

**Phase 2.2 — 6 publish hooks**: in `flag_envs.go` (toggle + rules_updated), `flags.go` (created/updated/archived), `segments.go` (created/updated/archived). **Note**: actually 8 publish hooks total (3 in flags, 3 in segments, 2 in flag_envs) — slightly more than the original 6 in the plan. E2E verified: SSE client received `flag_change` event on flag update.

**Phase 2.5a — `GET /api/v1/sdk/snapshot` endpoint** (`internal/api/sdk_snapshot.go`): returns `{environment, flags, segments, fetched_at, request_id}`. Reuses `buildFlagConfigFromJoined` from `evaluate.go`. Keyed by flag key and segment key for O(1) lookup. `Flags` and `Segments` JSON shapes are exactly the same as `engine.FlagConfig` and `engine.Segment` (no transformation).

**Phase 2.5b — route mounted**: with `RequestID + Logger + AuthAPIKey` middleware.

**Phase 2.6 — `pkg/sdk` package** (5 files: `doc.go`, `options.go`, `cache.go`, `types.go`, `stream.go`, `client.go`): Functional options (`WithEndpoint`, `WithAPIKey`, `WithCacheTTL`, `WithLogger`, `WithHTTPClient`). Public API: `Bool`/`String`/`Number`/`JSON`/`All` returning a `Value{Value, Reason}` struct. Engine is reused locally — `Bool` after the first snapshot is ~1µs, no HTTP.

**Phase 2.7 — tests**: 14 unit tests with `httptest` (all pass), 5 API tests for the snapshot endpoint (all pass), 1 integration test (skipped unless `BACKEND_URL` is set).

**Phase 2.8 — `pkg/sdk/example/main.go`**: a real HTTP server exposing `/checkout?user_id=42&country=AR`, returns `{"checkout": "v1"}` or `v2` based on `new-checkout`.

**Phase 2.9 — README + CHANGELOG**: replaced broken example with the one that compiles. Added Unreleased entry in `CHANGELOG.md`.

**End-to-end validation** (manual, via Docker):
- `enabled=true` with rule → SDK re-fetches via SSE → `{"checkout":"v2"}`.
- `enabled=false` (PATCH) → SDK re-fetches via SSE → `{"checkout":"v1"}`.
- `golangci-lint run ./...` → 0 issues.
- `go test ./...` → all pass (api 35s, middleware, auth, storage, engine, sdk).

**One important detail**: `cmd/seed/main.go` is a separate binary that does **not** call `NewServer`. It hits the API over HTTP. This is what makes it portable (works against any Flagstone instance, not just localhost).

---

## Phase 1 — Make the app demoable (~0.5 day)

### 1.1 `cmd/seed/main.go` — idempotent seed binary

**Why a Go binary, not a bash script**:
- It needs to distinguish "setup has not run yet" from "setup already ran" (by calling `GET /api/v1/setup/status`).
- It must insert flags/segments/environments matching the shape that `internal/api/setup.go:34-144` expects, which is cleaner with typed Go.
- It is idempotent by design: if rows already exist, it does not duplicate them (`ON CONFLICT DO NOTHING` or an explicit pre-check).
- It is testable (`go test ./cmd/seed/...`).

**What it does**:
1. `GET ${BACKEND_URL}/api/v1/setup/status` — if tenants already exist, no-op and exit 0.
2. Otherwise: `POST /api/v1/setup` with `{tenant_name: "Acme Corp", tenant_slug: "acme", admin_email: "admin@acme.com", admin_password: "password123", confirm_password: "password123"}`.
3. Log in to obtain an access token.
4. Create the `my-app` project.
5. Create environments `production` and `development`.
6. Create API keys `fs_test_my_app_prod` and `fs_test_my_app_dev`.
7. Create flags `new-checkout` (boolean) and `dark-mode` (boolean) with different rules per env.
8. Create segments `beta-users` and `premium-customers`.
9. Print a summary with credentials and API keys to stdout.

**Trade-off**: This could be done via direct SQL, but using HTTP matches exactly what a real user would do and implicitly validates those endpoints.

### 1.2 `docker-compose.yml` — add `api` and `web` services

```yaml
services:
  postgres:  # existing
  redis:     # existing
  api:
    build: .
    depends_on: [postgres, redis]
    ports: ["8080:8080"]
    environment:
      DATABASE_URL: postgres://flagstone:flagstone_dev@postgres:5432/flagstone?sslmode=disable
      REDIS_URL: redis://redis:6379
      JWT_SECRET: dev-secret-change-me
      GIN_MODE: release
    healthcheck:
      test: ["CMD", "wget", "-qO-", "http://localhost:8080/readyz"]
      interval: 5s
      timeout: 3s
      retries: 10
  web:
    build: ./web
    depends_on:
      api:
        condition: service_healthy
    ports: ["3002:3002"]
    environment:
      BACKEND_URL: http://api:8080
      NEXT_PUBLIC_APP_URL: http://localhost:3002
  seed:
    build: .
    depends_on:
      api:
        condition: service_healthy
    environment:
      BACKEND_URL: http://api:8080
    profiles: ["seed"]
    command: ["/app/seed"]
```

**Why the healthcheck on `api`**: `web` and `seed` must not start until `/readyz` returns 200. Without this, the seed step fails because `api` has not yet run migrations.

**Why `seed` is profile-scoped**: so it does not run on every `up`. Re-running it is safe because it is idempotent.

### 1.3 Validate the demo path

```bash
docker compose up -d                   # postgres, redis, api, web
docker compose --profile seed up seed  # one-time
open http://localhost:3002
# Login: admin@acme.com / password123
```

**What to verify**:
1. Login works and redirects to `/projects`.
2. The projects list shows `my-app`.
3. Clicking `my-app` reveals a sidebar with Flags / Segments / API Keys / Audit / Settings.
4. The flags page shows `new-checkout` and `dark-mode` with their production/development toggles.
5. The segments page shows `beta-users` and `premium-customers`.
6. The API keys page shows the 2 created keys.
7. **CRITICAL**: the audit page does not return 500. If it does, we fix it inline (the most likely cause is the audit query filtering on `tenant_id` when it should filter on `project_id`, or the recent `audit_test.go` changes not being reflected in the handler).

**If real bugs are found** (audit 500, missing `/auth/me` blocking the Account page, etc.), they are fixed as part of Phase 1 because they block the demo.

### 1.4 README — 5-line quickstart

Replace the existing "Local Development" section with:

```bash
git clone https://github.com/flagstonehq/flagstone
cd flagstone
docker compose up -d && docker compose --profile seed up seed
open http://localhost:3002
# Login: admin@acme.com / password123
```

Move detailed instructions to `docs/QUICKSTART_DEV.md` for those who want more.

---

## Phase 2 — SSE backend + Go SDK (~1 week)

### 2.1 `internal/streaming/sse.go` — event Hub

Based on `DESIGN.md:1029-1080`:

```go
type Event struct {
    ID            int64          `json:"id"`
    Type          string         `json:"type"`         // flag_change | segment_change | resync | heartbeat
    EnvironmentID uuid.UUID      `json:"environment_id"`
    Payload       map[string]any `json:"payload"`
    Timestamp     time.Time      `json:"timestamp"`
}

type Hub struct {
    redis      *redis.Client
    mu         sync.RWMutex
    clients    map[uuid.UUID]map[*Client]struct{} // envID → clients
    register   chan *Client
    unregister chan *Client
    publish    chan Event
}
```

**Why Redis and not memory-only**: `DESIGN.md:1071-1076` requires it for `Last-Event-ID` replay (capped list with TTL). Without Redis, if an SDK reconnects after an `api` deploy, pending events are lost. Redis is already running in `docker-compose.yml`, so no new dependency.

**Why we accept duplicate IDs across instances**: the ID only needs to be monotonic **within an environment over a 30-second window**. Redis `LPUSH` already provides per-key ordering. If a developer wants HA later, we can switch to a global `INCR` counter — that is a scale problem, not an MVP problem.

### 2.2 Publish hooks at 6 call sites

```go
// internal/api/flag_envs.go (after UpdateWithVersion succeeds)
if rowsAffected == 1 {
    s.hub.Publish(streaming.Event{
        Type:          "flag_change",
        EnvironmentID: envID,
        Payload: map[string]any{
            "flag_key": flagKey,
            "change":   "updated",
            "version":  newVersion,
        },
    })
}
```

The 6 call sites:
- `internal/api/flag_envs.go:183` → `flag_change updated`
- `internal/api/flag_envs.go` (enable/disable handler) → `flag_change enabled|disabled`
- `internal/api/flags.go:331` → `flag_change updated` (metadata-level)
- `internal/api/flags.go:389` → `flag_change archived`
- `internal/api/segments.go:322` → `segment_change updated` (extends DESIGN.md)
- `internal/api/segments.go:380` → `segment_change archived`

**Why in the API layer, not in storage** (decision confirmed): storage stays 100% pure, with no bus coupling. The only way to bypass the bus is to write directly to SQL, which should not happen. If a future data migration or admin script needs to write, we can add the publish call explicitly.

**Why wrap UPDATE + audit-insert in a single transaction**: today at `flag_envs.go:183-204` they are separate statements. If `UPDATE` commits but the audit insert fails, the `Publish` still fires, and the SDK refetches a version that the admin never "formally saw". We wrap them in `pgx.Tx` so the event fires only when both commit.

**Why extend DESIGN.md with `segment_change`**: the design only mentions `flag_change`, but the SDK needs to invalidate its segments cache too (when segment rules change, flags referencing that segment now match differently). This is a gap in the design that we fill with justification.

### 2.3 `internal/streaming/handler.go` — `GET /api/v1/stream`

```go
func (h *Handler) ServeSSE(w http.ResponseWriter, r *http.Request) {
    envID := middleware.EnvironmentIDFromContext(r.Context())

    w.Header().Set("Content-Type", "text/event-stream")
    w.Header().Set("Cache-Control", "no-cache")
    w.Header().Set("Connection", "keep-alive")
    w.Header().Set("X-Accel-Buffering", "no") // disable nginx buffering

    flusher, ok := w.(http.Flusher)
    if !ok { /* error */ }

    client := &Client{envID: envID, send: make(chan Event, 16), lastEventID: parseLastEventID(r)}
    h.hub.register <- client
    defer func() { h.hub.unregister <- client }()

    // Replay missed events from Redis
    if err := h.replayMissed(r.Context(), client); err != nil {
        fmt.Fprintf(w, "event: resync\ndata: {}\n\n")
        flusher.Flush()
    }

    // Heartbeat + main loop
    ticker := time.NewTicker(30 * time.Second)
    defer ticker.Stop()

    for {
        select {
        case <-r.Context().Done():
            return
        case ev := <-client.send:
            fmt.Fprintf(w, "id: %d\nevent: %s\ndata: %s\n\n", ev.ID, ev.Type, json.Marshal(ev.Payload))
            flusher.Flush()
        case <-ticker.C:
            fmt.Fprintf(w, "event: heartbeat\ndata: {\"timestamp\":%q}\n\n", time.Now().UTC().Format(time.RFC3339))
            flusher.Flush()
        }
    }
}
```

Mounted in `internal/api/server.go:361-379`, alongside the evaluate routes, with `middleware.AuthAPIKey` (which already injects `EnvironmentID`).

### 2.4 `git mv internal/engine pkg/engine`

**Why move to `pkg/`**: Go convention is that `pkg/` is importable by external modules and `internal/` is not. Moving it makes it officially public, with SemVer. It is trivial because the package has no internal dependencies.

Changes:
- `git mv internal/engine pkg/engine`
- Update imports in:
  - `internal/api/evaluate.go:14`
  - `internal/api/evaluate_test.go`
  - All `*_test.go` files that import `internal/engine`
- Add `pkg/engine/doc.go` with a note: "Stable API. Breaking changes bump major version."

**Why it is not a breaking change internally**: only the path changes; the public `Engine` API (its methods) stays identical.

### 2.5 `pkg/sdk/client.go` — the Go SDK

```go
package sdk

type Client struct {
    endpoint   string
    apiKey     string
    httpClient *http.Client
    cache      *snapshotCache
    stream     *streamConn
    engine     *engine.Engine // reuses the engine locally
    logger     *zap.Logger
}

type Options struct {
    Endpoint   string        // "https://api.flagstone.dev"
    APIKey     string        // "fs_live_..."
    CacheTTL   time.Duration // default 30s; used for re-fetch if SSE does not arrive
    Logger     *zap.Logger   // optional
    HTTPClient *http.Client  // optional, for testing
}

func New(opts Options) (*Client, error) { /* ... */ }

func (c *Client) Bool(ctx context.Context, key string, evalCtx map[string]any) (bool, error)   { /* ... */ }
func (c *Client) String(ctx context.Context, key string, evalCtx map[string]any) (string, error) { /* ... */ }
func (c *Client) Number(ctx context.Context, key string, evalCtx map[string]any) (float64, error) { /* ... */ }
func (c *Client) JSON(ctx context.Context, key string, evalCtx map[string]any) (any, error)     { /* ... */ }
func (c *Client) All(ctx context.Context, evalCtx map[string]any) (map[string]Value, error)     { /* ... */ }

func (c *Client) Start(ctx context.Context) error { /* dials SSE, starts background goroutine */ }
func (c *Client) Close() error                    { /* graceful shutdown */ }
```

**Reusing the engine** (decision confirmed): the first `Bool()` or `Start()` makes a `POST /api/v1/evaluate/flags` (bulk), parses the response, and builds in memory:
- `map[string]engine.FlagConfig` (the flags with their rules)
- `map[string]engine.Segment` (the project's segments)

Then each `Bool(key, ctx)` runs `engine.Evaluate(EvaluateRequest{FlagConfig: flags[key], Segments: segments, Context: ctx})` **locally**, with no HTTP. Result: ~1µs per flag, zero roundtrip.

**Cache + invalidation**:
- The snapshot is cached with TTL (`CacheTTL`, default 30s).
- When a `flag_change` arrives via SSE, we mark that specific flag as stale.
- When a `resync` arrives, we flush everything and re-bulk.
- If the `flag_change` was lost (disconnect), the TTL still expires and we re-bulk anyway.

**Polling fallback**: if SSE fails 3 times in a row, the SDK falls back to polling (re-bulk every `CacheTTL`). It is not a visible degradation to the user.

### 2.6 `pkg/sdk/stream.go` — SSE consumer

```go
type streamConn struct {
    endpoint   string
    apiKey     string
    httpClient *http.Client
    onEvent    func(Event)
    onResync   func()
    logger     *zap.Logger
}

func (s *streamConn) run(ctx context.Context) {
    backoff := time.Second
    for {
        if err := s.dial(ctx); err != nil {
            s.logger.Warn("SSE dial failed", zap.Error(err), zap.Duration("retry_in", backoff))
            select {
            case <-ctx.Done():
                return
            case <-time.After(backoff):
                backoff = min(backoff*2, 30*time.Second)
                continue
            }
        }
        backoff = time.Second // reset on success
    }
}
```

It parses the stream with `bufio.Scanner` line by line, identifying `event:`, `id:`, `data:`. `Last-Event-ID` is sent in the next dial's header after a disconnect.

### 2.7 SDK tests

- **Unit (`pkg/sdk/client_test.go`)**: with `httptest.NewServer`, mock both the evaluate and SSE endpoints. We verify:
  - `Bool` reads from cache without HTTP on the second hit.
  - `flag_change` invalidates only the affected flag.
  - `resync` flushes everything.
  - Reconnect with the correct `Last-Event-ID`.
- **Integration (`pkg/sdk/integration_test.go`)**: skips if `BACKEND_URL` is empty. If set, runs against the real backend brought up by `docker compose up`.

### 2.8 `pkg/sdk/example/main.go`

A small HTTP server that:
- Exposes `GET /checkout?user_id=42&country=AR`.
- Calls `client.Bool(ctx, "new-checkout", map[string]any{"user_id": 42, "country": "AR"})`.
- Returns `{"checkout": "v2"}` or `{"checkout": "v1"}` depending on the flag.
- Prints SSE events to stdout (so the developer can see them live).

Serves as a usage manual and a manual smoke test.

### 2.9 README + CHANGELOG

Replace the broken example in `README.md:277-311` with:

```go
import "github.com/flagstonehq/flagstone/pkg/sdk"

client, _ := sdk.New(sdk.Options{
    Endpoint: "https://api.flagstone.dev",
    APIKey:   os.Getenv("FLAGSTONE_API_KEY"),
})
go client.Start(ctx)
defer client.Close()

enabled, _ := client.Bool(ctx, "new-checkout", map[string]any{
    "user_id": 42,
    "country": "AR",
})
```

And add a `CHANGELOG.md` entry:

```
## [v0.2.0] - WIP
### Added
- Real-time flag updates via SSE (GET /api/v1/stream)
- Go SDK at pkg/sdk with TTL cache and reconnect
- cmd/seed for one-command demo provisioning
- Docker compose now includes api and web services
```

---

## Risks and mitigations

| Risk | Mitigation |
|---|---|
| SSE consumes one connection per API key | Hub limits to 1 connection per API key (SDKs do too); 30s heartbeat detects dead connections quickly. |
| Redis goes down, SSE stops working | SDK falls back to polling with TTL. The bulk evaluate endpoint keeps working. |
| `cmd/seed` runs twice and duplicates data | Idempotent: pre-checks `GET /api/v1/setup/status`; checks if `my-app` exists before creating. |
| Moving `internal/engine` to `pkg/engine` breaks imports | Find-and-replace all imports in a single PR. Tests cover the refactor. |
| README SDK example stays broken | Replaced explicitly with one we know compiles. |
| Phase 1 finds real bugs (audit 500) | Fixed inline because they block the demo. Each fix is documented. |

---

## Out of scope (and why)

- **TypeScript/Python SDKs**: user chose Go. We can port the pattern later.
- **Webhooks**: roadmap says Milestone 4. SSE already provides live push.
- **Helm chart**: infrastructure, not core. Better after the core is validated.
- **OpenTelemetry / metrics**: nice-to-have, does not block the demo or SDK.
- **SSE multi-region with Redis Streams**: the capped list is enough for now; multi-region HA is a scale problem, not an MVP problem.
- **E2E tests of the SSE in Playwright**: the SDK's unit tests cover the consumer's behavior. E2E of the stream itself can be added later.

---

## Execution order

1. **Phase 1.1**: `cmd/seed/main.go` + `make seed` (~1.5h)
2. **Phase 1.2**: updated `docker-compose.yml` (~1h)
3. **Phase 1.3**: smoke test + fix bugs found (~1h)
4. **Phase 1.4**: README quickstart (~30min)
5. **Phase 2.4**: `git mv internal/engine pkg/engine` (~30min, first because it unblocks everything else)
6. **Phase 2.1 + 2.3**: SSE hub + handler (~3h)
7. **Phase 2.2**: publish hooks at the 6 call sites (~2h)
8. **Phase 2.5 + 2.6**: `pkg/sdk` client + stream (~3h)
9. **Phase 2.7**: SDK tests (~2h)
10. **Phase 2.8 + 2.9**: example + README + CHANGELOG (~1h)

**Total estimate: ~2 days of focused work.**

---

## Phase 3 — SDK parity with competitors (PROPOSED, ~11 days)

### 3.0 Context: where we are, where we need to be

Phase 2 shipped a working SDK, but two findings from the post-completion audit force a new round of work:

**Finding 1: real data race in production code.** `go test -race ./pkg/sdk/...` reports a conflict between `invalidateFlag` (called from the SSE goroutine when a `flag_change` arrives) and `eval` (called from any goroutine on every `Bool`/`String`/`Number`/`JSON` call). The cause: `snapshotCache.get()` returns a pointer to the snapshot and releases the `RWMutex.RLock`, but the caller then reads `snap.flags[key]` outside the lock. The map delete in `invalidateFlag` and the map read in `eval` are not actually serialized. This is not a flaky test — it is a latent bug that surfaces at scale (more requests per second = higher probability of corrupting the in-memory map or panicking on a nil entry). **It must be fixed before anything else.**

**Finding 2: feature matrix gap vs competitors.** A side-by-side with LaunchDarkly v7, Unleash v6, Statsig, and the OpenFeature spec shows we are at "MVP functional" — we cover maybe 20% of what production users need:

| Feature | Flagstone (post-Phase 2) | LaunchDarkly | Unleash | Statsig | OpenFeature spec |
|---|:-:|:-:|:-:|:-:|:-:|
| Bool/String/Number/JSON eval | ✅ | ✅ | ✅ | ✅ | ✅ |
| Evaluation reasons | ✅ | ✅ | ✅ | ✅ | ✅ |
| **SSE streaming updates** | ✅ | ✅ | ❌ (polling) | ❌ (polling) | n/a |
| Typed `Context` (LD `ldcontext`, Unleash `Context`, OF `EvaluationContext`) | ❌ (raw `map[string]any`) | ✅ | ✅ | ✅ | ✅ |
| `ScopedClient` (ctx-bound) | ❌ | ✅ | ❌ | ❌ | ✅ |
| `EvaluationDetail` (value + reason + variant + error) | partial (`Value` only) | ✅ `VariationDetail` | ✅ | ✅ | ✅ |
| Offline mode | ❌ | ✅ | ✅ `LocalMode` | ✅ `LocalMode` | n/a |
| Bootstrap from file/JSON | ❌ | ✅ `ldfiledata` | ✅ `Bootstrap` | ✅ `BootstrapValues` | provider-specific |
| Persistent data store interface | ❌ | ✅ (Redis, etc.) | ✅ | ✅ | n/a |
| `StatusProvider` / `Initialized` / `WaitForReady` | ❌ | ✅ | ✅ | ✅ | ✅ |
| `Hooks` (Before/After/Error/Finally) | ❌ | ✅ `ldhooks` | ✅ listener | ✅ callbacks | ✅ |
| OpenTelemetry tracing | ❌ | ✅ `ldotel` | ❌ | ❌ | n/a |
| Custom strategies/operators (SDK-side) | ❌ | partial | ✅ | ❌ | n/a |
| Variants / A-B experiments | ❌ | ✅ | ✅ | ✅ | ✅ |
| Sticky/persistent assignment | ❌ | ✅ | ✅ | ✅ | n/a |
| Relay Proxy / daemon mode | ❌ | ✅ | ❌ | ❌ | n/a |
| Public flag-change subscription | ❌ | ✅ `FlagTracker` | ✅ listener | ✅ callback | ✅ eventing |
| Tests with `-race` | ❌ | ✅ | ✅ | ✅ | n/a |
| Benchmarks | ❌ | ✅ | ✅ | ✅ | n/a |

**Why we do Phases 3.0–3.5 first (and not 3.6+):**

- **3.0** is non-negotiable: shipping a race condition into a public SDK is a credibility problem. Also unlocks the ability to measure improvements in subsequent phases (we need `-race` and benchmarks to defend any optimization).
- **3.1–3.2** are table-stakes API surface. Every single competitor has typed `Context` and `EvaluationDetail`. A user porting from LD to Flagstone will look for these in the first 10 minutes and bounce if they are missing.
- **3.3** unblocks SSR (Next.js server-rendering cannot wait for a network round-trip per request), test fixtures (deterministic flag state without HTTP), and degraded operation (the SDK can start serving requests before the snapshot is loaded, with safe defaults).
- **3.4** is the foundation for 3.7+ (variants need persistent assignment for stickiness), for daemon mode (3.10), and for any future persistence target (Redis, MongoDB, etc. — we ship the interface, the community ships the adapter).
- **3.5** is the only way to debug the SDK in production. Without `Status()` and `WaitForReady`, every "why is my flag returning the default value?" support ticket is a fishing expedition.

**Why we are NOT doing 3.6+ (Hooks, OTel, Variants, Daemon, Custom operators) in this round:**

- **Hooks (3.6)**: requires design decisions about ordering, error propagation, sync vs async, and what "context" the user gets. Easy to get wrong, hard to change later. Better to ship the underlying primitives (Context, EvaluationDetail) and let users write the obvious glue code first.
- **OTel (3.8)**: pulls in `go.opentelemetry.io/otel` as a dependency. That is a 1-2MB transitive footprint. We can ship it as `pkg/sdk/otelhook` (a separate module) once the base SDK is stable, so it is opt-in.
- **Variants (3.7)**: requires a wire format change (the snapshot endpoint would need to include `flag.variants`). That is a backend change plus a migration story for existing clients. Defer until 3.0–3.5 are stable.
- **Custom operators (3.9)**: niche, no clear wire story (the server does not know about the user's custom operator), and easy to add later behind a `WithOperator(op)` option.
- **Daemon mode (3.10)**: needs a separate process (the Relay Proxy) plus a contract for how it writes to the shared DataStore. That is a deployment story, not just an SDK story.

### 3.0 — Quality baseline (~2 days)

**Why this is the first phase of Phase 3**: every subsequent phase benefits from `-race` and benchmarks, and we cannot ship a public SDK with a known data race.

#### 3.0a — Fix the data race AND the hot-path blocking with a single-writer background model

**The race is confirmed real** (`go test -race ./pkg/sdk/...`):
```
WARNING: DATA RACE
Write at ... invalidateFlag() cache.go:64    (mapdelete, from the SSE goroutine)
Previous read ... eval()       client.go:144 (mapaccess, from a Bool caller)
```
`get()` returns the snapshot pointer and releases the `RLock`; the caller then reads `snap.flags[key]` outside the lock, while the SSE goroutine deletes from the same map. **There is a second, unlisted race in the same family**: `withFlag` (`types.go:73`) mutates the map *in place* (`s.flags[fc.Key] = fc`) and then `replace`s with the *same pointer*. Any COW fix that only touches `invalidateFlag` leaves this one live.

**A deeper problem the original plan missed — the eval path blocks on the network.** Tracing `eval → ensureLoaded` (`client.go:162`) shows three places where evaluating a flag triggers a *synchronous HTTP fetch inside the caller's request*:
- **Cold start**: first `Bool` → `isStale` (zero `fetchedAt`) → `bulkFetch` (blocking).
- **After every `flag_change`**: `invalidateFlag` deletes the key → next `Bool(key)` finds it missing → `singleFetch` (blocking). So **every flag toggle injects a network round-trip into the next user request for that flag.**
- **Every TTL expiry (30s)**: the first `Bool` after expiry → `isStale` → `bulkFetch` (blocking). One unlucky request every 30s eats a full snapshot fetch. Same after a `resync` (`invalidateAll` empties the snapshot).

This is the single biggest "feels wrong" gap versus LD/Unleash/Statsig, all of which hold a hard rule: **evaluation never touches the network.** It always serves the last-known-good snapshot from memory; *all* fetching happens on a background goroutine.

**Decision**: replace `sync.RWMutex` with a **single-writer + `atomic.Pointer[snapshot]`** design. Reads are lock-free `Load()`; **all** writes (initial fetch, refetch on `flag_change`, resync, TTL refresh) are funneled through one background goroutine, so a plain `Store()` is enough — no `CompareAndSwap` loop, no per-event map clone. Because the background goroutine is the only writer and published snapshots are never mutated in place, the snapshot is effectively immutable once stored, which removes the race at the root (including the `withFlag` one).

```go
type snapshotCache struct {
    value atomic.Pointer[snapshot] // single writer ⇒ Store() suffices
}
func (c *snapshotCache) get() *snapshot   { return c.value.Load() } // lock-free
func (c *snapshotCache) store(s *snapshot) { c.value.Store(s) }      // only from refreshLoop
```

```go
// eval NEVER fetches. It serves whatever is in memory; missing/empty ⇒ default.
func (c *Client) eval(...) (any, Reason) {
    snap := c.cache.get()
    fc, ok := snap.flags[key]
    if !ok { return def, ReasonFlagNotFound } // no block, no surprise error on the hot path
    ...
}

// the background loop owns all writes:
func (c *Client) refreshLoop(ctx context.Context) {
    ticker := time.NewTicker(c.opts.cacheTTL)
    defer ticker.Stop()
    for {
        select {
        case <-c.flagChanged: c.cache.store(c.refetch(ctx)) // SSE asked for a refetch
        case <-ticker.C:      c.cache.store(c.refetch(ctx)) // TTL refresh, off the hot path
        case <-ctx.Done():    return
        }
    }
}
```

`flag_change` now signals the background loop to refetch and atomically swap; the *old value keeps being served* until the new snapshot lands (last-known-good), instead of blanking the flag or blocking the request.

**Files touched**: `pkg/sdk/cache.go` (rewrite), `pkg/sdk/client.go` (eval no longer calls `ensureLoaded`/`singleFetch`; fetching moves to `refreshLoop`), `pkg/sdk/stream.go` (`onFlagChange` posts to a channel instead of mutating the cache directly).

**Acceptance criteria**:
- `go test -race -count=100 ./pkg/sdk/...` passes 100/100 times. A test that fires `flag_change` every 1ms while 100 goroutines call `Bool` runs for 10s with no race report.
- `BenchmarkBool_Cached` (see 3.0d) shows **0 allocs/op** and no HTTP — proving eval is purely in-memory even mid-`flag_change`.

#### 3.0b — `Makefile` with `test-race` and `bench` targets

**Why a Makefile, not just a script**: the `Makefile` is the documented entry point. Anyone joining the project (or CI) should be able to run `make test-race` and have it work without reading the README.

Targets:
- `make test` — `go test ./...` (the default).
- `make test-race` — `go test -race ./...` (slower, used in CI and pre-release).
- `make bench` — `go test -bench=. -benchmem ./pkg/sdk/...`.
- `make lint` — `golangci-lint run ./...`.
- `make e2e` — `docker compose up -d --build api && smoke-test.sh`.
- `make cover` — `go test -coverprofile=cover.out ./...` and prints the percentage.

**Acceptance criterion**: `make test-race` exits 0 from a fresh clone.

#### 3.0c — Race-specific tests (`pkg/sdk/client_race_test.go`)

Three tests that are not just "run the existing tests with `-race`":

1. **`TestRace_ConcurrentEvalAndInvalidate`**: 100 goroutines call `Bool` on a loop. A publisher goroutine fires `flag_change` events every 1ms. Runs for 10 seconds. Must complete without race report and without panicking.
2. **`TestRace_BulkFetchDuringEval`**: 10 goroutines call `Bool` (which triggers `bulkFetch` on first call), while another goroutine reads `c.Status()`. Verifies the atomic state machine does not race with the in-progress fetch.
3. **`TestRace_StartAndCloseConcurrent`**: 10 goroutines call `Start(ctx)` while 10 others call `Close()`. With `sync.Once` on `Start`, this should be a no-op; the test catches any future regression that introduces a race.

**Why three, not one**: each test targets a different race scenario. If we collapse them into one, a future regression could mask one race with another.

#### 3.0d — Benchmarks (`pkg/sdk/bench_test.go`)

Six benchmarks, all using `b.RunParallel` to model real concurrency:

1. `BenchmarkBool_Cached` — `Bool` with the snapshot already in memory. Measures the in-process path.
2. `BenchmarkBool_Cold` — first `Bool` after `New()`, no snapshot yet. Measures the HTTP fetch + parse.
3. `BenchmarkAll_100Flags` — evaluate all 100 flags for one context.
4. `BenchmarkAll_Parallel` — 8 goroutines, each evaluates 100 flags.
5. `BenchmarkInvalidate_ConcurrentReaders` — readers running while a background `flag_change` swap happens. Proves the atomic `Store()` does not stall readers.
6. `BenchmarkSSE_Parse` — parse a single SSE event from a `bufio.Scanner`. Measures the hot path of the stream consumer.

**Why benchmarks are part of 3.0, not a separate phase**: the single-writer refactor in 3.0a changes the concurrency model. We need numbers to defend it — specifically that `BenchmarkBool_Cached` is in the low-ns range with 0 allocs and stays flat while a background swap is in flight. Running the benchmarks before and after the change is how we prove eval is genuinely non-blocking, not a vibes-based "atomic is faster".

**Output format**: each benchmark prints `ns/op` and `B/op` and `allocs/op`. The README is updated with a "Performance" section showing the numbers.

#### 3.0e — Edge-case tests (`pkg/sdk/edge_cases_test.go`)

Tests that protect against the support tickets we would otherwise get:

- `TestBool_FlagNotFound` — `Bool("nonexistent")` returns an error, not a panic.
- `TestBool_TypeMismatch` — `Bool` on a string flag returns a typed error, not a generic "flag not found".
- `TestContextCancellation` — `Bool` with a cancelled `ctx` returns `ctx.Err()` immediately, not after a network round-trip.
- `TestSnapshot_Timeout` — server takes 10s to respond (longer than client timeout), SDK returns an error, does not block forever.
- `TestSnapshot_MalformedJSON` — server returns `not json`, SDK returns a parse error, does not crash.
- `TestSnapshot_5xx` — server returns 500, SDK returns the wrapped error, does not silently fall back to defaults.
- `TestSSE_Heartbeat` — server sends only heartbeats for 60s, no events, connection stays alive.
- `TestSSE_LongIdle` — server delays 31s between heartbeats, client times out, reconnects with backoff.

**Why not 100% coverage**: 100% coverage is a vanity metric. These 8 tests cover the failure modes that users actually hit.

### 3.1 — Typed `Context` and `ScopedClient` (~2 days)

**Why this is its own phase**: every other competitor has it, and it is the single biggest "this looks like an LD clone" signal. Without `Context`, our API looks like an unfinished prototype.

#### 3.1a — Type `Context` (`pkg/sdk/context.go`)

```go
// Context carries the subject of a flag evaluation: who is the user (or
// service), what are their attributes, and which attributes are private.
type Context struct {
    // TargetingKey uniquely identifies the subject. Required for sticky
    // evaluation (e.g. consistent rollout bucketing).
    TargetingKey string

    // Attributes are arbitrary key-value pairs used in rule conditions
    // (e.g. "plan", "country", "user_id").
    Attributes map[string]any

    // PrivateAttributes lists attribute keys that must NOT be sent to
    // any external system (analytics, logs, error reports). For
    // GDPR/CCPA compliance.
    PrivateAttributes []string
}

func NewContext(targetingKey string, attrs map[string]any) Context
func (c Context) With(attr string, value any) Context
func (c Context) WithPrivate(attrs ...string) Context
func (c Context) asMap() map[string]any // merges TargetingKey into the map
```

**Why `With` returns a new `Context` (not a pointer)**: `Context` is a small value type. Returning by value makes it impossible to mutate accidentally, and copying is cheap (a string + a map header + a slice header).

**Why `PrivateAttributes` is on the SDK, not just a backend setting**: the backend does not know which attributes are sensitive in your application. The user tells the SDK which keys to redact in any logging or hook context.

**Why no `IPAddress`, `UserAgent`, `SessionID` etc. as first-class fields (unlike Unleash)**: those fields are application-specific. Unleash hard-codes them; we let users put them in `Attributes` with whatever name they want. This is more flexible and avoids breaking changes when a new field becomes relevant.

#### 3.1b — `ScopedClient` (`pkg/sdk/context.go`)

```go
// ScopedClient is a thin wrapper around *Client that binds a Context
// to all evaluations. Cheap to create (a pointer + a value); safe to
// share across goroutines.
type ScopedClient struct {
    client  *Client
    context Context
}

func (c *Client) Scoped(ctx Context) *ScopedClient {
    return &ScopedClient{client: c, context: ctx}
}

// Bool evaluates a boolean flag using the bound Context. No evalCtx
// parameter — the user has already provided it.
func (s *ScopedClient) Bool(key string) (bool, error)
func (s *ScopedClient) String(key string) (string, error)
func (s *ScopedClient) Number(key string) (float64, error)
func (s *ScopedClient) JSON(key string) (any, error)
func (s *ScopedClient) All() (map[string]Value, error)
```

**Why no `evalCtx` parameter on `Scoped`**: the whole point of `Scoped` is to avoid repeating yourself on every call. The user defines the context once (at request start in an HTTP server, or at startup for a service) and reuses it for all flag checks.

**Why `ScopedClient` is a value-like wrapper, not a method on `Client`**: keeps the public surface of `Client` small. Users who do not need scoping never see the `Scoped*` methods.

**Back-compat**: `Client.Bool(ctx, key, evalCtx)` stays exactly as it is. The `evalCtx` parameter wins over the `Scoped` context if both are provided (for the rare case where the caller wants to override one attribute).

#### 3.1c — Tests (`pkg/sdk/scoped_test.go`)

- `TestScoped_BindContext`: `c.Scoped(NewContext("u-1", ...)).Bool("flag")` uses the bound context, not nil.
- `TestScoped_OverrideAttr`: `c.Scoped(ctx).Bool("flag", map[string]any{"override": 1})` — the param-level attr overrides the scoped one.
- `TestScoped_Concurrent`: 100 goroutines share one `ScopedClient`, all read the same flag, all succeed.

### 3.2 — Safe-default API + `EvaluationDetail` (~1.5 days)

**Why this is its own phase, and why it is the headline DX change**: the current signature `Bool(ctx, key, evalCtx) (bool, error)` forces an error check at *every* call site. Ignore the error and you silently get `false` — which may be the wrong value. **No competitor does this.** LD, Statsig, and OpenFeature all take a default value and never make you handle an error on the hot path: the SDK is *designed* so a network blip or a missing flag can never break your request. The plan's own offline-mode section (3.3a) already says "Bool/String/Number/JSON return the supplied default value" — but the signature has no default parameter. That contradiction is resolved here.

The fix is a **two-tier API**: a happy path that takes a mandatory default and never returns an error, and a `*Detail` path for when you want the reason or the error. `EvaluationDetail` is not a bolt-on observability extra — it is the lower tier of the same decision, so the two ship together.

#### 3.2a — Primary API: defaults, not errors (`pkg/sdk/client.go`)

```go
// Happy path: mandatory default, no error return, never blocks (3.0a),
// never panics. A missing flag, type mismatch, or cold cache yields def.
func (c *Client) Bool(ctx context.Context, key string, def bool, evalCtx map[string]any) bool
func (c *Client) String(ctx context.Context, key string, def string, evalCtx map[string]any) string
func (c *Client) Number(ctx context.Context, key string, def float64, evalCtx map[string]any) float64
func (c *Client) JSON(ctx context.Context, key string, def any, evalCtx map[string]any) any
```

**Why mandatory default, not optional**: an optional default (variadic) lets a user forget it and fall back to the zero value, which reintroduces the "silent wrong value" footgun. Making it positional and required means the call site always declares its safe fallback explicitly — the value you are happy to serve when the platform is unreachable.

**Why no `error` on the happy path**: the only "errors" possible (flag not found, type mismatch, not yet loaded) all have one correct production response — serve the default. Returning an error would force `if err != nil { useDefault }` boilerplate at every call site, which is exactly the friction we are removing. When the caller *does* want to know, they use `BoolDetail` (3.2c), whose `EvaluationDetail.Reason` and `.Error` carry the full story.

**Migration note**: this is a breaking change to the Phase 2 signature. Since the SDK has not had a tagged release, we make the change now (pre-v0.2.0) rather than carrying two signatures forever. `ScopedClient` (3.1b) gets the same treatment: `s.Bool(key, def)`.

#### 3.2b — Type `EvaluationDetail` (`pkg/sdk/detail.go`)

```go
// EvaluationDetail is the full result of a single flag evaluation.
// Use the *Detail methods (BoolDetail, StringDetail, etc.) to get
// not just the value but the reason it was chosen and which rule (if
// any) matched.
type EvaluationDetail struct {
    // Value is the resolved value. Type matches the flag's flag_type
    // (bool for boolean flags, string for string flags, etc.).
    Value any

    // Reason explains why this value was returned. One of:
    //   RULE_MATCH, DEFAULT, DISABLED, FLAG_NOT_FOUND, FLAG_ARCHIVED, ERROR
    Reason Reason

    // RuleIndex is the index of the matching rule in the flag's rules
    // array, or -1 if no rule matched. Useful for A/B testing: log
    // "user X matched rule 3 of flag Y" to track which rule is winning.
    RuleIndex int

    // Variant is empty until 3.7 (Variants) is implemented. Reserved.
    Variant string

    // Error is non-nil only when Reason == ERROR.
    Error error
}

// Re-export engine reasons under our namespace so users do not have
// to import the engine package just to compare reasons.
type Reason = engine.Reason

const (
    ReasonRuleMatch    Reason = "RULE_MATCH"
    ReasonDefault      Reason = "DEFAULT"
    ReasonDisabled     Reason = "DISABLED"
    ReasonFlagNotFound Reason = "FLAG_NOT_FOUND"
    ReasonFlagArchived Reason = "FLAG_ARCHIVED"
    ReasonError        Reason = "ERROR"
)
```

**Why `Reason` is a type alias for `engine.Reason`**: avoids a duplicate constant. The engine already exports these. Re-exporting means the user does not have to import `pkg/engine` just to do `if detail.Reason == sdk.ReasonRuleMatch`.

**Why `Variant` is in the struct now even though 3.7 is not done**: changing the struct shape later is a breaking API change. Adding an unused field is a non-breaking change. Pay the small forward-compat cost now.

**Back-compat**: the `Value` type and `All() map[string]Value` stay. `Value{Value, Reason}` is exactly `EvaluationDetail` minus `RuleIndex` and `Variant`, so `All` callers do not notice.

#### 3.2c — `*Detail` methods (`pkg/sdk/detail.go`)

```go
// Same mandatory default as the happy path; never returns a Go error
// (the failure, if any, is carried in EvaluationDetail.Reason/.Error).
func (c *Client) BoolDetail(ctx context.Context, key string, def bool, evalCtx map[string]any) EvaluationDetail
func (c *Client) StringDetail(ctx context.Context, key string, def string, evalCtx map[string]any) EvaluationDetail
func (c *Client) NumberDetail(ctx context.Context, key string, def float64, evalCtx map[string]any) EvaluationDetail
func (c *Client) JSONDetail(ctx context.Context, key string, def any, evalCtx map[string]any) EvaluationDetail
```

Internally, these are one-liners over the existing `eval` method (which already returns the full `EvaluateResult`). The happy-path `Bool` (3.2a) is itself a one-liner over `BoolDetail` that returns just `.Value`.

#### 3.2d — Tests (`pkg/sdk/detail_test.go`)

- `TestBoolDetail_RuleMatch`: a rule matches, `Reason == ReasonRuleMatch` and `RuleIndex == 0`.
- `TestBoolDetail_Default`: no rules match, `Reason == ReasonDefault` and `RuleIndex == -1`.
- `TestBoolDetail_Disabled`: flag is disabled, `Reason == ReasonDisabled`.
- `TestBoolDetail_NotFound`: flag does not exist, `Reason == ReasonFlagNotFound` and `Error != nil`.

### 3.3 — Offline mode & Bootstrap (~2 days)

**Why this is its own phase**: it unlocks three production use cases (SSR, tests, degraded operation) that are completely impossible today. Without it, every consumer who wants any of those has to fork the SDK.

#### 3.3a — `WithOffline(true)` (`pkg/sdk/options.go`)

When the client is offline:
- `New()` returns immediately, no HTTP.
- `Initialized()` returns `true` (the client is in a known state).
- `Status().State()` returns `StateOffline`.
- `Bool/String/Number/JSON` return the supplied default value with `ReasonDefault`, no error.
- `All()` returns an empty map.
- `Start(ctx)` is a no-op (returns nil immediately).

**Justification for "no error on default"**: in offline mode, the user has explicitly opted out of network. Returning an error would force every `Bool` call to be wrapped in `if offline { ... } else { ... }`, which is exactly the boilerplate we are trying to avoid.

#### 3.3b — `WithBootstrap(json []byte)` and `WithBootstrapReader(io.Reader)`

Bootstrap loads a snapshot from a byte slice (or a Reader for files/streams). Use cases:
- **SSR**: server-side render at request time, no time for a fetch.
- **Tests**: deterministic flag state with no network dependency.
- **File fallback**: the user runs `SaveSnapshotToFile` once on a host with network, then ships the file to air-gapped hosts.
- **Cold-start performance**: load from disk on startup, fetch updates via SSE in the background.

```go
// WithBootstrap loads a snapshot from raw JSON bytes (the response
// shape of GET /api/v1/sdk/snapshot). Useful for SSR, tests, and
// air-gapped environments.
func WithBootstrap(json []byte) Option

// WithBootstrapReader is the io.Reader equivalent. Useful for
// loading from a file or an embedded asset without buffering.
func WithBootstrapReader(r io.Reader) Option

// WithBootstrapFile is a convenience wrapper that reads path and
// calls WithBootstrap. A missing file is a warning, not a fatal
// error (degraded mode: SDK starts, tries the network).
func WithBootstrapFile(path string) Option
```

**Why missing file is not fatal**: in production, the bootstrap file might be missing because the deploy is fresh. Crashing the app on a missing cache file is a worse failure mode than starting in degraded mode and falling back to defaults.

**Why bootstrap + SSE coexist**: bootstrap is a "starting state". Once loaded, the SSE stream (if available) updates it. If SSE is unavailable, the bootstrap is the entire state.

#### 3.3c — Wire format helpers (`pkg/sdk/bootstrap.go`)

```go
// LoadBootstrapFromFile reads a bootstrap file and returns the raw
// JSON bytes. The caller wraps it in WithBootstrap.
func LoadBootstrapFromFile(path string) ([]byte, error)

// SaveBootstrapToFile atomically writes the snapshot to path. Useful
// for "warm up" deployments: run the SDK once with network, then
// ship the file to air-gapped hosts.
func SaveBootstrapToFile(path string, snap []byte) error
```

`SaveBootstrapToFile` uses temp-file + `os.Rename` for atomicity (a process crash mid-write does not leave a corrupt file).

#### 3.3d — Tests (`pkg/sdk/bootstrap_test.go`)

- `TestBootstrap_Preload`: `New(WithBootstrap(validJSON))` then `Bool` returns the preloaded value with no HTTP.
- `TestBootstrap_InvalidJSON`: `WithBootstrap([]byte("not json"))` returns a `New` error, not a panic later.
- `TestBootstrap_MissingFile`: `WithBootstrapFile("/nonexistent")` does not error; the SDK starts and tries the network.
- `TestOffline_AllDefaultValues`: `New(WithOffline(true))` then `Bool` returns `false` (the zero value) and no error.
- `TestOffline_InitializedTrue`: `New(WithOffline(true)).Initialized() == true`.

### 3.4 — DataStore interface + InMemory + File (~2 days) — LOWER PRIORITY

**Priority note**: the original plan placed this at ~3 days and justified it almost entirely as "foundation for 3.7 (Variants) and 3.10 (daemon mode)" — both of which are explicitly out of scope. Building a public abstraction for deferred features is speculative (YAGNI). **Bootstrap (3.3) already delivers ~90% of the practical value** (SSR, tests, file fallback) without the interface. So 3.4 drops below 3.5 in the execution order, and we cut it down: ship the interface and the two trivial implementations, nothing more.

**The critical correction — traffic in `[]byte`, not an internal struct.** The original design did `type Snapshot = snapshot`, exporting an alias of a struct whose fields (`flags`, `segments`, `fetchedAt`) are all unexported. An external implementer literally **cannot read or construct** such a value, so the headline promise ("a community member implements `RedisDataStore` in ~50 lines") was false — they could hold a `*Snapshot` but never serialize it. The fix: the `DataStore` traffics in the **same raw `[]byte` wire format that `GET /api/v1/sdk/snapshot` already returns** and that `WithBootstrap` already consumes. External stores just persist and return bytes — zero internal types.

#### 3.4a — Interface `DataStore` (`pkg/sdk/datastore.go`)

```go
// DataStore persists the raw snapshot JSON (the exact body of
// GET /api/v1/sdk/snapshot). Trafficking in []byte means an external
// implementation never needs to import or construct an SDK type —
// it just stores and returns bytes.
type DataStore interface {
    // Load returns the most recent raw snapshot, or (nil, nil) if none.
    Load(ctx context.Context) ([]byte, error)
    // Save persists raw. Implementations should make this atomic
    // (no torn bytes visible to Load).
    Save(ctx context.Context, raw []byte) error
    // Close releases any resources. After Close, Load and Save return ErrClosed.
    Close() error
}
```

**Why `[]byte` and not a typed snapshot**: (1) it makes a third-party `RedisDataStore`/`S3DataStore` genuinely ~50 lines with no access to unexported fields; (2) `WithBootstrap([]byte)`, `SaveBootstrapToFile`, `FileDataStore`, and `DataStore` now all speak **one format**, so there is a single thing to learn and a single place the `schema_version` field lives; (3) it is forward-compatible — a snapshot field added server-side does not break a store that just relays bytes.

**Why a single Load/Save and not a key-value API**: the SDK only ever stores one thing — the current snapshot. A key-value API would invite "set flag X to Y", which is not a real use case (the snapshot is atomic). Less surface, fewer ways to misuse it.

**Why the interface takes `context.Context`**: the `File` implementation does not need it, but a future `Redis` implementation will (for timeouts on network calls). Adding the parameter now keeps the interface forward-compatible.

#### 3.4b — `InMemoryDataStore` (`pkg/sdk/datastore_memory.go`)

The default. Trivial: a `sync.RWMutex` guarding a `[]byte`. ~20 lines.

This is what the SDK uses internally today. The refactor in 3.4d makes it pluggable.

#### 3.4c — `FileDataStore` (`pkg/sdk/datastore_file.go`)

Stores the raw snapshot bytes as a single file. Operations:
- `Load`: read the file, return the bytes verbatim (no parsing — the SDK parses once on store).
- `Save`: write to a temp file, then `os.Rename` to the final path (atomic on POSIX).
- `Close`: no-op here, but the interface requires it for forward-compat.

**Why no file watching**: the SDK is pull-based. It calls `Load` when it needs a snapshot (on startup). The file being updated by another process is fine — the next `Load` sees the new bytes.

**Why no write coalescing**: a snapshot refresh fires one `Save`. Flag changes are human-paced; writing ~10 KB to disk a handful of times is negligible. A write coalescer would add complexity for no measurable benefit.

#### 3.4d — Wire `DataStore` into the single-writer loop (3.0a)

The `refreshLoop` (3.0a) is the only writer, so persistence slots in cleanly with no extra locking:

```go
// On startup, before the first network fetch:
if raw, _ := store.Load(ctx); raw != nil {
    c.cache.store(parseSnapshot(raw)) // serve immediately from disk
}
// After every successful refetch in refreshLoop:
snap, raw := c.refetchRaw(ctx)
c.cache.store(snap)        // in-memory, hot path
_ = store.Save(ctx, raw)   // persist the raw bytes, off the hot path
```

**Why keep the in-memory snapshot as the source of truth for reads**: the SDK does thousands of `Bool`/sec; reads always hit the lock-free `atomic.Pointer`, never the DataStore. The store is touched only on startup (`Load`) and after a refetch (`Save`).

**Back-compat**: `snapshotCache` is internal. The default store is `InMemoryDataStore`, so existing behavior is unchanged unless the user opts into `WithDataStore(...)`.

#### 3.4e — Tests (`pkg/sdk/datastore_test.go`)

- `TestInMemoryDataStore_RoundTrip`: `Save(raw)` then `Load()` returns the same bytes.
- `TestFileDataStore_RoundTrip`: `Save(raw)` then `Load()` returns the same bytes across a simulated restart (close + reopen the store).
- `TestFileDataStore_AtomicWrite`: a `Load` during a `Save` sees either the old bytes or the new bytes, never a partial write.
- `TestDataStore_LoadOnStartup`: starting with a pre-populated DataStore serves flags from disk **with no network call** until the first background refresh.

#### 3.4f — Out of scope (explicit)

**Redis (and any other network-backed DataStore) is NOT in 3.4.** Justification:
- Each new persistence target is a dependency + adapter maintenance burden. We ship the interface, not the implementations.
- Because the interface is `[]byte`-based, a community member can implement `RedisDataStore` in ~50 lines (mostly `go-redis` boilerplate) — no SDK internal types needed.
- We document the interface in `pkg/sdk/datastore.go` with the `FileDataStore` as the worked example and a "to implement your own" comment.

### 3.5 — StatusProvider + WaitForReady (~1 day)

**Why this is its own phase**: without observability, every "why is my flag returning the default value?" support ticket is a fishing expedition. The user needs to know the SDK's state without reading the source.

#### 3.5a — State machine (`pkg/sdk/status.go`)

```go
type State int
const (
    StateInitializing State = iota // New() called, no fetch yet
    StateConnected                 // bulkFetch succeeded at least once
    StateStale                     // bulkFetch failed but cache is non-empty
    StateOffline                   // WithOffline(true) was used
    StateError                     // bulkFetch failed and cache is empty
)
func (s State) String() string // "INITIALIZING", "CONNECTED", etc.
```

Transitions:
- `New()` → `Initializing`.
- `bulkFetch` OK → `Connected`, close the `readyCh`.
- `bulkFetch` fails, cache non-empty → `Stale` (serves cached values, but warn the user).
- `bulkFetch` fails, cache empty → `Error`.
- `WithOffline(true)` → `Offline` from `New()`.

**Why "Stale" is a separate state from "Connected"**: a stale SDK is still serving values, but they might be wrong. The caller needs to know this to decide whether to trust the result or fall back to a safe default at the application level.

#### 3.5b — `StatusProvider` (`pkg/sdk/status.go`)

```go
type StatusProvider interface {
    // State is the current state. Atomic read.
    State() State
    // Initialized is true once the first bulkFetch completed (or
    // bootstrap was provided, or offline mode is on).
    Initialized() bool
    // LastError is the most recent error from a fetch or SSE dial.
    // nil if there has been no error.
    LastError() error
    // LastUpdated is when the snapshot was last successfully fetched.
    // Zero time if never.
    LastUpdated() time.Time
    // WaitForReady blocks until Initialized() is true or ctx is done.
    // Returns ctx.Err() if the context expires first.
    WaitForReady(ctx context.Context) error
}

func (c *Client) Status() StatusProvider
```

**Why an interface, not a struct**: the user can mock it in tests, and a future version can add more methods without breaking the public API.

**Why `WaitForReady` takes a `context.Context`**: so the user can put a deadline on it (e.g. K8s readiness probe waits at most 5s).

**Why `Status()` is a method, not a field**: the field would expose the internal state, and any future change to that state would be a breaking API change. The method returns an interface, so we can swap implementations freely.

#### 3.5c — `OnStatusChange` callback

```go
// WithOnStatusChange registers a callback that fires on every state
// transition. The callback is called from internal goroutines; it
// must not block. Useful for emitting metrics.
func WithOnStatusChange(cb func(State)) Option

// AddOnStatusChange (post-New) is the same, but can be called
// anytime. Useful for late attach (e.g. after config is loaded).
func (c *Client) AddOnStatusChange(cb func(State))
```

**Why a callback AND a `Status()` method**: `Status()` is pull-based (the user polls). The callback is push-based (the user gets notified). Both are useful in different scenarios.

**Why "must not block" is documented but not enforced**: enforcement (a panic on a slow callback) is worse than the failure mode. Documenting is enough.

#### 3.5d — Tests (`pkg/sdk/status_test.go`)

- `TestStatus_Initializing`: right after `New()`, `State() == StateInitializing`.
- `TestStatus_ConnectedAfterFetch`: after a successful `bulkFetch`, `State() == StateConnected`.
- `TestStatus_StaleAfterFetchFail`: kill the server, wait for the next `bulkFetch` cycle, `State() == StateStale` (because the cache is non-empty from the previous fetch).
- `TestStatus_ErrorOnEmptyCache`: same as above but with no prior fetch → `State() == StateError`.
- `TestStatus_Offline`: `WithOffline(true)` → `State() == StateOffline` from the start, `Initialized() == true`.
- `TestWaitForReady_Success`: `WaitForReady(ctx)` returns nil after `bulkFetch` completes.
- `TestWaitForReady_Timeout`: `WaitForReady(ctx)` with a 50ms ctx returns `ctx.Err()` if the fetch is still in flight.
- `TestOnStatusChange_Fires`: a counter callback is invoked once per state transition (not retroactively).

---

## Updated risks and mitigations (Phases 1–3)

| Risk | Mitigation |
|---|---|
| SSE consumes one connection per API key | Hub limits to 1 connection per API key (SDKs do too); 30s heartbeat detects dead connections quickly. |
| Redis goes down, SSE stops working | SDK falls back to polling with TTL. The bulk evaluate endpoint keeps working. |
| `cmd/seed` runs twice and duplicates data | Idempotent: pre-checks `GET /api/v1/setup/status`; checks if `my-app` exists before creating. |
| Moving `internal/engine` to `pkg/engine` breaks imports | Find-and-replace all imports in a single PR. Tests cover the refactor. |
| README SDK example stays broken | Replaced explicitly with one we know compiles. |
| Phase 1 finds real bugs (audit 500) | Fixed inline because they block the demo. Each fix is documented. |
| **3.0a — single-writer refactor introduces a subtle bug** | The race tests in 3.0c are the canary; `BenchmarkBool_Cached` must stay 0-alloc under a concurrent background swap. If either fails, the model is wrong. |
| **3.2a — breaking signature change (default param) annoys early users** | No tagged release exists yet, so we change it pre-v0.2.0 and call it out in the CHANGELOG. Carrying two signatures forever is the worse outcome. |
| **3.4 — DataStore refactor breaks the in-memory case** | Default store stays `InMemoryDataStore`; existing tests must pass unmodified. New tests are additive. |
| **3.5 — `OnStatusChange` callback blocks the SDK** | Document "must not block" prominently. Future: a `BlockingCallbackError` log if a callback takes >1s. |
| **Snapshot wire shape changes in a future backend version** | A `schema_version` field lives in the snapshot JSON (the one format used by bootstrap, file, and DataStore). The SDK refuses to load an unknown version. |

---

## Updated out-of-scope (and why)

- **TypeScript/Python SDKs**: user chose Go. We can port the pattern later.
- **Webhooks**: roadmap says Milestone 4. SSE already provides live push.
- **Helm chart**: infrastructure, not core. Better after the core is validated.
- **OpenTelemetry / metrics**: covered in Phase 3.8 (future), behind an opt-in sub-package so the OTel dependency is not mandatory.
- **SSE multi-region with Redis Streams**: the capped list is enough for now; multi-region HA is a scale problem, not an MVP problem.
- **E2E tests of the SSE in Playwright**: the SDK's unit tests cover the consumer's behavior. E2E of the stream itself can be added later.
- **Redis / MongoDB / DynamoDB DataStore implementations**: we ship the `[]byte` interface (3.4). Community implements the adapters. Each new persistence target is a dependency + maintenance burden we are not taking on.
- **Exposure / evaluation analytics events**: the biggest capability gap, but it needs a backend ingestion endpoint, so it is a platform project, not an SDK-only one. First item in the post-3.5 roadmap.
- **Variants / A-B experiments (3.7)**: requires a snapshot wire-format change. Deferred until 3.0–3.5 are stable; pairs with analytics events.
- **Custom strategies / operators (3.9)**: niche, easy to add later as `WithOperator(op)`.
- **Relay Proxy / daemon mode (3.10)**: needs a deployment story, not just an SDK story.

> **Note**: the OpenFeature provider is **no longer deferred** — it is promoted to differentiator #7 / roadmap, because portability (anti-lock-in) is a core part of why someone would adopt this SDK. It ships as a thin sub-package over the stable eval surface.

---

## Updated execution order (Phases 1–3)

1. **Phase 1.1**: `cmd/seed/main.go` + `make seed` (~1.5h) ✅
2. **Phase 1.2**: updated `docker-compose.yml` (~1h) ✅
3. **Phase 1.3**: smoke test + fix bugs found (~1h) ✅
4. **Phase 1.4**: README quickstart (~30min) ✅
5. **Phase 2.4**: `git mv internal/engine pkg/engine` (~30min) ✅
6. **Phase 2.1 + 2.3**: SSE hub + handler (~3h) ✅
7. **Phase 2.2**: publish hooks at the 8 call sites (~2h) ✅
8. **Phase 2.5 + 2.6**: `pkg/sdk` client + stream (~3h) ✅
9. **Phase 2.7**: SDK tests (~2h) ✅
10. **Phase 2.8 + 2.9**: example + README + CHANGELOG (~1h) ✅
11. **Phase 3.0a**: single-writer background model — fixes the race *and* the hot-path blocking (~5h)
12. **Phase 3.0b**: `Makefile` with `test-race` + `bench` (~1h)
13. **Phase 3.0c**: race-specific tests (~2h)
14. **Phase 3.0d**: benchmarks — incl. `BenchmarkBool_Cached` 0-alloc proof (~2h)
15. **Phase 3.0e**: edge-case tests (~2h)
16. **Phase 3.2**: safe-default API + `EvaluationDetail` (~1.5 days) — *moved up: the headline DX change, and the eval path it touches was just rewritten in 3.0a*
17. **Phase 3.1**: typed `Context` + `ScopedClient` (~2 days)
18. **Phase 3.3**: offline mode + bootstrap (~2 days)
19. **Phase 3.5**: `StatusProvider` + `WaitForReady` + `OnStatusChange` (~1 day)
20. **Phase 3.4**: `DataStore` (`[]byte`) interface + InMemory + File (~2 days) — *moved down: speculative foundation; bootstrap already covers most of its value*

**Total estimate**: Phases 1+2 = ~2 days (done). Phase 3.0–3.5 = ~11.5 days (proposed). The post-3.5 roadmap (analytics events, variants, `sdktest`, codegen, FlagTracker, OpenFeature provider) is separately scoped.

---

## What makes this SDK "better than competitors" (the explicit differentiators)

**Honest framing first**: Phases 3.0–3.5 get us to *parity of SDK ergonomics* with the best clients, plus a handful of genuine edges below. They do **not** make us "better" on the feature surface that people pay LD/Statsig for — experimentation, exposure analytics, multi-variant assignment (see "What's still missing", next section). So the pitch is not "more features than LD". The pitch is: **open-source, real-time by default, a clean non-blocking API that is hard to misuse, and no lock-in.** That is what makes a developer reach for it on a new project — and keep using it alongside the Flagstone product, not just because of it.

The real edges, each backed by something concrete (not a TODO):

1. **Evaluation never touches the network — and we prove it.** Every `Bool`/`String`/`Number`/`JSON` is a lock-free `atomic.Pointer` load plus an in-memory engine eval, even while a `flag_change` is being applied in the background (3.0a). The differentiator is the *guarantee* (a network blip or a flag toggle can never add latency to your request), defended by `BenchmarkBool_Cached` showing low-ns, 0-alloc, stable-under-write numbers in the README — not a hand-wavy "COW beats futex" claim.

2. **Safe by default — impossible to get a surprise wrong value.** Every eval takes a mandatory default and returns no error on the hot path (3.2a). A missing flag, type mismatch, cold cache, or dead backend yields *your* declared fallback, never a silent zero value or an unchecked error. LD/OpenFeature have this shape; we make the default mandatory so a call site cannot forget it.

3. **One evaluation method, no `*Ctx` twins.** LaunchDarkly ships `BoolVariation` *and* `BoolVariationCtx`. We ship one `Bool(ctx, key, def, evalCtx)`. Half the surface, nothing to choose between.

4. **The engine is a separate importable package.** `pkg/engine` is reusable in isolation — `import .../pkg/engine` and call `engine.Evaluate` directly in a custom tool, with no SDK or network. LD and Unleash bury the engine inside the SDK.

5. **No required daemon.** LD needs the Relay Proxy in some topologies. Ours is optional; the SDK talks straight to the Flagstone server, no extra moving parts.

6. **One wire format end to end.** Bootstrap, the on-disk file, and any `DataStore` all speak the same raw snapshot JSON that the API returns (3.3/3.4). A third-party persistence adapter is genuinely ~50 lines and never imports an SDK internal type. No "you must construct this exact struct" coupling.

7. **OpenFeature-native (3.6, promoted).** We ship an OpenFeature provider as a thin sub-package. A team already standardized on OpenFeature adopts Flagstone by changing one line — and can leave the same way. Betting on portability instead of lock-in is itself the differentiator versus the incumbents.

These are positioning truths, not polish. But see the next section for what we still owe before "better than competitors" is fully earned.

---

## What's still missing to truly beat competitors (post-3.5 roadmap)

Phases 3.0–3.5 close the *ergonomics* gap. These close the *capability* gap — ordered by how much they move the "why would I pick this over LD" needle. None are in the current ~11-day scope; they are written down so the sequencing is deliberate, not forgotten.

1. **Exposure / evaluation analytics events (the experimentation loop).** This is the single biggest gap. Commercial platforms batch and ship "who saw which value" events back to the server; that data powers experiment results, "is this flag still used?" cleanup, and audit. Without it, Flagstone is a *flag delivery* tool, not an *experimentation* platform. Needs: a backend ingestion endpoint, an SDK-side batching/flush buffer (size + interval, drop-on-overflow, flush-on-`Close`), and `PrivateAttributes` (3.1a) actually wired to redact here. **This is what makes `PrivateAttributes` stop being a no-op.** Biggest lift, biggest payoff.

2. **Multi-variant flags + experiments (the deferred 3.7).** The engine already does single-bucket percentage rollout via consistent hashing (`pkg/engine/rollout.go`), but not weighted multi-variant assignment (A/B/C with 33/33/34 splits) or sticky bucketing keyed on `Context.TargetingKey`. Requires a snapshot wire-format change to carry `variants`. Pairs with #1 — variants without exposure events tell you nothing.

3. **A first-class test package (`pkg/sdk/sdktest`).** A fake client where tests set values directly: `c := sdktest.New(); c.SetBool("new-checkout", true)`. Bootstrap (3.3) makes this *possible*; a dedicated test double makes it *pleasant*. Testing flags is normally painful — nailing this is a cheap, loud DX win that makes people recommend the SDK. (LD ships `ldtest`/test data sources for exactly this reason.)

4. **Typed-flag codegen (`flagstone gen`).** Generate Go accessors from the flag definitions: `flags.NewCheckout(ctx, evalCtx)` instead of stringly-typed `Bool(ctx, "new-checkout", …)`. Compile-time safety against typos and type mismatches. Few competitors do this well; it is a memorable differentiator.

5. **`FlagTracker` — subscribe to flag changes.** We already receive `flag_change` over SSE; expose `c.OnFlagChange(key, func(EvaluationDetail){...})` so apps can react (drain a pool, bust a downstream cache) instead of polling. Nearly free given the stream already exists; in the competitor matrix as a missing feature and currently unplanned.

6. **SDK self-metrics (`expvar`/Prometheus, opt-in).** Eval count, cache age, last-fetch latency, SSE connection state. Lets operators see SDK health without adding the heavy OTel dependency (which stays the separate opt-in `otelhook`, 3.8).

7. **Hooks (the deferred 3.6).** Before/After/Error stages. Deliberately after #1 — most of what users want hooks *for* is logging exposures, which #1 delivers natively. Ship the primitive once the obvious use case is already covered.

---

## Phase 4 — Compete, not just match (detailed sub-plans, PROPOSED)

> **Status of the codebase today (verified, not assumed):**
> - `pkg/engine` supports `boolean|string|number|json` flag types, `all/any/not` targeting, reusable segments, and **single-bucket percentage rollout** via FNV hashing on `user_id` (`pkg/engine/rollout.go`, `bucket = h.Sum32() % 100`).
> - **NOT present anywhere** (zero matches for `variant`, `experiment`, `prerequisite`, `sticky`, `holdout`, `schedule` across `internal/`, `pkg/`, `migrations/`): weighted multi-variant assignment, experiments/metrics, exposure analytics, sticky variant assignment, prerequisites, scheduled changes.
> - **Observability today = `zap` logs only.** No OpenTelemetry, no Prometheus, no `/metrics`, no tracing, no spans — not in the SDK, not in the backend. `go.mod` has no telemetry dependency.
>
> So this is the gap between "flag delivery tool" and "experimentation platform". Phase 4 is the path across it. These are cross-cutting (backend + engine + SDK) and sized accordingly.

### 4.1 — Multi-variant flags + exposure events (~6–8 days)

These ship together on purpose: **a variant split with no exposure data is an experiment you cannot read.** Multi-variant is the *assignment*; exposure events are the *measurement*.

#### 4.1a — Engine: weighted variants + sticky bucketing (`pkg/engine`)

Today `RolloutConfig` is binary (in/out of one percentage) and buckets on a hardcoded `user_id` with `% 100` (whole-percent precision only). Multi-variant needs:

```go
// New: a named, weighted outcome.
type Variant struct {
    Key    string `json:"key"`    // "control", "treatment-a", ...
    Value  any    `json:"value"`  // the value served for this variant
    Weight int    `json:"weight"` // per-mille (0–1000); weights in a split sum to 1000
}

// FlagConfig gains:
Variants []Variant `json:"variants,omitempty"`

// Rule gains an alternative to a flat Value: a weighted split.
type Rule struct {
    Conditions   ConditionNode  `json:"conditions"`
    Distribution []WeightedRef  `json:"distribution,omitempty"` // variant key + weight
    VariantKey   string         `json:"variant_key,omitempty"`  // or pin one variant
    Value        any            `json:"value,omitempty"`        // legacy: still works
}
```

**Bucketing changes:**
- Widen the bucket to `% 1000` (per-mille) so a 33.3% split is representable. Keep FNV (stable, dependency-free, already tested).
- **Bucket on `Context.TargetingKey`** (3.1a), falling back to `user_id` for back-compat — not a hardcoded field. This is what makes assignment *sticky*: same key + same seed ⇒ same variant, deterministically, with no server state. (Persisted sticky overrides that survive a weight change are a later, storage-backed feature — noted, not built here.)
- `EvaluateResult.Variant` is populated (the field 3.2 already reserved on `EvaluationDetail`). `Reason` gains `SPLIT` to distinguish "landed in a distribution bucket" from a flat `RULE_MATCH`.

**Why per-mille and not float weights**: integers hash and compare exactly; floats invite drift and "weights sum to 0.999" bugs. Validate `sum == 1000` at write time in the API.

#### 4.1b — Schema + wire format (backend)

- Migration: add `variants JSONB` to `flag_environments` (rules are already JSONB; this matches the existing shape and the `version` auto-increment trigger keeps working — `migrations/000001_init.up.sql:338`).
- `internal/api/sdk_snapshot.go` + `buildFlagConfigFromJoined` include `variants` in the snapshot.
- Bump the snapshot `schema_version` (the field 3.4/risks introduced). Old SDKs ignore the unknown `variants` field (additive); new SDKs require it for multivariate flags.
- API validation: weights sum to 1000; every `distribution[].key` and `variant_key` references a declared variant.

#### 4.1c — Exposure events: backend ingestion

- New endpoint `POST /api/v1/sdk/events` (bulk, `AuthAPIKey`, env-scoped). Body: `[{flag_key, variant, reason, rule_index, targeting_key, ts}]`.
- Storage MVP: an `flag_exposures` table (append-only) + a periodic rollup, **or** stream to the existing audit/analytics path. Keep ingestion dumb and fast; dashboards are a separate deliverable.
- Rate/size limits reuse the existing `body_limit` + `rate_limit` middleware.

#### 4.1d — Exposure events: SDK buffer (`pkg/sdk`)

- Every eval (happy-path and `*Detail`) records an exposure into a bounded in-memory ring buffer.
- A background flusher (own goroutine, same single-writer discipline as 3.0a) ships batches every `FlushInterval` (default 5s) or when the buffer hits `FlushSize`. **Drop-on-overflow** with a counter (surfaced via 4.3 metrics) — never block eval, never grow unbounded.
- `Close()` does a final synchronous flush.
- **Summarization** (like LD's summary events): dedupe identical `(flag, variant, reason)` within the window into a count, so a hot path with 1M evals/sec sends KB, not GB. MVP can send raw and add summarization behind a flag.
- **This is where `PrivateAttributes` (3.1a) finally does its job**: listed attribute keys are stripped from the exposure payload before it leaves the process. Until 4.1 it is a documented no-op; here it becomes real.
- Options: `WithExposureEvents(true)` (default on), `WithFlushInterval`, `WithFlushSize`, `WithExposureSink(sink)` for tests/custom transport.

**Acceptance**: an A/B flag with 50/50 weights, evaluated for 10k distinct targeting keys, lands ~50/50 (±2%); the same key always gets the same variant; the backend receives one summarized batch per interval with private attrs absent.

### 4.2 — Testability: `pkg/sdk/sdktest` + an `Evaluator` interface (~2 days)

The goal: a consumer's tests should set flag values in two lines, with **no network, no Docker, no SSE**, and ideally exercise the *real* engine so the test catches real rule bugs.

#### 4.2a — Extract an `Evaluator` interface (`pkg/sdk`)

```go
// Evaluator is the read surface of Client. User code accepts this so it
// can be swapped for a test double. *Client satisfies it.
type Evaluator interface {
    Bool(ctx context.Context, key string, def bool, evalCtx map[string]any) bool
    String(ctx context.Context, key string, def string, evalCtx map[string]any) string
    Number(ctx context.Context, key string, def float64, evalCtx map[string]any) float64
    JSON(ctx context.Context, key string, def any, evalCtx map[string]any) any
    BoolDetail(...) EvaluationDetail
    // ...String/Number/JSONDetail
}
```

This makes the **entire SDK mockable** by accepting `sdk.Evaluator` in application code instead of `*sdk.Client`. Cheap, idiomatic, and the single most-requested thing from anyone who has tried to unit-test flag-gated code.

#### 4.2b — `sdktest` package, two modes

```go
// Static: flat overrides, no engine. For "this flag is on" unit tests.
td := sdktest.NewStatic()
td.SetBool("new-checkout", true)
td.SetVariant("pricing", "treatment-a") // returns this variant's value + Variant="treatment-a"
svc := NewMyService(td) // td is an sdk.Evaluator

// Snapshot: builds a real offline Client (WithOffline + WithBootstrap) from
// a fixture, so evaluation runs the REAL engine — rules, segments, variants.
c := sdktest.FromSnapshot(testdataSnapshotJSON)
```

- `NewStatic` returns a double that answers from a `map[string]EvaluationDetail` — zero engine, fully deterministic, perfect for table tests.
- `FromSnapshot` reuses 3.3 bootstrap + 3.4 `[]byte` wire format, so the fixture is the *same* JSON the real backend emits. One format, again.
- Helpers: `SetBoolForKey`/`...ForContext(matcher)` for per-context overrides; `Reset()`; `Calls()` to assert which flags were read (exposure-style spy) — great for "did we even check this flag?" tests.

**Acceptance**: a downstream repo can unit-test flag-gated branches with `sdktest.NewStatic()` and no build tags, no network; and `FromSnapshot` correctly resolves a segment+variant rule using the production engine.

### 4.3 — Observability: light core, opt-in OpenTelemetry (~2–3 days)

**Principle (keep the core dependency-free):** the base SDK pulls in **no** telemetry libraries — today it is `zap` only, and that stays true. We expose a tiny zero-dependency `Observer` seam in core, and ship OTel as a **separate module** so only users who want it pay the ~1–2 MB transitive cost.

#### 4.3a — Core: a zero-dep `Observer` seam (`pkg/sdk`)

```go
// Observer receives SDK lifecycle + eval signals. All methods must be
// non-blocking. Default is a no-op. Zero external dependencies.
type Observer interface {
    OnEval(key string, detail EvaluationDetail, dur time.Duration)
    OnFetch(dur time.Duration, err error)
    OnStateChange(from, to State)         // ties into 3.5
    OnExposureDrop(count int)             // ties into 4.1d backpressure
}
func WithObserver(o Observer) Option
```

Also expose dependency-free counters via `expvar` (eval count by reason/variant, cache age, last-fetch latency, SSE state, exposure-buffer depth + drops) so operators get numbers with **zero** added deps.

#### 4.3b — Opt-in OTel adapter (separate module `pkg/sdk/otel` or its own go.mod)

- A `metric.Meter`-backed `Observer` implementation: counters/histograms for the signals above.
- A tracing helper that opens a span per eval (or annotates the active span with `feature_flag.key`, `feature_flag.variant`, `feature_flag.reason`) — this aligns with the **OpenTelemetry semantic conventions for feature flags**, which is a real "we did our homework" signal to enterprise buyers.
- Imported only by users who want it: `import flagstoneotel ".../pkg/sdk/otel"; sdk.New(..., sdk.WithObserver(flagstoneotel.New(meter, tracer)))`.

#### 4.3c — Backend observability: OpenTelemetry is a first-class citizen (COMMITTED)

**Decision**: OTel is **first-class and always-on in the backend** — the asymmetry with the SDK is deliberate and is the whole point. We own the server deployment, so we instrument it fully: HTTP middleware spans (slot into the existing `internal/api/middleware/` chain), pgx query spans, the streaming hub, and a `/metrics` (or OTLP) exporter. Operators get traces and metrics without enabling anything.

The SDK is the opposite — **opt-in** — because we do *not* own the user's process and cannot force the ~1–2 MB OTel dependency tree onto a latency-sensitive service. So: **backend = OTel built in; SDK = zero telemetry deps in core, OTel via the opt-in adapter (4.3b).** Both ends emit the OTel feature-flag semantic conventions, so a flag evaluated in an SDK-instrumented app and the backend that served it correlate in the same trace.

**Why the split is the right call**: the #1 complaint about heavy SDKs is dependency bloat — a flags SDK that drags in the entire OTel tree by default is a non-starter. The backend has no such constraint, so there is no reason to make its observability optional. Lean SDK core, fully-instrumented server.

This backend work is its own deliverable (it does not block the SDK roadmap) but it is **committed, not "maybe later"**: ~3–4 days for spans + metrics + exporter wiring.

### Phase 4 execution order & estimate

1. **4.1a** engine variants + per-mille sticky bucketing (~2d)
2. **4.1b** schema + snapshot wire format + validation (~1.5d)
3. **4.1c** exposure ingestion endpoint + storage (~1.5d)
4. **4.1d** SDK exposure buffer + summarization + `PrivateAttributes` redaction (~2d)
5. **4.2a/b** `Evaluator` interface + `sdktest` (~2d)
6. **4.3a** core `Observer` seam + `expvar` (~1d)
7. **4.3b** opt-in OTel adapter module (~1.5d)

**Total estimate**: Phase 4 ≈ 11–12 days (SDK/backend) + the 4.W frontend track below. Sequence after 3.0–3.5, since 4.1 depends on the typed `Context`/`TargetingKey` (3.1), the default API (3.2), and the `[]byte` wire format (3.3/3.4); 4.2 depends on the default-API signatures; 4.3a's `OnStateChange` depends on the 3.5 state machine.

### 4.W — Frontend track (the features must be *visible*, not just shipped)

**Why this is its own track**: a feature-flag platform is judged by its dashboard. Today `web/` has flags, segments, rules (with a binary `rollout-input`), API keys, and audit — and **nothing** for variants, experiments, or analytics. Backend features with no UI are invisible to the buyer. Each Phase 4 (and 5/6) deliverable carries matching web work of comparable size.

| Backend/SDK feature | Frontend deliverable | Notes |
|---|---|---|
| 4.1a/b multi-variant | **Variant editor**: add/remove variants (key/value), a weight allocator that enforces sum = 100% (per-mille under the hood), and a per-rule "serve variant / split traffic" toggle. Replaces the binary `rollout-input`. | Reuses `components/rules/rule-editor.tsx`. |
| 4.1c/d exposure events | **Per-flag usage panel**: evals over time, breakdown by variant and reason, last-seen timestamp. | New `components/flags/usage/`. Feeds 6.1 (dead-flag detection). |
| 4.3 observability | No bespoke UI — deep-link to the operator's Grafana/OTel backend instead. | Avoids rebuilding a metrics product. |

**Estimate**: ~5–6 days of frontend, parallelizable with the backend work once the wire format (4.1b) is frozen.

---

## Competitive strategy — the explicit positioning

**The one-line thesis**: *match the incumbents on delivery + targeting (table stakes to be considered), and win where LD/Statsig actively hurt their users — cost, data privacy, and flag lifecycle.* We do not try to out-sophisticate Statsig's statistics on day one; we make the data stay in your infra and make dead flags clean themselves up.

**Where the competitors hurt (the pain we sell against):**
- **LaunchDarkly** — priced per-seat + MAU, so it punishes exactly the teams that succeed and scale; heavyweight SDKs; complex setup.
- **Unleash** — weak experimentation/stats, thin analytics, **polling** updates (we already push via SSE).
- **Statsig** — powerful but a black box; **your data goes to their cloud** (a hard blocker for fintech/health/EU); warehouse-native is enterprise-tier only.

**Our axes:**
1. **Open-source + self-host, data stays in your DB** — privacy/GDPR and cost predictability as product features, not afterthoughts. Direct counter to Statsig.
2. **Real-time by default (SSE)** + a lean, non-blocking, hard-to-misuse SDK (Phase 3) — counter to Unleash's polling and LD's SDK bloat.
3. **No lock-in** — OpenFeature-native, importable readable engine.
4. **Lifecycle built in** — dead-flag detection and code references (Phase 6) attack the #1 operational pain *nobody* solves well: flag debt.

This section is the "why us" that the README, the landing page, and the sales narrative all derive from. Everything in Phases 5 and 6 maps back to one of these four axes.

---

## Phase 5 — Experimentation & statistics engine (PROPOSED, compete head-on)

**Decision (confirmed): build the full statistics layer**, not just raw-data plumbing. This is the third layer of A/B testing — *assignment* (4.1a) and *measurement* (4.1c/d) are the plumbing; this is the **results** layer that makes the difference between "we have variants" and "we have an experimentation product".

### 5.1 — Experiment entity + lifecycle (backend)

- New `experiments` table: name, hypothesis, the flag + variants under test, the **primary metric**, optional **guardrail metrics**, start/stop, status (`draft|running|stopped`).
- An experiment binds a flag's variants to metrics and a time window. Stopping freezes the readout.

### 5.2 — Metrics ingestion (backend + SDK)

- Beyond exposure events (4.1), the SDK/app reports **metric events**: `client.Track(ctx, "checkout_completed", value, evalCtx)` → `POST /api/v1/sdk/events` (same bulk path, different event kind).
- Backend joins exposures (who saw which variant) with metric events (who converted) by `targeting_key`.

### 5.3 — Statistics engine (backend, the hard part)

- **Frequentist**: per-variant conversion/mean, standard error, two-tailed significance vs control, confidence intervals, relative lift. Correct handling of ratio metrics and CUPED-style variance reduction is a *future* refinement, not v1.
- **Sequential testing / peeking correction**: at minimum warn against early stopping; ideally an always-valid sequential test so the dashboard can be checked anytime without inflating false positives. (This is exactly the trap Statsig markets against — getting it right is credibility.)
- **Guardrails**: flag a variant that significantly *regresses* a guardrail metric.
- Pure-Go, deterministic, unit-tested against known datasets. No external stats service.

### 5.4 — Results UI (frontend)

- Experiment dashboard: per-variant metric values with CIs, significance badges, lift, sample size / power, a "safe to call it?" indicator, and guardrail status. Time series of the metric per variant.

**Estimate**: Phase 5 ≈ 8–10 days (5.3 is the bulk). Hard dependency on Phase 4 (variants + exposure + metric events). **Risk**: a *wrong* stats engine is worse than none — it loses trust permanently. 5.3 must ship with a validation suite against reference datasets and a statistician-reviewed methodology doc before it is exposed in the UI.

---

## Phase 6 — Differentiation bets (PROPOSED, the net-new wins)

The four features below are not parity items — competitors do them poorly or not at all, they solve real operational pain, and three of the four are **nearly free byproducts of data we already collect in Phase 4**. All four were explicitly chosen as priorities.

### 6.1 — Dead-flag detection / lifecycle (~3 days; mostly free after 4.1d)

- Using exposure events (4.1d): surface flags whose evaluations have returned a single value for 100% of traffic over a configurable window ("stale"), or that have not been evaluated at all ("orphaned").
- UI: a "Flag health" view + a badge on the flags table; one-click archive with an audit entry.
- **Why it wins**: every team drowns in flag debt; nobody cleans it up because nobody can *see* it. This turns the data we already have into the answer to "is this flag safe to delete?".

### 6.2 — Code references (~4 days)

- A `flagstone refs` CLI (and/or a CI action) that greps a repo for flag keys and reports where each is used — and which keys in the codebase no longer exist in Flagstone (and vice-versa: defined-but-unused).
- UI: per-flag "Used in" panel linking to file/line in the user's VCS.
- **Why it wins**: pairs with 6.1 to answer "can I delete this flag without breaking the build?" — LD has this behind enterprise tiers; we ship it open.

### 6.3 — Data residency / privacy as a product (~2 days, mostly positioning + hardening)

- Guarantee and **document** that exposures, metric events, and targeting attributes never leave the self-hosted instance. `PrivateAttributes` (3.1a/4.1d) enforced end-to-end.
- A "self-host hardening" guide + a data-flow diagram for compliance reviews (the artifact fintech/health/EU buyers actually ask for).
- **Why it wins**: a hard blocker against Statsig's cloud model; turns "we're open-source" into a concrete compliance answer.

### 6.4 — Change governance / approvals (~4 days)

- Optional approval workflow on flag-environment changes in protected environments (e.g. `production`): a change becomes a pending request requiring a second approver before it applies. Reuses the existing RBAC (`internal/api/middleware/rbac.go`) and audit trail.
- UI: pending-changes queue, approve/reject with comment.
- **Why it wins**: classic enterprise pain the OSS tools cover badly; a gateway to larger orgs.

**Phase 6 ordering**: 6.1 first (highest pain-to-effort ratio, unlocked by 4.1d), then 6.2 (completes the lifecycle story with 6.1), then 6.3 (positioning/hardening), then 6.4 (enterprise gate). Estimate ≈ 13 days total.

---

## Release & go-to-market staging — "when do we compete / ship / go public?"

**The key judgment: do NOT wait for the whole plan.** "Compete" is not one finish line — it is a staircase, and each step is a shippable, claimable milestone. More importantly, **the repo should go public at the end of Phase 3, not at the end of Phase 6.** Waiting until everything is polished forfeits the two things open-source gives you early — community feedback and momentum — and means building 4/5/6 in the dark instead of in the open. The only hard rule (the plan's own principle from 3.0a) is: **never go public with a known data race.** Phase 3 removes it; that is the gate.

### Version milestones

| Version | Ships at end of | What you can honestly claim | Who you compete with |
|---|---|---|---|
| **v0.3.0** | **Phase 3** | "A real-time Go SDK that never blocks your request path, is safe-by-default, and is OpenFeature-native." A credible, race-free OSS SDK. | Nobody yet — this is the *credibility* bar, the price of being taken seriously. |
| **v0.4.0** | Phase 4 | "Multi-variant flags, exposure analytics, self-hosted — your data stays in your DB." A *platform*, not just delivery. | **Unleash** (you now beat it on streaming + variants + analytics). |
| **v0.5.0** | Phase 5 | "Built-in experimentation with a real statistics engine (significance, guardrails, peeking-safe)." | **Statsig / LaunchDarkly experimentation** — head-on. |
| **v1.0.0** | Phase 6 | "Flag lifecycle that cleans itself up, code references, governance, data residency." The differentiated, stable platform. | The whole field, on *our* axes (cost, privacy, lifecycle). |

**So the answer to "when do we compete?":** you start competing at **v0.4.0** (vs Unleash), seriously at **v0.5.0** (vs Statsig), and win on differentiation at **v1.0.0**. v0.3.0 is not a competitive release — it is the entry ticket. Trying to "compete" before v0.4.0 is premature; refusing to *ship* until v1.0.0 is the bigger mistake.

### Go-public gate (end of Phase 3) — readiness checklist

Going public is hard to reverse (forks, indexed history, first-impression scrutiny). Treat it as a one-way door with a checklist, not a flip of the repo's visibility toggle:

- [ ] **3.0 done**: `go test -race ./...` clean, benchmarks published. No known race. (Hard gate.)
- [ ] **Secret scan of full git history** (not just HEAD) — `gitleaks`/`trufflehog`. A public repo exposes every past commit.
- [ ] **License chosen and applied** (Apache-2.0 or MIT for max adoption; if a commercial tier is planned, decide on BSL/AGPL *now* — relicensing after public is painful).
- [ ] **`CONTRIBUTING.md`, `CODE_OF_CONDUCT.md`, `SECURITY.md`** (a private disclosure path before attackers find issues publicly).
- [ ] **CI green on a clean clone** + re-enable CodeQL auto-triggers (currently disabled "until repo is public" per `7641981` — flipping public unlocks GHAS on a public repo).
- [ ] **README that sells the v0.3.0 story** (the Competitive-strategy thesis, quickstart, the SDK example that compiles).
- [ ] **`/security-review` pass** on the public surface (auth, API-key handling, SSE endpoint, the new `/sdk/events` ingest once 4.1 lands).
- [ ] **Versioned, tagged `v0.3.0`** with a real CHANGELOG entry and a `pkg/engine` + `pkg/sdk` API-stability note.

### Definition of Done per phase (so "compete" is measurable, not vibes)

- **Phase 3**: race-free, non-blocking eval proven by a 0-alloc benchmark; safe-default API; offline/bootstrap; status/readiness. → *ship v0.3.0, go public.*
- **Phase 4**: a 50/50 variant flag splits correctly and stickily; exposure events land in your DB summarized and with private attrs stripped; backend OTel traces a request end-to-end; `sdktest` lets a downstream repo unit-test a flag in two lines; the variant editor + usage panel are live in `web/`. → *v0.4.0.*
- **Phase 5**: an experiment reads out significance + CIs + guardrails, validated against reference datasets with a reviewed methodology doc. → *v0.5.0.*
- **Phase 6**: dead-flag detection surfaces a real stale flag in the demo data; `flagstone refs` maps keys to code; a data-flow/compliance doc exists; protected-env approvals work. → *v1.0.0.*

**One caution on Phase 5 timing**: the statistics engine is the only deliverable where shipping *early and wrong* is worse than shipping *late*. A bogus significance number that costs someone a real product decision destroys trust permanently. v0.5.0 must clear its validation suite before it is exposed — even if that slips the date.

---

## Appendix A — Implementation spec for 4.1a/b (multi-variant), ready to code

This is the design-decision-complete spec for the first block that gets coded. It is grounded in today's engine (`pkg/engine/engine.go:50-81`, `rollout.go`, `types.go`). Goal: someone can start implementing without making further design calls.

### A.1 — Engine type changes (`pkg/engine/types.go`)

```go
// New: a named, weighted outcome of a flag.
type Variant struct {
    Key    string `json:"key"`    // "control", "treatment_a", ...
    Value  any    `json:"value"`  // the value served when this variant is assigned
    Weight int    `json:"weight"` // per-mille, 0–1000; weights of a Distribution sum to 1000
}

// FlagConfig gains (additive — old snapshots without it decode to nil):
type FlagConfig struct {
    // ...existing fields unchanged...
    Variants []Variant `json:"variants,omitempty"`
}

// Rule gains a third way to resolve its served value, alongside the
// existing flat Value and binary Rollout. Exactly one of
// {Value, Rollout, Distribution} is the resolution mode; validated at
// write time (A.4). Distribution always assigns SOME variant, so it
// never "falls through".
type Rule struct {
    Conditions   ConditionNode  `json:"conditions"`
    Rollout      *RolloutConfig `json:"rollout,omitempty"`      // legacy binary in/out
    Distribution []WeightedRef  `json:"distribution,omitempty"` // NEW: weighted split
    Value        any            `json:"value,omitempty"`        // legacy flat value
}

// WeightedRef points a slice of traffic at a declared Variant.
type WeightedRef struct {
    VariantKey string `json:"variant_key"`
    Weight     int    `json:"weight"` // per-mille; the WeightedRefs in a Rule sum to 1000
}

// EvaluateResult gains Variant (empty when no variant was assigned).
type EvaluateResult struct {
    Value     any
    Reason    Reason
    RuleIndex int
    Variant   string // NEW: assigned variant key, "" for flat/default outcomes
}

// New reason, to distinguish a weighted-split assignment from a flat match.
const ReasonSplit Reason = "SPLIT"
```

### A.2 — Bucketing (`pkg/engine/rollout.go`)

Today `inRollout` buckets `% 100` on a hardcoded `user_id`. Multi-variant needs per-mille precision and a configurable bucketing key. Keep FNV (stable, dependency-free, already tested); **do not** change the existing `inRollout` (legacy binary rollout keeps its exact behavior and tests). Add:

```go
// bucketOf returns a stable bucket in [0,1000) for (seed, key). Same
// (seed, key) ⇒ same bucket forever ⇒ assignment is sticky with no
// server state. Seed defaults to the flag key (caller passes it).
func bucketOf(seed, key string) uint32 {
    h := fnv.New32a()
    _, _ = h.Write([]byte(seed + ":" + key))
    return h.Sum32() % 1000
}

// pickVariant walks cumulative weights (assumed validated to sum 1000)
// and returns the variant key whose band contains bucket.
func pickVariant(dist []WeightedRef, bucket uint32) string {
    var cum uint32
    for _, ref := range dist {
        cum += uint32(ref.Weight)
        if bucket < cum {
            return ref.VariantKey
        }
    }
    return dist[len(dist)-1].VariantKey // float-safety fallback; unreachable if sum==1000
}
```

**Bucketing key precedence**: `ctx["targeting_key"]` if present (set by the SDK's `Context.asMap()`, 3.1a), else `ctx["user_id"]` (back-compat). One helper:

```go
func bucketingKey(ctx map[string]any) string {
    if v, ok := ctx["targeting_key"].(string); ok && v != "" { return v }
    if v, ok := ctx["user_id"].(string); ok { return v }
    return ""
}
```

### A.3 — Engine evaluation change (`pkg/engine/engine.go`)

Insert the Distribution branch in the per-rule loop, **before** the existing `Rollout` handling, after the conditions match (replacing the block at `engine.go:55-80`):

```go
key := bucketingKey(req.Context) // was: userID, _ := req.Context["user_id"].(string)
for i, rule := range fc.Rules {
    if !e.evaluateNode(rule.Conditions, req.Context, req.Segments, map[string]struct{}{}) {
        continue
    }

    // NEW: weighted multi-variant split.
    if len(rule.Distribution) > 0 {
        if key == "" {
            // Consistent with legacy rollout (engine.go:62): a split rule
            // needs a bucketing key. Without one it cannot assign fairly,
            // so it does not apply — fall through to the next rule.
            e.logger.Debug("split rule skipped: no bucketing key",
                zap.String("flag_key", fc.Key), zap.Int("rule_index", i))
            continue
        }
        seed := fc.Key // (optional per-rule seed override can be added later)
        vkey := pickVariant(rule.Distribution, bucketOf(seed, key))
        v, ok := fc.variant(vkey)
        if !ok { // misconfig: referenced a variant that doesn't exist
            e.logger.Warn("split references unknown variant",
                zap.String("flag_key", fc.Key), zap.String("variant", vkey))
            continue
        }
        return EvaluateResult{Value: v.Value, Reason: ReasonSplit, RuleIndex: i, Variant: vkey}
    }

    // ...existing Rollout (binary) and flat-Value branches unchanged...
}
```

Plus a small lookup helper:

```go
func (fc FlagConfig) variant(key string) (Variant, bool) {
    for _, v := range fc.Variants {
        if v.Key == key { return v, true }
    }
    return Variant{}, false
}
```

**Stickiness & its known limit**: assignment is deterministic in `(flag key, targeting key)`, so a user keeps their variant across evals and across snapshot refreshes with zero state. If the *weights* change, buckets near a band boundary reassign — acceptable for v1; persistent sticky overrides that survive a weight change are deferred (noted in 4.1a) and would need a DataStore-backed assignment log.

### A.4 — Schema + wire format + validation (backend)

- **Migration `migrations/000002_variants.up.sql`**: `ALTER TABLE flag_environments ADD COLUMN variants JSONB NOT NULL DEFAULT '[]';`. The `Distribution`/`WeightedRef` live *inside* the existing rules JSON, so no new column for them. The `version` auto-increment trigger (`000001_init.up.sql:338`) keeps working untouched. Provide the matching `.down.sql` (`DROP COLUMN variants`).
- **Snapshot** (`internal/api/sdk_snapshot.go` + `buildFlagConfigFromJoined`): include `variants` in the per-flag JSON. Bump the snapshot `schema_version` (additive — old SDKs ignore the field).
- **Write-time validation** (flag-environment create/update handler): (1) every `WeightedRef.VariantKey` and the variants in a flag reference a key that exists in `variants`; (2) the weights of each `Distribution` sum to **exactly 1000**; (3) a rule sets exactly one of `Value` / `Rollout` / `Distribution`; (4) all `Variant.Value`s match the flag's `flag_type`. Reject with 422 + a precise message otherwise — never store an un-evaluable flag.

### A.5 — SDK wiring (`pkg/sdk`)

- `flagEnvConfigJSON` (`types.go`) gains `Variants []engine.Variant`; `toSnapshot()` copies it into `engine.FlagConfig.Variants`.
- `EvaluationDetail.Variant` (3.2b, already reserved) is populated from `EvaluateResult.Variant`. No happy-path signature change.
- `sdktest.SetVariant(key, variantKey)` (4.2b) builds a fixture flag whose single rule is a 100%-weight `Distribution` to that variant — so tests can pin a variant deterministically.

### A.6 — Tests (engine)

- `TestSplit_DistributionIsDeterministicAndSticky`: same `(flag, targeting_key)` ⇒ same variant across 1000 calls.
- `TestSplit_50_50_DistributesEvenly`: 10k distinct keys land ≈50/50 (±2%).
- `TestSplit_NoBucketingKey_Skips`: missing `targeting_key`/`user_id` ⇒ rule skipped, falls through to default.
- `TestSplit_UnknownVariantRef_Skips`: a `WeightedRef` to a missing variant ⇒ skip + warn, never panic.
- `TestSplit_LegacyRolloutUnchanged`: existing binary-rollout flags evaluate identically (regression guard on `inRollout`).

This appendix is the unblocker for the entire variant + experiment line (Phases 4.1, 5). Everything downstream — exposure events keyed by `Variant`, experiment readouts, the variant editor UI — consumes the `Variant` field this produces.
