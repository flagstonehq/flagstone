# Plan: Demoable App + Go SDK with SSE

> **Status:** Approved, not yet implemented.
> **Scope:** Phase 1 (demoable app) + Phase 2 (SSE backend + Go SDK).
> **Estimated effort:** ~2 focused days.

---

## Context and rationale

A full inventory of the repository surfaced these facts, which dictate the order of work:

- **`internal/engine/` is 100% pure Go**: zero DB/HTTP imports, only `zap` for logging. It is trivially SDK-reusable.
- **The backend already exposes** `POST /api/v1/evaluate/flags/{key}` and `POST /api/v1/evaluate/flags`, gated by API-key auth. There is no `/stream` endpoint yet.
- **SSE is designed in `DESIGN.md`** (lines 1029-1387) but never implemented: `internal/streaming/` is an empty directory with a `.gitkeep`.
- **There is no seed data**: `admin@acme.com` / `password123` are referenced in 8 Playwright specs and in `setup_test.go`, but **they are never provisioned anywhere**. The only bootstrap path is `POST /api/v1/setup` with arbitrary data.
- **`docker-compose.yml` only starts Postgres and Redis**. A developer must run `make run` and `npm run dev` in two separate terminals.
- **The Go SDK example in the README does not compile**: it imports `github.com/thomas-vilte/flagstone/pkg/sdk`, which is an empty directory.
- **`flag_environments.version` already exists** with an auto-increment trigger at `migrations/000001_init.up.sql:338-347`. Optimistic concurrency control is already wired; SSE will simply expose it.
- **DESIGN.md does not specify a `segment_change` event**, but the SDK needs one to invalidate cached segments when their rules change. We extend the design with justification.

**Why we start with Phase 1**: without seed data and without the `api` service in `docker-compose.yml`, it is impossible to test the app as a user. Without testing it as a user, we cannot know whether the UX is sound before investing in SDKs. This is the order of least regret.

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
git clone https://github.com/thomas-vilte/flagstone
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
import "github.com/thomas-vilte/flagstone/pkg/sdk"

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
