# Flagstone — Plan de Implementacion Milestone 1

> Plan detallado, justificado y con checklist para el desarrollo del MVP local.
> Cada fase incluye sus tests. Se avanza fase por fase en orden.

---

## Tabla de Contenidos

1. [Estado Actual del Proyecto](#1-estado-actual-del-proyecto)
2. [Decisiones Tecnicas del Plan](#2-decisiones-tecnicas-del-plan)
3. [Dependencias y Herramientas](#3-dependencias-y-herramientas)
4. [Fase 0 — Foundation](#4-fase-0--foundation)
5. [Fase 1 — Storage Layer](#5-fase-1--storage-layer)
6. [Fase 2 — Auth Core](#6-fase-2--auth-core)
7. [Fase 3 — Middleware + Router + Bootstrap](#7-fase-3--middleware--router--bootstrap)
8. [Fase 4 — Auth Endpoints](#8-fase-4--auth-endpoints)
9. [Fase 4.5 — Engine Spike (validacion de modelo)](#85-fase-45--engine-spike-validacion-de-modelo)
10. [Fase 5 — CRUD Endpoints](#9-fase-5--crud-endpoints)
11. [Fase 7 — Rule Evaluation Engine](#11-fase-7--rule-evaluation-engine)
12. [Fase 8 — Evaluate Endpoints](#12-fase-8--evaluate-endpoints)
13. [Fase 6 — Audit Log Endpoint](#10-fase-6--audit-log-endpoint)
14. [Fase 9 — CI Pipeline + Integracion Final](#13-fase-9--ci-pipeline--integracion-final)
15. [Dependencias entre Fases](#14-dependencias-entre-fases)
16. [Estimacion de Esfuerzo](#15-estimacion-de-esfuerzo)

> **Nota de orden**: el orden de ejecucion recomendado es 0 → 1 → 2 → 3 → 4 → **4.5 (spike)** → 5 → 7 → 8 → 6 → 9. La numeracion de fases se mantiene por compatibilidad con commits/PRs ya hechos, pero las dependencias reales se describen en la seccion 14.

---

## 1. Estado Actual del Proyecto

### Lo que YA existe y funciona

| Componente | Estado | Detalle |
|---|---|---|
| Documentacion (README, DESIGN, SECURITY, BUSINESS) | Completo | 569 + 2222 + 771 + 313 lineas |
| Schema de DB (2 migraciones, 13 tablas) | Completo | `000001_init.up.sql` (350 lines), `000002_email_flows.up.sql` (82 lines) |
| Entry point (`cmd/flagstone/main.go`) | Minimo | `/healthz` con graceful shutdown, build-time version injection |
| Config loader (`internal/config/config.go`) | Completo | Lee env vars, valida `DATABASE_URL` y `JWT_SECRET` |
| Dockerfile | Completo | Multi-stage, non-root user, ~25MB final |
| Makefile | Completo | 15 targets: build, run, test, migrate, setup, etc. |
| Terraform AWS | Completo | VPC, EC2 t4g.small, RDS PostgreSQL 16, IAM, security groups |
| Linter config (`.golangci.yml`) | Completo | govet, errcheck, staticcheck, gosec, revive, gocritic |
| Diagramas de arquitectura | 7 PNGs | ERD, auth, eval flow, pub/sub, AWS, API, observability |

### Lo que FALTA implementar (todo el codigo de aplicacion)

| Componente | Estado |
|---|---|
| `docker-compose.yml` | Completo | Postgres 16 + Redis 7, healthchecks, volumes |
| `go.sum` | Completo | Dependencias resueltas |
| `main.go` con config + readyz | Completo | Integrado con `config.Load()`, connectWithRetry, pool, `/readyz` |
| `internal/storage/` | Vacio (solo `.gitkeep`) |
| `internal/auth/` | Vacio (solo `.gitkeep`) |
| `internal/api/` | Vacio (solo `.gitkeep`) |
| `internal/engine/` | Vacio (solo `.gitkeep`) |
| `internal/streaming/` | Vacio (solo `.gitkeep`) |
| `internal/telemetry/` | Vacio (solo `.gitkeep`) |
| `pkg/sdk/` | Vacio (solo `.gitkeep`) |
| `web/` | Vacio (Milestone 2) |
| Tests | Cero archivos `*_test.go` |
| CI pipeline | `.github/workflows/ci.yml` existe pero todo comentado |

### Cambios recientes en documentacion (incorporados en este plan)

- **Logging**: `go.uber.org/zap` confirmado como libreria de logging (no `log/slog`). Justificacion: alta performance en el hot path de evaluacion.
- **Web dashboard**: Next.js 16.2.6 + React 19.2.6 + TypeScript + Tailwind + shadcn/ui (Milestone 2). Container separado del API. Se usa la version mas reciente por las 13 vulnerabilidades parcheadas en mayo 2026, incluyendo RCE pre-autenticacion en React Server Components (CVE-2025-55182, CVSS 9.5) y bypass de autorizacion en Middleware (CVE-2025-29927, CVSS 9.5). Vercel confirmo que no hay mitigacion via WAF — el unico fix es actualizar.
- **Contenedores separados**: `Dockerfile.api` (Go, ~25MB) + `web/Dockerfile` (Next.js, ~120MB). Un solo `docker-compose.yml` para self-host.
- **Go version**: `go 1.26.2` (confirmado en `go.mod`). El Dockerfile usa `golang:1.23-alpine` como builder — se actualizara cuando la imagen oficial soporte 1.26.

---

## 2. Decisiones Tecnicas del Plan

### Go 1.26

El proyecto usa Go 1.26.2. El `go.mod` ya lo refleja. El Dockerfile builder usa `golang:1.23-alpine` porque esa es la ultima imagen Alpine disponible; se actualizara a `golang:1.26-alpine` cuando exista. Mientras tanto, el build funciona correctamente (Go es compatible hacia adelante dentro de la misma major version para este tipo de proyecto).

### Tests con docker-compose (NO testcontainers)

**Decision**: Los tests de integracion usan `docker-compose.yml` para levantar Postgres y Redis reales, NO `testcontainers-go`.

**Justificacion**:
- `testcontainers-go` agrega una dependencia pesada (~30 transitive deps) que complica el build y el binary size.
- `docker-compose` ya es requerido para desarrollo local — no agrega nada nuevo al entorno del developer.
- El Makefile ya tiene targets `setup` / `down` que usan docker-compose.
- Los tests de integracion se ejecutan contra los mismos servicios que el developer usa localmente — misma superficie de prueba.
- CI puede usar docker-compose con el mismo `docker-compose.yml` (o un variante `docker-compose.ci.yml`).

**Como funciona**:
1. `make setup` levanta Postgres + Redis via docker-compose
2. `make test-int` corre los tests contra esos servicios
3. `make down` limpia todo
4. En CI: se levantan los servicios como parte del workflow, se corren tests, se limpian

**Tests unitarios** (engine, auth core, middleware) NO necesitan docker-compose — son funciones puras sin I/O.

### Stores separados por entidad (NO monolitico)

**Decision**: Cada tabla tiene su propio store struct (`TenantStore`, `UserStore`, etc.), agrupados en un container `Stores`.

**Justificacion**:
- Cada store es independiente y testeable en aislamiento.
- El `Stores` container se inyecta una sola vez en `main.go` y se pasa a los handlers.
- Si un store crece mucho, es facil extraerlo a su propio paquete sin romper los demas.
- Alternativa monolitica (`type Store struct { pool *pgxpool.Pool }` con 50 metodos): dificil de navegar, tests mezclados, violations de SRP.

### El engine es puro (sin I/O)

**Decision**: `internal/engine/` no importa ningun paquete de storage, HTTP, o red.

**Justificacion**:
- El engine es la parte mas compleja algoritmica del sistema. Si tiene I/O, los tests requieren mocks de DB.
- Ser puro lo hace trivialmente testeable: input -> output, sin side effects.
- La storage layer hace todo el I/O (eager load de flags + segments) y le pasa structs al engine.
- Esta separacion esta explicitamente disenada en DESIGN.md.

### Engine spike antes de CRUDs (orden de fases)

**Decision**: despues de cerrar la Fase 4 (auth endpoints), corre una **Fase 4.5 — Engine Spike** antes de empezar la Fase 5 (CRUDs). Es un paquete `internal/engine/` puro Go, sin HTTP ni DB, que se construye contra structs de test en memoria. Solo despues de validar el modelo se hacen los CRUDs.

**Justificacion**:
- La parte mas riesgosa algoritmicamente del sistema es la evaluacion (operadores, segment resolution con cycle detection, rollout hashing, precedence de reglas).
- Los CRUDs cementan en HTTP la forma del modelo (`Flag`, `FlagEnvironment.Rules` JSONB, `Segment.Rules` JSONB). Si el engine descubre que esa forma no aguanta lo que necesita, terminas reescribiendo handlers ya entregados.
- Hacer un spike pequeno del engine (~3-5 dias) valida el modelo antes de comprometerlo en la API publica.
- Si el spike fuerza cambios al schema, se crea una migracion 000004 y los stores se ajustan ANTES de tocar handlers.
- Despues del spike, la Fase 7 (Engine completo) productioniza lo que el spike ya valido: panic recovery, los 15 operadores completos, performance, edge cases extra.

**Trade-off**: agrega ~3-5 dias antes de empezar CRUDs. El downside es que si confias plenamente en que el modelo actual es correcto, podes saltearlo y arrancar Fase 5 directo. El upside es evitar una reescritura grande de handlers si el modelo no aguanta.

### Middleware stack order (load-bearing)

```
RecoverPanic -> RequestID -> Logger -> [Auth] -> [RBAC] -> [BodyLimit] -> [ContentType] -> Handler
```

**Justificacion** (de DESIGN.md):
- `RecoverPanic` es el outermost — atrapa todo.
- `RequestID` se genera antes de cualquier log para correlacion.
- `Logger` usa el request_id inyectado.
- `Auth` va antes de `RBAC` (necesitas saber quien eres antes de saber que puedes hacer).
- `BodyLimit` y `ContentType` van antes de leer el body en el handler.

---

## 3. Dependencias y Herramientas

### Go modules a agregar

| Paquete | Version | Para que | Fase |
|---|---|---|---|
| `github.com/jackc/pgx/v5` | latest | Postgres driver + connection pool | 0 |
| `github.com/redis/go-redis/v9` | latest | Redis client | 0 |
| `github.com/golang-jwt/jwt/v5` | latest | JWT sign/validate (HS256) | 2 |
| `golang.org/x/crypto` | latest | bcrypt password hashing | 2 |
| `golang.org/x/sync` | latest | `singleflight` (cache stampede prevention) | 1 |
| `github.com/stretchr/testify` | latest | Test assertions (`require`, `assert`) | 1 |

### Herramientas externas (dev machine)

| Herramienta | Para que | Instalacion |
|---|---|---|
| Docker + Docker Compose | Postgres + Redis local | `docker compose version` |
| golang-migrate | DB migrations | `go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest` |
| golangci-lint | Linting | `go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest` |
| goimports | Auto-format + imports | `go install golang.org/x/tools/cmd/goimports@latest` |

---

## 4. Fase 0 — Foundation

**Objetivo:** `make setup && make migrate && make run` funciona. El servidor responde en `/healthz` y `/readyz`.

### Archivos

| Archivo | Accion | Lineas aprox |
|---|---|---|
| `docker-compose.yml` | **Crear** | ~40 |
| `go.mod` | **Editar** — agregar dependencias | +6 |
| `go.sum` | **Generar** via `go mod tidy` | auto |
| `cmd/flagstone/main.go` | **Editar** — integrar config, pool, readyz | ~60 nuevas |

### Detalle por archivo

#### `docker-compose.yml`

```yaml
services:
  postgres:
    image: postgres:16-alpine
    environment:
      POSTGRES_USER: flagstone
      POSTGRES_PASSWORD: flagstone_dev
      POSTGRES_DB: flagstone
    ports: ["5432:5432"]
    volumes:
      - pgdata:/var/lib/postgresql/data
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U flagstone"]
      interval: 5s
      timeout: 3s
      retries: 5

  redis:
    image: redis:7-alpine
    ports: ["6379:6379"]
    volumes:
      - redisdata:/data
    healthcheck:
      test: ["CMD", "redis-cli", "ping"]
      interval: 5s
      timeout: 3s
      retries: 5

volumes:
  pgdata:
  redisdata:
```

**Justificacion**:
- Postgres 16 Alpine: matchea el schema (usa `gen_random_uuid()` built-in, CITEXT extension).
- Redis 7 Alpine: version estable mas reciente, soporta pub/sub, streams, capped lists.
- Healthchecks: `make setup` espera a que ambos esten ready antes de retornar.
- Volumes nombrados: persistencia entre restarts de contenedores, pero `make clean` los borra (fresh DB).
- Credenciales matchean `.env.example`: `postgres://flagstone:flagstone_dev@localhost:5432/flagstone?sslmode=disable`.

#### `go.mod` — nuevas dependencias

```
require (
    go.uber.org/zap v1.27.0
    github.com/jackc/pgx/v5 v5.x.x
    github.com/redis/go-redis/v9 v9.x.x
    github.com/golang-jwt/jwt/v5 v5.x.x
    golang.org/x/crypto v0.x.x
    golang.org/x/sync v0.x.x
    github.com/stretchr/testify v1.x.x
)
```

#### `cmd/flagstone/main.go` — cambios

**Que cambia**:
1. Importar `"github.com/flagstonehq/flagstone/internal/config"`
2. Eliminar funcion `envOr` duplicada (ya existe en `config`)
3. Llamar `config.Load()` al inicio — si falla, `logger.Fatal`
4. Inicializar pool de Postgres con retry (5 intentos, backoff exponencial, como DESIGN.md)
5. Inicializar Redis client
6. Agregar `/readyz` endpoint
7. Trackear `startTime` para calcular `uptime_seconds` en `/healthz`

**`/readyz` implementacion**:
```
GET /readyz

200 OK:
{
  "status": "ready",
  "checks": {
    "postgres": { "status": "up", "latency_ms": 2 },
    "redis": { "status": "up", "latency_ms": 1 }
  }
}

503 Service Unavailable (si Postgres down):
{
  "status": "not_ready",
  "checks": {
    "postgres": { "status": "down", "error": "connection refused" },
    "redis": { "status": "up", "latency_ms": 1 }
  }
}
```

**Logica de readiness** (de DESIGN.md):
- Postgres UP -> 200 (Redis es nice-to-have, no bloquea)
- Postgres DOWN -> 503 (no se pueden evaluar flags que no estan en cache)
- Redis DOWN -> 200 con warning en el check (el servidor funciona pero con latencia mayor)

#### Startup retry para Postgres

```go
func connectWithRetry(ctx context.Context, url string, logger *zap.Logger) (*pgxpool.Pool, error) {
    backoff := time.Second
    for i := range 5 {
        pool, err := pgxpool.New(ctx, url)
        if err == nil {
            if err := pool.Ping(ctx); err == nil {
                return pool, nil
            }
        }
        logger.Warn("db not ready, retrying", zap.Int("attempt", i+1), zap.Duration("backoff", backoff))
        time.Sleep(backoff)
        backoff *= 2
    }
    return nil, fmt.Errorf("storage: failed to connect after 5 attempts")
}
```

**Justificacion**: En Docker, Postgres tarda unos segundos en estar listo. Sin retry, `make setup && make migrate && make run` fallaria intermitentemente.

### Checklist Fase 0

- [x] Crear `docker-compose.yml` con Postgres 16 + Redis 7, healthchecks, volumes
- [x] Agregar dependencias a `go.mod` (pgx, redis, jwt, x/crypto, x/sync, testify)
- [x] Ejecutar `go mod tidy` para generar `go.sum`
- [x] Integrar `config.Load()` en `main.go` (eliminar `envOr` duplicado)
- [x] Implementar `connectWithRetry` para Postgres (5 intentos, backoff exponencial)
- [x] Inicializar pool de Postgres con config (25 MaxConns, 5 MinConns, etc.)
- [x] Inicializar Redis client
- [x] Implementar `/readyz` endpoint (SELECT 1 a Postgres, PING a Redis)
- [x] Agregar `uptime_seconds` a `/healthz` (trackear `startTime`)
- [x] Verificar: `make setup` levanta ambos servicios
- [x] Verificar: `make migrate` aplica ambas migraciones sin error
- [x] Verificar: `make run` inicia el servidor
- [x] Verificar: `curl /healthz` retorna `{"status":"ok","version":"dev","uptime_seconds":N}`
- [x] Verificar: `curl /readyz` retorna checks de postgres y redis "up"

---

## 5. Fase 1 — Storage Layer

**Objetivo:** Acceso a datos completo para todas las tablas. Cada query incluye `tenant_id` para aislamiento cross-tenant.

### Estructura de archivos

```
internal/storage/
├── postgres.go          # NewPool() con retry, config del pool
├── models.go            # Structs del dominio (Tenant, User, Flag, etc.)
├── errors.go            # Sentinel errors
├── types.go             # Value types (FlagKey, EnvironmentID, TenantID)
├── tenant_store.go      # Create, GetBySlug, GetByID, ExistsAny
├── user_store.go        # Create, GetByEmail, GetByID, UpdateLastLogin, VerifyEmail
├── session_store.go     # Create, GetByRefreshHash, DeleteByID, DeleteByUserID, DeleteExpired
├── member_store.go      # Add, GetRole, ListByTenant, UpdateRole, Remove
├── project_store.go     # Create, List, GetBySlug, Update
├── environment_store.go # Create, List, GetBySlug, GetByID
├── apikey_store.go      # Create, GetByHash, List, Revoke, UpdateLastUsed
├── flag_store.go        # Create, GetByKey, List, Update, Archive
├── flag_env_store.go    # Upsert, Get, UpdateWithVersion (OCC), ListByEnvironment (bulk JOIN)
├── segment_store.go     # Create, GetByKey, List, Update, Archive, ListByProject
├── audit_store.go       # Insert, Query (filtros + pagination)
├── postgres_test.go     # Test de conexion pool
├── tenant_store_test.go
├── user_store_test.go
├── session_store_test.go
├── member_store_test.go
├── project_store_test.go
├── environment_store_test.go
├── apikey_store_test.go
├── flag_store_test.go
├── flag_env_store_test.go
├── segment_store_test.go
└── audit_store_test.go
```

### Detalle por archivo

#### `postgres.go`

```go
func NewPool(ctx context.Context, cfg *config.Config) (*pgxpool.Pool, error)
```

Config del pool (de DESIGN.md):
- `MaxConns: 25` (calculado para db.t3.micro con headroom)
- `MinConns: 5` (pool tibio, evita cold-connect en startup)
- `MaxConnLifetime: 1h` (reciclar para detectar failovers de RDS)
- `MaxConnIdleTime: 5m` (liberar conexiones idle)
- `HealthCheckPeriod: 30s` (detectar conexiones muertas)

**Tests**: Verificar que el pool se crea, que `Ping` funciona, que las config se aplican.

#### `models.go`

Structs que mapean 1:1 con las tablas de la migracion:

```go
type Tenant struct {
    ID        uuid.UUID
    Slug      string
    Name      string
    Plan      string  // "free", "team", "enterprise"
    CreatedAt time.Time
    UpdatedAt time.Time
}

type User struct {
    ID              uuid.UUID
    Email           string
    PasswordHash    *string  // nullable para SSO/OAuth
    EmailVerifiedAt *time.Time
    LastLoginAt     *time.Time
    CreatedAt       time.Time
    UpdatedAt       time.Time
}

type TenantMember struct {
    TenantID  uuid.UUID
    UserID    uuid.UUID
    Role      string  // "owner", "admin", "member", "viewer"
    CreatedAt time.Time
}

type Session struct {
    ID          uuid.UUID
    UserID      uuid.UUID
    TenantID    uuid.UUID
    RefreshHash string
    UserAgent   *string
    IPAddress   net.IP
    ExpiresAt   time.Time
    CreatedAt   time.Time
}

type Project struct {
    ID        uuid.UUID
    TenantID  uuid.UUID
    Slug      string
    Name      string
    CreatedAt time.Time
    UpdatedAt time.Time
}

type Environment struct {
    ID        uuid.UUID
    ProjectID uuid.UUID
    Slug      string
    Name      string
    CreatedAt time.Time
    UpdatedAt time.Time
}

type APIKey struct {
    ID            uuid.UUID
    EnvironmentID uuid.UUID
    Name          string
    KeyHash       string
    KeyPrefix     string
    LastUsedAt    *time.Time
    ExpiresAt     *time.Time
    RevokedAt     *time.Time
    CreatedAt     time.Time
    CreatedBy     *uuid.UUID
}

type Flag struct {
    ID           uuid.UUID
    ProjectID    uuid.UUID
    Key          string
    Name         string
    Description  *string
    Type         string  // "boolean", "string", "number", "json"
    DefaultValue json.RawMessage
    ArchivedAt   *time.Time
    CreatedAt    time.Time
    UpdatedAt    time.Time
    CreatedBy    *uuid.UUID
}

type FlagEnvironment struct {
    FlagID        uuid.UUID
    EnvironmentID uuid.UUID
    Enabled       bool
    Rules         json.RawMessage  // JSONB
    DefaultValue  *json.RawMessage // per-env override
    Version       int64
    CreatedAt     time.Time
    UpdatedAt     time.Time
    UpdatedBy     *uuid.UUID
}

type Segment struct {
    ID          uuid.UUID
    ProjectID   uuid.UUID
    Key         string
    Name        string
    Description *string
    Rules       json.RawMessage  // JSONB
    ArchivedAt  *time.Time
    CreatedAt   time.Time
    UpdatedAt   time.Time
    CreatedBy   *uuid.UUID
}

type AuditLogEntry struct {
    ID           uuid.UUID
    TenantID     uuid.UUID
    ActorID      *uuid.UUID
    ActorType    string  // "user", "api_key", "system"
    Action       string  // "flag.created", "auth.login", etc.
    ResourceType string  // "flag", "segment", "environment"
    ResourceID   *uuid.UUID
    Changes      *json.RawMessage
    IPAddress    net.IP
    UserAgent    *string
    CreatedAt    time.Time
}
```

#### `errors.go`

```go
var (
    ErrNotFound           = errors.New("not found")
    ErrFlagNotFound       = errors.New("flag not found")
    ErrFlagArchived       = errors.New("flag archived")
    ErrEnvironmentNotFound = errors.New("environment not found")
    ErrProjectNotFound    = errors.New("project not found")
    ErrVersionConflict    = errors.New("version conflict")       // OCC
    ErrDuplicateKey       = errors.New("duplicate key")          // unique constraint
    ErrAlreadyInitialized = errors.New("already initialized")    // bootstrap TOCTOU
)
```

#### `types.go` — Value types para IDs criticos

```go
type FlagKey       string
type EnvironmentID string
type TenantID      string
type ProjectID     string
type SegmentKey    string
```

**Justificacion** (de DESIGN.md): Previene el bug donde `flagKey` y `environmentID` se intercambian en una llamada a funcion. El compilador lo atrapa si son tipos distintos.

#### `flag_env_store.go` — Bulk query (hot path)

La query mas importante del sistema (SDK bootstrap + bulk evaluation):

```sql
SELECT f.id, f.key, f.type, f.default_value AS flag_default,
       fe.enabled, fe.rules, fe.default_value AS env_default, fe.version
FROM flags f
JOIN flag_environments fe ON fe.flag_id = f.id
WHERE fe.environment_id = $1
  AND f.archived_at IS NULL
```

Una sola query, sin importar cuantos flags haya. El resultado se evalua enteramente en memoria.

#### `flag_env_store.go` — OCC (optimistic concurrency control)

```sql
UPDATE flag_environments
SET enabled = $3, rules = $4, default_value = $5, updated_by = $6
WHERE flag_id = $1 AND environment_id = $2 AND version = $7
```

Si `rows_affected == 0` -> retornar `ErrVersionConflict`. El trigger de la DB auto-incrementa `version`, asi que el application code no necesita hacerlo manualmente.

#### `apikey_store.go` — UpdateLastUsed async

`UpdateLastUsed` se ejecuta de forma async (goroutine separada, non-blocking) para no agregar latencia a cada request de evaluacion. Si falla, no importa — es solo metrica.

### Tests Fase 1

**Estrategia**: Integration tests contra Postgres real via docker-compose.

Cada store tiene su archivo `*_test.go` con tests que:
1. Crean la tabla (las migraciones ya corrieron via `make setup && make migrate`)
2. Insertan datos de prueba
3. Verifican que las queries retornan lo esperado
4. Verifican edge cases (not found, duplicate, version conflict)

**Tests criticos por store**:

| Store | Tests clave |
|---|---|
| `tenant_store` | Create + GetBySlug round-trip, ExistsAny (true/false), slug uniqueness |
| `user_store` | Create + GetByEmail round-trip, email case-insensitive (CITEXT), UpdateLastLogin |
| `session_store` | Create + GetByRefreshHash round-trip, DeleteExpired, expired session no se encuentra |
| `member_store` | Add + GetRole round-trip, role update, remove member |
| `project_store` | Create + List (scoped a tenant), slug uniqueness por tenant |
| `environment_store` | Create + GetBySlug (scoped a project), slug uniqueness por project |
| `apikey_store` | Create + GetByHash round-trip, Revoke (revoked_at set), active partial index funciona |
| `flag_store` | Create + GetByKey round-trip, Archive (archived_at set), archived flag no aparece en List |
| `flag_env_store` | Upsert (create si no existe, update si existe), UpdateWithVersion (OCC conflict), ListByEnvironment (bulk JOIN) |
| `segment_store` | Create + GetByKey round-trip, Archive, ListByProject |
| `audit_store` | Insert + Query (filtros por tenant, actor, resource, time range), pagination |

### Checklist Fase 1

- [x] Crear `internal/storage/postgres.go` con `NewPool()` (config pool, retry)
- [x] Crear `internal/storage/models.go` con todos los structs del dominio
- [x] Crear `internal/storage/errors.go` con sentinel errors
- [x] Crear `internal/storage/types.go` con value types para IDs
- [x] Crear `internal/storage/tenant_store.go` (4 metodos)
- [x] Crear `internal/storage/user_store.go` (5 metodos)
- [x] Crear `internal/storage/session_store.go` (5 metodos)
- [x] Crear `internal/storage/member_store.go` (5 metodos)
- [x] Crear `internal/storage/project_store.go` (4 metodos)
- [x] Crear `internal/storage/environment_store.go` (4 metodos)
- [x] Crear `internal/storage/apikey_store.go` (5 metodos)
- [x] Crear `internal/storage/flag_store.go` (5 metodos)
- [x] Crear `internal/storage/flag_env_store.go` (4 metodos, bulk JOIN, OCC)
- [x] Crear `internal/storage/segment_store.go` (6 metodos)
- [x] Crear `internal/storage/audit_store.go` (2 metodos: Insert, Query)
- [x] Tests: `postgres_test.go` — pool creation, ping
- [x] Tests: `tenant_store_test.go` — CRUD, ExistsAny
- [x] Tests: `user_store_test.go` — CRUD, CITEXT case-insensitive
- [x] Tests: `session_store_test.go` — CRUD, DeleteExpired
- [x] Tests: `member_store_test.go` — Add, GetRole, UpdateRole, Remove
- [x] Tests: `project_store_test.go` — CRUD, tenant-scoped
- [x] Tests: `environment_store_test.go` — CRUD, project-scoped
- [x] Tests: `apikey_store_test.go` — CRUD, Revoke, active index
- [x] Tests: `flag_store_test.go` — CRUD, Archive
- [x] Tests: `flag_env_store_test.go` — Upsert, OCC conflict, bulk JOIN
- [x] Tests: `segment_store_test.go` — CRUD, Archive
- [x] Tests: `audit_store_test.go` — Insert, Query con filtros

---

## 6. Fase 2 — Auth Core

**Objetivo:** Funciones puras de autenticacion. Sin HTTP, sin DB. Altamente testeables.

### Estructura de archivos

```
internal/auth/
├── jwt.go        # GenerateAccessToken, ValidateAccessToken
├── claims.go     # Claims struct + helpers
├── password.go   # HashPassword, VerifyPassword (bcrypt)
├── apikey.go     # GenerateAPIKey (crypto/rand, base62, SHA-256)
├── token.go      # GenerateRefreshToken (crypto/rand, base62, SHA-256)
├── roles.go      # Role type, constants, Level(), AtLeast()
├── base62.go     # Encode/Decode base62
├── jwt_test.go
├── password_test.go
├── apikey_test.go
├── token_test.go
├── roles_test.go
└── base62_test.go
```

### Detalle por archivo

#### `jwt.go`

```go
func GenerateAccessToken(userID, tenantID uuid.UUID, role string, secret string, ttl time.Duration) (string, error)
func ValidateAccessToken(tokenStr, secret string) (*Claims, error)
```

- Algoritmo: HS256 (HMAC-SHA256)
- Claims (de SECURITY.md):
  - `sub` (subject): user ID
  - `tid` (tenant): tenant ID
  - `role`: role within tenant
  - `exp` (expiration): 15 min TTL default
  - `iss` (issuer): "flagstone" (prevents token confusion)

**Justificacion HS256** (de SECURITY.md): Single-issuer system — symmetric signing es mas simple que RSA/ECDSA. La signing key nunca sale del servidor.

#### `claims.go`

```go
type Claims struct {
    UserID   uuid.UUID `json:"sub"`
    TenantID uuid.UUID `json:"tid"`
    Role     string    `json:"role"`
    Issuer   string    `json:"iss"`
    jwt.RegisteredClaims
}
```

#### `password.go`

```go
func HashPassword(plain string, cost int) (string, error)
func VerifyPassword(hash, plain string) error
```

- Algoritmo: bcrypt, cost=12 por defecto (~250ms por operacion)
- **Justificacion bcrypt vs argon2id** (de SECURITY.md): bcrypt es battle-tested, disponible en `golang.org/x/crypto`, la diferencia de seguridad es marginal para nuestro threat model. Migrar a argon2id despues es trivial.

#### `apikey.go`

```go
func GenerateAPIKey(envHint string, randomBytes int) (rawKey, keyHash, keyPrefix string, error)
```

Formato: `fs_{envHint}_{32 bytes crypto/rand en base62}`

Ejemplo: `fs_live_a3b9d2c8e4f1g5h7j8k9m2n4p6q8r1s3t5`

- `key_hash`: SHA-256 del key completo
- `key_prefix`: primeros 12 chars (`fs_live_a3b9`)
- **Justificacion SHA-256** (de SECURITY.md): API keys son high-entropy (32 bytes random). No son passwords humanos. SHA-256 es rapido (intencionalmente — se hashea en cada request). Brute-force es computacionalmente inviable.

#### `token.go`

```go
func GenerateRefreshToken(randomBytes int) (rawToken, tokenHash string, error)
```

- 32 bytes de `crypto/rand`, encodeado en base62
- Se almacena `SHA-256(rawToken)` en la tabla `sessions`

#### `roles.go`

```go
type Role string

const (
    RoleOwner  Role = "owner"
    RoleAdmin  Role = "admin"
    RoleMember Role = "member"
    RoleViewer Role = "viewer"
)

func (r Role) Level() int {
    switch r {
    case RoleOwner:  return 4
    case RoleAdmin:  return 3
    case RoleMember: return 2
    case RoleViewer: return 1
    default:         return 0
    }
}

func (r Role) AtLeast(minimum Role) bool {
    return r.Level() >= minimum.Level()
}
```

**Justificacion**: Los niveles numericos permiten comparacion simple en middleware RBAC. `RoleMember.AtLeast(RoleViewer)` -> true. `RoleViewer.AtLeast(RoleAdmin)` -> false.

#### `base62.go`

Encoder base62: alfabeto `0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz` (62 chars).

**Justificacion**: Base62 produce tokens legibles (solo alphanumeric), sin caracteres especiales que podrian causar problemas en URLs o logs. Es mas seguro que base64 standard (que incluye `+`, `/`, `=`).

### Tests Fase 2

Todos son unit tests puros — no necesitan DB ni HTTP.

| Test | Que verifica |
|---|---|
| `jwt_test.go` | Generate -> Validate round-trip, expired token rejected, wrong secret rejected, claims correctos |
| `password_test.go` | Hash -> Verify round-trip, wrong password rejected, cost configurable, hash no es plaintext |
| `apikey_test.go` | Formato `fs_(live|test)_...`, hash es deterministico, prefix longitud correcta, keys diferentes cada vez |
| `token_test.go` | Generate -> hash round-trip, tokens diferentes cada vez |
| `roles_test.go` | Owner >= Admin >= Member >= Viewer, nivel 0 para rol desconocido |
| `base62_test.go` | Encode -> Decode round-trip, empty input, single byte, 32 bytes |

### Checklist Fase 2

- [x] Crear `internal/auth/jwt.go` (GenerateAccessToken, ValidateAccessToken, HS256)
- [x] Crear `internal/auth/claims.go` (Claims struct con sub, tid, role, exp, iss)
- [x] Crear `internal/auth/password.go` (HashPassword, VerifyPassword, bcrypt cost=12)
- [x] Crear `internal/auth/apikey.go` (GenerateAPIKey, crypto/rand, base62, SHA-256)
- [x] Crear `internal/auth/token.go` (GenerateRefreshToken, crypto/rand, base62, SHA-256)
- [x] Crear `internal/auth/roles.go` (Role type, Level, AtLeast)
- [x] Crear `internal/auth/base62.go` (Encode base62)
- [x] Tests: `jwt_test.go` — round-trip, expired, wrong secret, claims
- [x] Tests: `password_test.go` — round-trip, wrong password, cost
- [x] Tests: `apikey_test.go` — formato, hash deterministico, prefix, unicidad
- [x] Tests: `token_test.go` — round-trip, unicidad
- [x] Tests: `roles_test.go` — hierarchy, AtLeast, unknown role
- [x] Tests: `base62_test.go` — round-trip, edge cases

---

## 7. Fase 3 — Middleware + Router + Bootstrap

**Objetivo:** Framework HTTP completo y `POST /api/v1/setup` funcional.

### Estructura de archivos

```
internal/api/
├── server.go               # Server struct, NewServer(), Routes() http.Handler
├── response.go             # JSON(), Error(), ErrorFromDomain()
├── request.go              # DecodeJSON() con body limit + content-type check
├── context.go              # ClaimsFromContext, EnvironmentFromContext, etc.
├── middleware/
│   ├── recover.go          # Panic recovery
│   ├── request_id.go       # Request ID generation
│   ├── logger.go           # Structured logging con request_id
│   ├── body_limit.go       # http.MaxBytesReader 1MB
│   ├── content_type.go     # Content-Type: application/json enforcement
│   ├── auth_jwt.go         # JWT auth middleware
│   ├── auth_apikey.go      # API key auth middleware
│   └── rbac.go             # RequireRole middleware
└── handlers/
    └── setup.go            # POST /api/v1/setup
```

### Detalle por archivo

#### `server.go`

```go
type Server struct {
    stores  *storage.Stores
    engine  engine.Engine  // nil en esta fase, se setea en Fase 8
    config  *config.Config
    logger  *zap.Logger
}

func NewServer(stores *storage.Stores, cfg *config.Config, logger *zap.Logger) *Server
func (s *Server) Routes() http.Handler
```

**Routes registradas en esta fase**:

| Metodo | Path | Handler | Auth | RBAC |
|---|---|---|---|---|
| GET | `/healthz` | (ya existe en main.go) | None | - |
| GET | `/readyz` | (ya existe en main.go) | None | - |
| POST | `/api/v1/setup` | `setup.go` | None | - |

**Middleware chain** (aplicado a todos los endpoints):
```
RecoverPanic -> RequestID -> Logger -> [Auth] -> [RBAC] -> Handler
```

#### `response.go`

Formato de error consistente (de DESIGN.md):
```json
{
  "error": {
    "code": "FLAG_NOT_FOUND",
    "message": "Flag 'checkout-v2' does not exist in this project",
    "request_id": "req_abc123"
  }
}
```

Helpers:
- `JSON(w http.ResponseWriter, status int, data any)` — setea Content-Type, encodea JSON
- `Error(w http.ResponseWriter, status int, code, message string)` — error helper
- `ErrorFromDomain(w http.ResponseWriter, err error)` — mapea domain errors a HTTP status

#### `request.go`

- `DecodeJSON(r *http.Request, target any) error` — envuelve `r.Body` con `http.MaxBytesReader(1MB)`, verifica Content-Type, decodea JSON
- Retorna errores descriptivos: "request body too large", "unsupported media type", "invalid JSON"

#### `context.go`

Helpers para extraer valores del context (inyectados por middleware):

```go
func ClaimsFromContext(ctx context.Context) *auth.Claims
func EnvironmentIDFromContext(ctx context.Context) string
func TenantIDFromContext(ctx context.Context) string
func RequestIDFromContext(ctx context.Context) string
func LoggerFromContext(ctx context.Context) *zap.Logger
```

#### Middleware details

| Middleware | Que hace |
|---|---|
| `RecoverPanic` | `defer func() { if r := recover(); r != nil { log stack, return 500 } }()` |
| `RequestID` | Genera `req_` + 8 chars random, inyecta en context + header `X-Request-ID` |
| `Logger` | Crea logger scoped con request_id, loguea metodo, path, status, duracion |
| `BodyLimit` | `http.MaxBytesReader(w, r.Body, 1<<20)` (1MB) |
| `ContentType` | Rechaza POST/PUT/PATCH sin `Content-Type: application/json` -> 415 |
| `AuthJWT` | Extrae `Bearer <token>`, valida JWT, inyecta Claims en context |
| `AuthAPIKey` | Extrae `Bearer <key>`, SHA-256, lookup en DB, inyecta environment_id en context |
| `RBAC` | `RequireRole(minimum)` — verifica `claims.Role.AtLeast(minimum)` -> 403 si no |

#### `handlers/setup.go` — `POST /api/v1/setup`

**Spec** (de DESIGN.md):

```
POST /api/v1/setup
(no auth — solo funciona cuando no existen tenants)

Body:
{
  "tenant_name": "Acme Corp",
  "admin_email": "admin@acme.com",
  "admin_password": "secure-password-here"
}

Response 201 Created:
{
  "tenant_id": "...",
  "user_id": "...",
  "access_token": "eyJhbGci..."
}

Set-Cookie: refresh_token=<opaque>; HttpOnly; Secure; SameSite=Strict; Path=/api/v1/auth

Response 409 Conflict (si ya existe un tenant):
{
  "error": { "code": "ALREADY_INITIALIZED", "message": "This instance is already set up." }
}
```

**Implementacion** (atomico, una sola transaccion):

1. Verificar `stores.Tenants.ExistsAny()` — si existe algun tenant, retornar 409
2. Validar input: email valido, password >= 8 chars, tenant_name no vacio
3. Hash password con bcrypt
4. Transaccion SQL:
   - `INSERT INTO tenants (slug, name, plan)` — slug generado desde tenant_name
   - `INSERT INTO users (email, password_hash, email_verified_at=NOW())` — bootstrap user se considera verificado
   - `INSERT INTO tenant_members (tenant_id, user_id, role='owner')`
   - `INSERT INTO sessions (...)` — refresh token
5. Generar JWT access token (15 min)
6. Insertar audit log: `system.bootstrap`
7. Retornar 201 con tenant_id, user_id, access_token + Set-Cookie

**TOCTOU protection** (de SECURITY.md T18):
- `INSERT INTO tenants ... WHERE NOT EXISTS (SELECT 1 FROM tenants)` en una sola sentencia atomica
- Si `rows_affected == 0`, retornar 409
- El unique constraint en `tenants.slug` es un segundo guard

**Justificacion email_verified_at=NOW()**: El bootstrap user es creado por el operador del servidor (no por signup publico). No necesita verificacion por email.

### Tests Fase 3

| Test | Que verifica |
|---|---|
| `middleware/recover_test.go` | Panic se recupera, retorna 500, loguea stack trace |
| `middleware/request_id_test.go` | Cada request tiene request_id unico, header X-Request-ID presente |
| `middleware/logger_test.go` | Log incluye request_id, metodo, path, status, duracion |
| `middleware/body_limit_test.go` | Body > 1MB retorna 413 |
| `middleware/content_type_test.go` | POST sin Content-Type application/json retorna 415 |
| `middleware/auth_jwt_test.go` | JWT valido pasa, JWT invalido/expirado retorna 401 |
| `middleware/auth_apikey_test.go` | API key valido pasa, key invalido/revocado retorna 401 |
| `middleware/rbac_test.go` | Owner accede a todo, viewer bloqueado en write |
| `handlers/setup_test.go` | Bootstrap exitoso (201), segundo bootstrap falla (409), input validation |

### Checklist Fase 3

- [x] Crear `internal/api/server.go` (Server struct, Routes, middleware chain)
- [x] Crear `internal/api/middleware/response.go` (JSON, Error, ErrorFromDomain)
- [x] Crear `internal/api/request.go` (DecodeJSON con body limit + content-type)
- [x] Crear `internal/api/middleware/context.go` (helpers para extraer del context)
- [x] Crear `internal/api/middleware/recover.go`
- [x] Crear `internal/api/middleware/request_id.go`
- [x] Crear `internal/api/middleware/logger.go`
- [x] Crear `internal/api/middleware/body_limit.go`
- [x] Crear `internal/api/middleware/content_type.go`
- [x] Crear `internal/api/middleware/auth_jwt.go`
- [x] Crear `internal/api/middleware/auth_apikey.go` (incluye `expires_at` check y `last_used_at` async)
- [x] Crear `internal/api/middleware/rbac.go` (RequireRole)
- [x] Crear `internal/api/setup.go` (POST /api/v1/setup)
- [x] Tests: `recover_test.go`
- [x] Tests: `request_id_test.go`
- [x] Tests: `logger_test.go`
- [x] Tests: `body_limit_test.go`
- [x] Tests: `content_type_test.go`
- [x] Tests: `auth_jwt_test.go`
- [x] Tests: `auth_apikey_test.go` (missing/unknown/valid/expired/revoked)
- [x] Tests: `rbac_test.go`
- [x] Tests: `setup_test.go` (success, 409, validacion, wrong content-type, GET, body too large, password >72)

---

## 8. Fase 4 — Auth Endpoints

**Objetivo:** Login, refresh, logout funcionales con token rotation, soporte multi-tenant, account lockout (T19) y refresh token reuse detection (T20).

### Cambios vs. spec original

Esta fase incorpora 3 features adicionales que en el plan inicial no estaban:

1. **Multi-tenant login**: `users.email` es global, `tenant_members` es many-to-many. Un usuario puede pertenecer a varios tenants — el login tiene que resolver a cuál.
2. **Account lockout (T19)**: rate limit por IP no alcanza si el atacante rota IPs. Lockout per-user es la defensa complementaria.
3. **Refresh token reuse detection (T20)**: si un refresh ya rotado se presenta de nuevo, es señal de compromiso → matar todas las sessions del user.

Las 3 están reflejadas en `SECURITY.md` (sección Authentication + threat matrix T19/T20).

### Migración nueva

```
migrations/000003_auth_phase4.up.sql
```

Tablas a agregar:

```sql
CREATE TABLE login_attempts (
    id         BIGSERIAL    PRIMARY KEY,
    user_id    UUID         NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    ip_address INET,
    failed_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX login_attempts_user_recent_idx
    ON login_attempts(user_id, failed_at DESC);

-- Retencion: cleanup periodico de filas con failed_at < NOW() - INTERVAL '1 day'

CREATE TABLE revoked_refresh_tokens (
    refresh_hash CHAR(64)    PRIMARY KEY,
    user_id      UUID        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    revoked_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at   TIMESTAMPTZ NOT NULL
);

CREATE INDEX revoked_refresh_tokens_expires_idx
    ON revoked_refresh_tokens(expires_at);

-- Retencion: cleanup periodico de filas con expires_at < NOW()
```

**Por qué `BIGSERIAL` en `login_attempts`**: alto volumen de inserts esperado, no necesita UUID (no es expuesto al cliente).

**Por qué `CHAR(64)` PK en `revoked_refresh_tokens`**: el hash es la clave de lookup natural. Sin id sobrante.

### Estructura de archivos

```
internal/api/
├── auth.go            # login, refresh, logout, mfa handlers
└── auth_test.go       # integration tests

internal/storage/
├── login_attempt_store.go        # Record, CountSince, ClearForUser
├── login_attempt_store_test.go
├── revoked_token_store.go        # Insert, Lookup, CleanupExpired
├── revoked_token_store_test.go
```

### Detalle por endpoint

#### `POST /api/v1/auth/login` (multi-tenant híbrido)

```
Body:
{
  "email": "ana@acme.com",
  "password": "...",
  "tenant_slug": "acme"   // OPCIONAL
}

Server (orden de checks):
  1. Validar formato email + largo password (8-72 chars)
  2. Lookup user by email (CITEXT)
     - Si no existe: bcrypt-time fake hash para evitar timing oracle, 401 generico
  3. Check account lockout (T19):
     - SELECT COUNT(*) FROM login_attempts
       WHERE user_id = $1 AND failed_at > NOW() - INTERVAL '15 minutes'
     - Si count >= 5: 423 Locked con Retry-After header
  4. bcrypt.CompareHashAndPassword
     - Si falla: INSERT INTO login_attempts, INSERT audit (auth.login_failed), 401 generico
  5. Resolver tenant context:
     a. Si tenant_slug provisto:
        - JOIN tenant_members + tenants verificar membership
        - Si no es miembro / slug no existe: 401 INVALID_CREDENTIALS (ver nota mas abajo)
     b. Si tenant_slug ausente:
        - SELECT memberships del user
        - 0 memberships: 401 INVALID_CREDENTIALS (ver nota mas abajo)
        - 1 membership: usar ese tenant
        - 2+ memberships: 409 MULTIPLE_TENANTS con lista en response
  6. Generar JWT access (15 min) con sub=user_id, tid=tenant_id, role
  7. Generar refresh token (32 bytes crypto/rand)
  8. Transaccion:
     - INSERT INTO sessions (refresh_hash = SHA-256(token))
     - UPDATE users SET last_login_at = NOW()
     - DELETE FROM login_attempts WHERE user_id = $1  (clear on success)
     - INSERT INTO audit_log (auth.login)
  9. Set-Cookie + return access_token

Response 200 (single tenant o tenant_slug resuelto):
{
  "access_token": "eyJhbGci...",
  "token_type": "Bearer",
  "expires_in": 900,
  "tenant": { "id": "...", "slug": "acme", "role": "owner" }
}

Response 409 MULTIPLE_TENANTS:
{
  "error": {
    "code": "MULTIPLE_TENANTS",
    "message": "Specify tenant_slug to continue.",
    "available_tenants": [
      { "slug": "acme", "name": "Acme Corp", "role": "owner" },
      { "slug": "beta-labs", "name": "Beta Labs", "role": "viewer" }
    ]
  }
}

Response 423 Locked:
{
  "error": {
    "code": "ACCOUNT_LOCKED",
    "message": "Account locked due to repeated failed logins. Try again in 15 minutes.",
    "retry_after": 900
  }
}
+ Header: Retry-After: 900
```

**Notas críticas**:

- El paso 2 hashea siempre, incluso si el user no existe, para mantener tiempo de respuesta constante (timing oracle defense).
- El paso 3 ocurre ANTES del bcrypt para no quemar CPU en cuentas lockeadas (DoS amplification).
- El paso 5b nunca retorna 200 con tenant arbitrario — eso violaría el principio de "el rol no es global".
- Los mensajes 401 son idénticos para "user no existe", "password mal", "user no verificado" → previene enumeration.

#### `POST /api/v1/auth/refresh` (con reuse detection)

```
Cookie: refresh_token=<opaque>

Server:
  1. Extraer refresh token del cookie
  2. Computar hash = SHA-256(token)
  3. Lookup en sessions:
     a. Si encontrado y no expirado: flow normal
     b. Si NO encontrado: lookup en revoked_refresh_tokens
        - Si encontrado ALLI: REUSE DETECTADO (T20)
          - DELETE FROM sessions WHERE user_id = $victim
          - INSERT audit (auth.refresh_reuse_detected)
          - Send alert email (async)
          - Clear cookie, return 401
        - Si tampoco: token desconocido, return 401
  4. Flow normal (token encontrado en sessions):
     - Generar nuevo refresh token + JWT
     - Transaccion:
       * INSERT old hash INTO revoked_refresh_tokens (expires_at = old expires_at)
       * DELETE FROM sessions WHERE refresh_hash = $old_hash
       * INSERT INTO sessions (new refresh_hash)
       * INSERT audit (auth.refresh)
     - Set-Cookie nuevo + return access_token
```

**El truco**: la tabla `revoked_refresh_tokens` retiene hashes rotados durante `REFRESH_TOKEN_TTL` (7 días por defecto). Una vez expirado, el cleanup periódico lo borra. Si un atacante presenta un token rotado, lo agarrás.

**Si el atacante usa el token antes que el legítimo**: el atacante consigue una rotation exitosa, el legítimo presenta el ahora-rotado token y dispara el reuse detection → ambos quedan fuera. El legítimo recibe el email de alerta.

#### `POST /api/v1/auth/logout`

```
Authorization: Bearer <access_token>
Cookie: refresh_token=<opaque>

Server:
  1. Computar hash = SHA-256(refresh_token)
  2. Transaccion:
     - SELECT refresh_hash, expires_at FROM sessions WHERE refresh_hash = $hash
     - INSERT INTO revoked_refresh_tokens (...)  -- previene reuse despues de logout
     - DELETE FROM sessions WHERE refresh_hash = $hash
     - INSERT audit (auth.logout)
  3. Clear refresh_token cookie (MaxAge=-1)
  4. Return 204
```

Mismo principio: agregás el hash a la blocklist al hacer logout, asi si alguien clava ese refresh despues de un logout, lo detectás como reuse.

### Tests Fase 4

| Test | Que verifica |
|---|---|
| Login single-tenant success | 200, JWT con tid correcto, refresh cookie, audit entry |
| Login multi-tenant sin slug | 409 MULTIPLE_TENANTS, lista de tenants en response |
| Login multi-tenant con slug | 200, JWT con tid del slug indicado |
| Login con slug invalido (no miembro) | 401 INVALID_CREDENTIALS (mismo bucket que sin tenants — anti enumeration) |
| Login password incorrecto | 401 generico, INSERT en login_attempts |
| Login email no existe | 401 generico, tiempo de respuesta similar a password mal |
| Login con 5 intentos fallidos | 6to intento → 423 Locked con Retry-After |
| Login exitoso despues de fails parciales | Limpia login_attempts del user |
| Login sin tenants asociados | 401 INVALID_CREDENTIALS (responde igual que password mal — anti enumeration, ver nota al final) |
| Refresh exitoso | Nuevo access + refresh, old hash en revoked_refresh_tokens |
| Refresh con token expirado | 401 |
| Refresh con token desconocido | 401 |
| Refresh con token rotado (REUSE) | 401 + DELETE FROM sessions + audit entry refresh_reuse_detected |
| Refresh attack scenario | Token usado por atacante → legítimo dispara reuse detection → ambos quedan fuera |
| Logout exitoso | Session deleted, hash en revoked_refresh_tokens, cookie cleared |
| Logout luego refresh con mismo token | 401, ya esta en revoked |
| Account locked timing | Lockout check corre ANTES de bcrypt (medible: respuesta < bcrypt-time) |

### Checklist Fase 4

- [x] Migration `000003_auth_security.up.sql` (login_attempts + revoked_refresh_tokens) + down
- [x] Crear `internal/storage/login_attempt_store.go` (Record, CountSince, ClearForUser, DeleteOlderThan)
- [x] Crear `internal/storage/revoked_token_store.go` (Insert, Lookup, DeleteExpired)
- [x] Tests de storage: `login_attempt_store_test.go`, `revoked_token_store_test.go` (cubren Record/CountSince/ClearForUser/DeleteOlderThan + Insert/InsertIdempotent/Lookup/DeleteExpired)
- [x] Refactor stores a interface `Querier` (`internal/storage/querier.go`, `Stores.WithTx`, `Stores.BeginTx`)
- [x] Crear `internal/api/auth.go` con login (multi-tenant), refresh (reuse detection), logout
- [x] Tests: login single-tenant, multi-tenant con/sin slug, slug invalido (`TestLogin_MultiTenant_*`, `TestLogin_TenantSlug_*`)
- [x] Tests: usuario sin tenants (0 memberships) (`TestLogin_NoTenantAccess`) — responde 401 INVALID_CREDENTIALS (ver nota abajo)
- [x] Tests: login password incorrecto, email no existe (timing equivalente via `fakePasswordHash`)
- [x] Tests: account lockout — 5 fails → 423, reset en login exitoso (`TestLogin_LocksAfterFiveFailures`, `TestLogin_SuccessClearsLockoutCounter`)
- [x] Tests: refresh exitoso, expirado, reuse detection (`TestRefresh_ReuseDetected_KillsAllSessions`, `TestRefresh_OldTokenAfterRotation_IsRevoked`)
- [x] Tests: refresh attack scenario completo (`TestRefresh_AttackScenario_StolenTokenBurnsLegitSession` — legítimo rota, atacante replay → todas las sessions mueren incluyendo la nueva del legítimo, audit row escrito)
- [x] Tests: logout exitoso, logout + refresh re-uso (`TestLogout_RevokesRefreshToken`)
- [x] Job de cleanup: `Server.StartCleanup` con ticker de 30 min, timeout por operación (`cleanupOpTimeout`), wired en `cmd/flagstone/main.go`. Retención: `login_attempts` 1 hora, `revoked_refresh_tokens` hasta `expires_at`.
- [x] Actualizar threat matrix en SECURITY.md: T19, T20 → "Mitigated"

### Cambio respecto al spec original — `NO_TENANT_ACCESS`

El plan original (sección "Login multi-tenant híbrido") especificaba que un usuario sin memberships (`len(members) == 0`) recibiera **403 NO_TENANT_ACCESS**. Durante la implementación se descubrió que ese 403 distingue "email registrado sin tenant" de "email no registrado" (que responde 401 INVALID_CREDENTIALS), creando un vector de enumeración de emails.

**Decisión**: colapsar todos los casos de fallo de resolución de tenant (sin memberships, slug no existe, no es miembro del slug) en el mismo **401 INVALID_CREDENTIALS**. El cliente legítimo sin memberships ve la misma respuesta que un password incorrecto; un atacante no puede usar la respuesta para enumerar quién está registrado.

El sentinel `errNoTenantAccess` se mantiene internamente para logging/debugging, pero no se propaga al cliente como código de error distinto.

---

## 8.5. Fase 4.5 — Engine Spike (validacion de modelo)

**Objetivo:** Validar que el modelo de datos actual aguanta la evaluacion de reglas **antes** de comprometerlo en handlers CRUD. Paquete puro Go, sin HTTP ni DB.

### Justificacion

El engine (Fase 7) es la parte mas riesgosa algoritmicamente: operadores con type coercion, segment resolution con cycle detection, rollout hashing, precedence de reglas. Si la forma de `FlagEnvironment.Rules`, `Segment.Rules` o `Flag.DefaultValue` no aguanta esos casos, descubrir eso DESPUES de tener CRUDs entregados implica reescribir handlers y migrar JSONB existente.

Un spike pequeno (3-5 dias) valida la forma del modelo contra los casos reales antes de cementarlo.

### Que NO es esta fase

- NO es la implementacion completa del engine (eso queda en Fase 7).
- NO es el endpoint de evaluate (eso queda en Fase 8).
- NO toca HTTP, DB, ni stores.
- NO requiere panic recovery, structured logging, ni performance tuning.

### Que SI hace

Construye lo minimo del engine para validar:

1. La forma de `FlagEnvironment.Rules` (JSONB) puede expresar AND/OR/NOT anidados.
2. La forma de `Segment.Rules` (JSONB) es consistente con la de flag rules.
3. Los operadores principales funcionan con type coercion sobre valores realistas (string, number, bool, array).
4. Segment refs entre reglas se resuelven con cycle detection.
5. Rollouts deterministicos por hash funcionan (mismo user + mismo flag = mismo resultado).
6. La precedencia "first rule wins" es expresable.

### Estructura de archivos

```
internal/engine/
├── types.go           # FlagConfig, Rule, ConditionNode, RolloutConfig, Segment, Reason
├── engine.go          # Evaluate() — solo el path principal, sin panic recovery aun
├── conditions.go      # evaluateNode() recursivo (all/any/not/leaf)
├── operators.go       # subset minimo: eq, neq, in, gt, contains, segment, exists
├── rollout.go         # inRollout() con FNV-1a
├── segments.go        # resolveSegment() con visited set
├── engine_test.go     # casos representativos de cada path
├── conditions_test.go
├── operators_test.go
├── rollout_test.go
└── segments_test.go
```

**Diferencia con Fase 7**: el spike implementa ~7 operadores (los que cubren los casos representativos), no los 15. No tiene panic recovery, no tiene structured logging integrado, no hace performance tuning. Es codigo "good enough to validate the model", no "good enough to ship".

### Decision al final del spike

Tres salidas posibles:

1. **Modelo OK** → seguir con Fase 5 (CRUD) sin cambios al schema. El codigo del spike se queda en `internal/engine/` y se completa en Fase 7.

2. **Modelo necesita cambios menores** → ajustar `models.go` y crear migracion `000004_engine_model_fixes.up.sql`. Los stores afectados se actualizan ANTES de Fase 5.

3. **Modelo necesita refactor mayor** (improbable) → pausar y discutir. Puede implicar cambiar la forma de Rules JSONB completamente.

### Tests del spike

Cada uno cubre **un riesgo concreto del modelo**, no exhaustividad:

| Test | Que valida del modelo |
|---|---|
| `rule_with_nested_and_or` | `Rules` JSONB puede anidar AND/OR/NOT a 5+ niveles sin perdida de estructura |
| `rule_first_match_wins` | El orden de las reglas en el array JSONB es respetado |
| `rule_segment_ref_works` | Una regla puede referenciar un segmento por key y resolverlo |
| `segment_cycle_detected` | A→B→A no causa stack overflow, retorna false |
| `rollout_deterministic` | Mismo user_id + mismo flag_key → mismo bucket en 1000 corridas |
| `rollout_monotonic` | Users en 10% son subset de users en 25% |
| `operator_type_coercion` | `eq` con "5" vs 5 retorna false (no coerciona implicitamente cross-type) |
| `flag_default_value_resolution` | Sin rules y sin env_default → usa flag.DefaultValue del modelo |
| `flag_env_default_override` | Con env_default presente → ese tiene precedencia sobre flag.DefaultValue |

### Checklist Fase 4.5

- [x] Crear `internal/engine/types.go` (Rule, ConditionNode, RolloutConfig, FlagConfig, Segment, Reason, EvaluateResult)
- [x] Crear `internal/engine/engine.go` (Evaluate — path principal, sin panic recovery)
- [x] Crear `internal/engine/conditions.go` (recursive walker: all/any/not/leaf)
- [x] Crear `internal/engine/operators.go` (subset: eq, neq, in, gt, contains, segment, exists)
- [x] Crear `internal/engine/rollout.go` (FNV-1a, inRollout)
- [x] Crear `internal/engine/segments.go` (resolveSegment con cycle detection)
- [x] Tests representativos (1 por riesgo, ver tabla arriba) — 24 tests, todos PASS
- [x] Decision documentada: modelo OK — el schema actual es compatible, no necesita migracion 000004
- [x] Si aplica: migracion 000004 + ajustes en `models.go` + stores afectados — **N/A** (modelo OK)
- [x] Confirmar con un PR pequeno antes de empezar Fase 5

---

## 9. Fase 5 — CRUD Endpoints

**Objetivo:** Gestion completa de flags, segments, projects, environments, API keys desde la dashboard.

**Prerequisito**: Fase 4.5 (spike) cerrada — el modelo de datos esta validado y no va a cambiar de forma significativa durante esta fase.

### Estructura de archivos

```
internal/api/handlers/
├── projects.go      # POST, GET, GET/:slug, PUT/:slug
├── environments.go  # GET (list por project), POST
├── apikeys.go       # POST, GET, DELETE/:id
├── flags.go         # POST, GET, GET/:key, PUT/:key, DELETE/:key
├── flag_envs.go     # PUT /flags/:key/environments/:env
├── segments.go      # POST, GET, GET/:key, PUT/:key, DELETE/:key
├── projects_test.go
├── environments_test.go
├── apikeys_test.go
├── flags_test.go
├── flag_envs_test.go
└── segments_test.go
```

### Detalle por endpoint

#### Projects

| Metodo | Path | Auth | RBAC | Detalle |
|---|---|---|---|---|
| POST | `/api/v1/projects` | JWT | Admin+ | Crea project + 3 environments auto (development, staging, production) |
| GET | `/api/v1/projects` | JWT | Viewer+ | Lista projects del tenant |
| GET | `/api/v1/projects/:slug` | JWT | Viewer+ | Detalle de un project |
| PUT | `/api/v1/projects/:slug` | JWT | Admin+ | Update name/slug |

**`POST /projects` auto-crea environments** en una sola transaccion:
1. INSERT INTO projects
2. INSERT INTO environments (development)
3. INSERT INTO environments (staging)
4. INSERT INTO environments (production)
5. INSERT INTO audit_log (project.created)

#### Environments

| Metodo | Path | Auth | RBAC | Detalle |
|---|---|---|---|---|
| GET | `/api/v1/projects/:slug/environments` | JWT | Viewer+ | Lista environments del project |
| POST | `/api/v1/projects/:slug/environments` | JWT | Admin+ | Crea environment adicional |

#### API Keys

| Metodo | Path | Auth | RBAC | Detalle |
|---|---|---|---|---|
| POST | `/api/v1/api-keys` | JWT | Admin+ | Crea key scoped a un environment. Retorna raw key UNA vez |
| GET | `/api/v1/api-keys` | JWT | Member+ | Lista keys (solo prefix visible, nunca el key completo) |
| DELETE | `/api/v1/api-keys/:id` | JWT | Admin+ | Revoca key (soft delete: sets revoked_at) |

**`POST /api-keys` response**:
```json
{
  "id": "...",
  "name": "production-backend",
  "key": "fs_live_a3b9d2c8e4f1g5h7j8k9m2n4p6q8r1s3t5",
  "prefix": "fs_live_a3b9",
  "environment_id": "...",
  "created_at": "..."
}
```

El `key` solo aparece en esta response. Nunca se almacena ni se retorna de nuevo.

#### Flags

| Metodo | Path | Auth | RBAC | Detalle |
|---|---|---|---|---|
| POST | `/api/v1/flags` | JWT | Member+ | Crea flag definition (project-level) |
| GET | `/api/v1/flags` | JWT | Viewer+ | Lista flags del project |
| GET | `/api/v1/flags/:key` | JWT | Viewer+ | Detalle de un flag (incluye configs por environment) |
| PUT | `/api/v1/flags/:key` | JWT | Member+ | Update flag definition (name, description, type, default_value) |
| DELETE | `/api/v1/flags/:key` | JWT | Admin+ | Archive flag (soft delete: sets archived_at) |

**`POST /flags`**:
1. Validar key format (`^[a-z0-9][a-z0-9_-]*[a-z0-9]$`, max 128)
2. INSERT INTO flags
3. Para cada environment del project: INSERT INTO flag_environments (enabled=false, rules=[], version=1)
4. INSERT INTO audit_log (flag.created)

#### Flag Environments

| Metodo | Path | Auth | RBAC | Detalle |
|---|---|---|---|---|
| PUT | `/api/v1/flags/:key/environments/:env_slug` | JWT | Member+ | Update per-env config (enabled, rules, default_value) con OCC |

**OCC (Optimistic Concurrency Control)**:
- Client envia `expected_version` en el body
- `UPDATE ... WHERE version = $expected_version`
- Si `rows_affected == 0` -> `409 Conflict` con `ErrVersionConflict`
- El trigger de la DB auto-incrementa `version` despues del UPDATE

#### Segments

| Metodo | Path | Auth | RBAC | Detalle |
|---|---|---|---|---|
| POST | `/api/v1/segments` | JWT | Member+ | Crea segment con JSONB rules |
| GET | `/api/v1/segments` | JWT | Viewer+ | Lista segments del project |
| GET | `/api/v1/segments/:key` | JWT | Viewer+ | Detalle de un segment |
| PUT | `/api/v1/segments/:key` | JWT | Member+ | Update segment |
| DELETE | `/api/v1/segments/:key` | JWT | Admin+ | Archive segment |

### Validacion de input (de SECURITY.md)

| Campo | Regla | Donde se aplica |
|---|---|---|
| Flag key | `^[a-z0-9][a-z0-9_-]*[a-z0-9]$`, max 128 | flags, segments |
| Slug | `^[a-z0-9]([a-z0-9-]*[a-z0-9])?$`, max 64 | projects, environments |
| Name | Non-empty, max 255 | todos |
| Description | Max 2000 | flags, segments |
| Pagination limit | 1-100, default 20 | list endpoints |
| Rules JSONB | Max depth 10, max 50 nodos, max 64KB, operator whitelist | flags, segments |
| Attribute names | `^[a-zA-Z_][a-zA-Z0-9_.]{0,63}$` | rules |

### RBAC matrix (de SECURITY.md)

| Accion | viewer | member | admin | owner |
|---|---|---|---|---|
| List/read flags | Yes | Yes | Yes | Yes |
| Create flag | - | Yes | Yes | Yes |
| Update flag | - | Yes | Yes | Yes |
| Archive flag | - | - | Yes | Yes |
| List/read segments | Yes | Yes | Yes | Yes |
| Create/update segments | - | Yes | Yes | Yes |
| Archive segment | - | - | Yes | Yes |
| List environments | Yes | Yes | Yes | Yes |
| Create environment | - | - | Yes | Yes |
| Delete environment | - | - | - | Yes |
| List keys (prefix) | - | Yes | Yes | Yes |
| Create key | - | - | Yes | Yes |
| Revoke key | - | - | Yes | Yes |

### Tests Fase 5

| Test | Que verifica |
|---|---|
| Projects CRUD | Create + auto-envs, List (tenant-scoped), Update, slug uniqueness |
| Environments CRUD | Create, List (project-scoped), slug uniqueness |
| API Keys CRUD | Create (raw key solo una vez), List (solo prefix), Revoke (revoked_at set) |
| Flags CRUD | Create + auto-flag_envs, List, Update, Archive, key validation |
| Flag Envs | Update con OCC (version conflict), enabled toggle, rules update |
| Segments CRUD | Create, List, Update, Archive, key validation |
| RBAC | Viewer no puede crear, member no puede archivar, admin puede todo menos delete env |
| Cross-tenant isolation | User de tenant A no ve recursos de tenant B |

### Checklist Fase 5

- [x] Crear `internal/api/projects.go` (POST, GET, GET/:slug, PUT/:slug)
- [x] Crear `internal/api/environments.go` (GET, POST, DELETE owner-only)
- [x] Crear `internal/api/apikeys.go` (POST, GET, DELETE/:id)
- [x] Crear `internal/api/flags.go` (POST, GET, GET/:key, PUT/:key, DELETE/:key)
- [x] Crear `internal/api/flag_envs.go` (GET + PUT con OCC)
- [x] Crear `internal/api/segments.go` (POST, GET, GET/:key, PUT/:key, DELETE/:key)
- [x] Tests: projects CRUD, auto-env creation (3 envs verificados), tenant-scoped
- [x] Tests: environments CRUD
- [x] Tests: API keys CRUD, raw key solo una vez, revoke
- [x] Tests: flags CRUD, auto-flag_envs verificado, key validation
- [x] Tests: flag_envs OCC version conflict
- [x] Tests: segments CRUD
- [x] Tests: RBAC enforcement por endpoint (`rbac_test.go` — matriz de roles × endpoint)
- [x] Tests: cross-tenant isolation (`cross_tenant_test.go` — projects, flags, segments, envs, apikeys)
- [x] Audit logs en todas las mutaciones (project.*, flag.*, segment.*, environment.*, apikey.*)
- [x] Validación description max 2000 chars en flags y segments
- [x] DELETE environment handler restringido a Owner+ (cascada borra flag_envs y api_keys)

### Lo que queda fuera del scope de Fase 5

- **Paginación** en list endpoints — el plan menciona `limit 1-100, default 20`, pero los volúmenes esperados en MVP (decenas de flags por proyecto) no lo justifican. Se agrega cuando un cliente lo pida.
- **JSONB rule validation** (max depth 10, max 50 nodos, max 64KB, operator whitelist) — la validación estructural de Rules sucede en la frontera del engine (Fase 7), no en los CRUDs. Por ahora solo se valida `json.Valid`.

---

## 10. Fase 6 — Audit Log Endpoint

**Objetivo:** Query del audit log para dashboard. Los writes ya se hacen en cada handler de mutacion (Fases 3-5).

### Estructura de archivos

```
internal/api/handlers/
├── audit.go       # GET /api/v1/audit
└── audit_test.go
```

### Detalle

#### `handlers/audit.go`

```
GET /api/v1/audit?actor_id=...&resource_type=flag&resource_id=...&since=...&until=...&limit=20&offset=0
Auth: JWT
RBAC: Viewer+

Response 200:
{
  "entries": [
    {
      "id": "...",
      "actor_id": "...",
      "actor_type": "user",
      "action": "flag.created",
      "resource_type": "flag",
      "resource_id": "...",
      "changes": { "before": null, "after": { "key": "new-checkout" } },
      "created_at": "..."
    }
  ],
  "total": 42,
  "limit": 20,
  "offset": 0
}
```

**Query params**:
- `actor_id` (UUID) — filtrar por quien hizo la accion
- `actor_type` (user|api_key|system) — filtrar por tipo de actor
- `resource_type` (flag|segment|environment|project|api_key) — filtrar por tipo de recurso
- `resource_id` (UUID) — filtrar por recurso especifico
- `action` (flag.created|auth.login|etc.) — filtrar por accion
- `since` (RFC3339) — desde cuando
- `until` (RFC3339) — hasta cuando
- `limit` (1-100, default 20) — pagination
- `offset` (>= 0) — pagination

**Query SQL**:
```sql
SELECT * FROM audit_log
WHERE tenant_id = $1
  AND (actor_id = $2 OR $2 IS NULL)
  AND (resource_type = $3 OR $3 IS NULL)
  AND (resource_id = $4 OR $4 IS NULL)
  AND (created_at >= $5 OR $5 IS NULL)
  AND (created_at <= $6 OR $6 IS NULL)
ORDER BY created_at DESC
LIMIT $7 OFFSET $8
```

### Tests Fase 6

| Test | Que verifica |
|---|---|
| Query sin filtros | Retorna todos los entries del tenant, ordenados por created_at DESC |
| Query con filtros | Filtra por actor_id, resource_type, resource_id, time range correctamente |
| Pagination | limit/offset funcionan, total count correcto |
| Cross-tenant | User de tenant A no ve entries de tenant B |
| Writes de fases anteriores | Verificar que los CRUD tests generaron audit entries |

### Checklist Fase 6

- [x] Crear `internal/api/audit.go` (GET /api/v1/audit con filtros: actor_id, actor_type, action, resource_type, resource_id, since, until, limit, offset)
- [x] Tests: query sin filtros (`TestAuditLog_QueryNoFilters`)
- [x] Tests: con filtros — action, actor_type, resource_id (`TestAuditLog_QueryWithFilters`)
- [x] Tests: paginación — limit/offset, total count correcto (`TestAuditLog_Pagination`)
- [x] Tests: cross-tenant isolation — tenant A no ve entries de tenant B (`TestAuditLog_CrossTenant`)
- [x] Tests: sin auth → 401 (`TestAuditLog_NoAuth`)
- [x] Ruta: `GET /api/v1/audit` con `AuthJWT + RequireRole(Viewer+)`
- [x] Tests: mutations generan audit entries visibles via `GET /api/v1/audit` (`TestAuditLog_MutationsWriteEntries` — crea project + flag via HTTP, verifica que ambas acciones aparecen en el log, y que el filtro `resource_type=flag` las reduce correctamente)

---

## 11. Fase 7 — Rule Evaluation Engine

**Objetivo:** Productionizar el engine. Funciones puras, sin I/O, altamente testeables.

**Prerequisito**: Fase 4.5 (spike) ya implemento el core de `engine.Evaluate` con un subset de operadores. Esta fase **completa** lo que el spike dejo: los 15 operadores, panic recovery, structured logging, y los casos edge.

**Diferencia con el spike**:

| Aspecto | Fase 4.5 (spike) | Fase 7 (engine completo) |
|---|---|---|
| Operadores | ~7 (representativos) | 15 completos |
| Panic recovery | No | Si (cada Evaluate dentro de defer/recover) |
| Logging | Nada o minimo | zap structured logs por path de error |
| Performance | No medido | Benchmarked, target < 1ms por flag |
| `EvaluateAll` (bulk) | No implementado | Si — para Fase 8 |
| Error policy completa | No | Si — toda la tabla de "Resilience over Correctness" |
| Type coercion | Casos basicos | Tabla completa de coercions |

### Estructura de archivos

```
internal/engine/
├── engine.go         # Engine struct, Evaluate(), EvaluateAll()
├── types.go          # EvaluateRequest, EvaluateResult, FlagConfig, Reason enum
├── conditions.go     # evaluateNode() — recursive DFS tree walker
├── operators.go      # 15 operadores con type coercion
├── rollout.go        # inRollout() — FNV-1a consistent hashing
├── segments.go       # resolveSegment() con cycle detection
├── engine_test.go
├── conditions_test.go
├── operators_test.go
├── rollout_test.go
└── segments_test.go
```

### Detalle por archivo

#### `types.go`

```go
type Reason string

const (
    ReasonRuleMatch    Reason = "RULE_MATCH"
    ReasonDefault      Reason = "DEFAULT"
    ReasonDisabled     Reason = "DISABLED"
    ReasonFlagNotFound Reason = "FLAG_NOT_FOUND"
    ReasonFlagArchived Reason = "FLAG_ARCHIVED"
    ReasonInternalError Reason = "INTERNAL_ERROR"
)

type EvaluateResult struct {
    Value     any    // bool (MVP), despues string/number/json para multivariate
    Reason    Reason
    RuleIndex int    // -1 si no matcheo ninguna regla
}

type EvaluateRequest struct {
    Flag     *FlagConfig
    Segments map[SegmentKey]*Segment  // preloaded by caller
    Context  map[string]any           // user attributes
}

type FlagConfig struct {
    Key          string
    Type         string       // "boolean", "string", "number", "json"
    DefaultValue any          // project-level default
    Enabled      bool         // master kill switch
    Rules        []Rule       // parsed from JSONB
    EnvDefault   *any         // per-env override
    Version      int64
}

type Rule struct {
    Conditions ConditionNode
    Rollout    *RolloutConfig  // nil = 100% match
    Value      any             // override value on match
}

type ConditionNode struct {
    Attribute string          // leaf: key en user context
    Op        string          // leaf: operador
    Value     any             // leaf: valor a comparar
    All       []ConditionNode // AND
    Any       []ConditionNode // OR
    Not       *ConditionNode  // NOT
}

type RolloutConfig struct {
    Percentage int    // 0-100
    Seed       string // override hash seed (default: flag_key)
}
```

#### `engine.go`

```go
type Engine struct {
    logger *zap.Logger
}

func New(logger *zap.Logger) *Engine

func (e *Engine) Evaluate(req EvaluateRequest) EvaluateResult
func (e *Engine) EvaluateAll(flags []*FlagConfig, segments map[SegmentKey]*Segment, ctx map[string]any) map[string]EvaluateResult
```

**Evaluation flow** (de DESIGN.md):

```
Input: (flag_key, user_context)
  ↓
1. Check enabled = false? → return { false, "DISABLED" }
  ↓
2. For each rule in order (first match wins):
   ├─ Evaluate rule.conditions (recursive tree walk)
   ├─ Conditions matched?
   │   ├─ No → continue to next rule
   │   └─ Yes → check rollout
   │       ├─ No rollout config? → return { rule.value, "RULE_MATCH", index }
   │       └─ Has rollout?
   │           ├─ No user_id in context? → skip rule (not matched), warning log
   │           ├─ hash(seed + ":" + user_id) % 100 < percentage?
   │           │   ├─ Yes → return { rule.value, "RULE_MATCH", index }
   │           │   └─ No → continue to next rule (NOT default — fall through)
   │
3. No rules matched → return { default_value, "DEFAULT", -1 }
```

**Error policy** (de DESIGN.md — "Resilience over Correctness"):

| Scenario | Comportamiento |
|---|---|
| Flag not found | Caller maneja (engine asume FlagConfig valida) |
| Flag disabled | Retorna `false` + `DISABLED` |
| Malformed rule | Skip rule, continue to next, error log |
| Unknown operator | Condition -> `false`, continue, error log |
| Segment not found | Condition -> `false`, continue, warning log |
| Segment circular ref | Condition -> `false`, continue, error log |
| Regex compile fail | Condition -> `false`, continue, error log |
| Type coercion fail | Condition -> `false`, continue, debug log |
| Panic en eval | Recover, return `false` + `INTERNAL_ERROR`, critical log |

**Panic recovery**: Cada evaluacion corre dentro de `defer func() { recover() }()`. Un bug en rule evaluation nunca debe crashear el proceso.

#### `conditions.go` — Recursive tree walker

```go
func (e *Engine) evaluateNode(node ConditionNode, ctx map[string]any, segments map[SegmentKey]*Segment, visited map[string]bool) bool
```

- `All` node: short-circuit AND (para en el primer `false`)
- `Any` node: short-circuit OR (para en el primer `true`)
- `Not` node: invierte el resultado del hijo
- Leaf node: delega a `operators.go`

#### `operators.go` — 15 operadores

| Operador | Tipo | Descripcion |
|---|---|---|
| `eq` | any | Igualdad exacta (con type coercion) |
| `neq` | any | No igual |
| `gt` | number | Mayor que |
| `gte` | number | Mayor o igual |
| `lt` | number | Menor que |
| `lte` | number | Menor o igual |
| `in` | array | Valor esta en la lista |
| `not_in` | array | Valor no esta en la lista |
| `contains` | string | Substring match |
| `starts_with` | string | Prefix match |
| `ends_with` | string | Suffix match |
| `matches` | string | Regex match (RE2 syntax) |
| `exists` | - | Attribute esta presente en context |
| `not_exists` | - | Attribute NO esta presente en context |
| `segment` | string | User matches named segment (recursive eval) |

**Type coercion rules**: El engine compara segun el tipo del `value` en la regla. Si el atributo del contexto no se puede coercer (ej: `"abc"` vs numerico `gt`), la condicion evalua a `false` — nunca error.

**`segment` operator**: Evalua el condition tree del segmento recursivamente. Cycle detection con `visited` set. Si cycle -> `false` + warning log.

**`matches` operator**: Usa `regexp.MatchString` (RE2 syntax). Go's RE2 garantiza O(n) time complexity — no es vulnerable a ReDoS. Invalid regex -> `false` + error log.

#### `rollout.go` — FNV-1a consistent hashing

```go
func inRollout(seed, userID string, percentage int) bool {
    if percentage >= 100 { return true }
    if percentage <= 0 { return false }
    h := fnv.New32a()
    h.Write([]byte(seed + ":" + userID))
    bucket := h.Sum32() % 100
    return bucket < uint32(percentage)
}
```

**Propiedades**:
- Deterministico: mismo user + mismo flag = mismo resultado siempre
- Monotonico: 10% es subset de 25% (al aumentar, nadie sale del grupo)
- `seed` defaults a `flag_key` — incluye el flag_key en el hash para que el mismo user no siempre este en el mismo porcentaje para todos los flags
- `seed` custom permite correlated rollouts (mismo seed = mismos users) o decorrelated (diferente seed = diferentes users)

**Justificacion FNV-1a** (de DESIGN.md): No necesitamos resistencia criptografica. FNV esta en la standard library de Go y es suficiente. Si benchmarks muestran que es el bottleneck (improbable), se cambia.

#### `segments.go` — Cycle detection

```go
func (e *Engine) resolveSegment(key SegmentKey, ctx map[string]any, segments map[SegmentKey]*Segment, visited map[string]bool) bool
```

- Si `key` no existe en `segments` -> `false` + warning log
- Si `key` ya esta en `visited` -> `false` + error log (circular reference)
- Si no, agregar a `visited` y evaluar el condition tree del segmento

### Tests Fase 7 — La mas testeada del proyecto

#### `engine_test.go`

| Test | Input | Expected |
|---|---|---|
| No rules -> default | `rules: []` | `{ false, "DEFAULT", -1 }` |
| Disabled flag | `enabled: false` | `{ false, "DISABLED", -1 }` |
| Single rule match | `rules: [{conditions: eq, value: true}]` | `{ true, "RULE_MATCH", 0 }` |
| Rule order (first match wins) | 2 rules, ambos matchean | Primer rule gana |
| Rollout miss falls through | Rule 1 rollout 50% (miss), Rule 2 match | Rule 2 evalua |
| Default value resolution | No rules match, env_default set | `{ env_default, "DEFAULT", -1 }` |

#### `conditions_test.go`

| Test | Node type | Expected |
|---|---|---|
| `all` match | `{all: [eq, eq]}` | `true` |
| `all` short-circuit | `{all: [eq(true), eq(false)]}` | `false` (para en segundo) |
| `any` match | `{any: [eq(false), eq(true)]}` | `true` |
| `any` short-circuit | `{any: [eq(true), eq(false)]}` | `true` (para en primero) |
| `not` | `{not: {eq}}` | inverted |
| Nested `all` in `any` | `{any: [{all: [eq, eq]}, eq]}` | correcto |
| Max depth (10 levels) | deeply nested | no stack overflow |

#### `operators_test.go` — ~45 test cases

| Operador | Test cases |
|---|---|
| `eq` | string match, number match, bool match, type mismatch, missing attr |
| `neq` | string no match, number no match |
| `gt/gte/lt/lte` | number comparisons, string input (-> false), missing attr |
| `in` | value in list, value not in list, empty list |
| `not_in` | value not in list, value in list |
| `contains` | substring found, not found, case sensitive |
| `starts_with` | prefix match, no match |
| `ends_with` | suffix match, no match |
| `matches` | valid regex match, no match, invalid regex (-> false) |
| `exists` | attr present, attr absent |
| `not_exists` | attr absent, attr present |
| `segment` | segment match, segment not found (-> false) |

#### `rollout_test.go`

| Test | Expected |
|---|---|
| 0% -> nobody pasa | `inRollout(seed, anyID, 0) == false` |
| 100% -> todos pasan | `inRollout(seed, anyID, 100) == true` |
| Deterministico | Mismo seed + mismo userID = mismo resultado en 1000 llamadas |
| Monotonico | Users en 10% estan en 25% (subset) |
| No user_id -> skip | Rollout sin user_id = not matched (fall through) |
| Custom seed | Diferente seed = diferente cohort de users |
| flag_key en hash | Mismo user en diferentes flags = diferente bucket |

#### `segments_test.go`

| Test | Expected |
|---|---|
| Segment normal | Evalua condition tree del segmento correctamente |
| Segment not found | `false` + warning log |
| Circular A->B->A | `false` + error log, no infinite loop |
| Transitive A->B->C | Evalua toda la cadena correctamente |

### Checklist Fase 7

- [x] Crear `internal/engine/types.go` (EvaluateRequest, EvaluateResult, FlagConfig, Reason, Rule, ConditionNode, RolloutConfig)
- [x] Crear `internal/engine/engine.go` (Evaluate, EvaluateAll, panic recovery)
- [x] Crear `internal/engine/conditions.go` (evaluateNode, all/any/not/leaf, depth limit 10)
- [x] Crear `internal/engine/operators.go` (15 operadores, type coercion, compareNumber factorizado)
- [x] Crear `internal/engine/rollout.go` (FNV-1a, consistent hashing)
- [x] Crear `internal/engine/segments.go` (resolveSegment, cycle detection)
- [x] Tests: `engine_test.go` — default, disabled, rule match, first match wins, fall through, EvaluateAll
- [x] Tests: `conditions_test.go` — all, any, not, nested, short-circuit
- [x] Tests: `operators_test.go` — los 15 operadores con match, no match, type mismatch, missing attr
- [x] Tests: `rollout_test.go` — 0%, 100%, deterministico, monotonic, seed affects bucket, empty userID
- [x] Tests: `segments_test.go` — normal, not found, circular, transitive (nested)

---

## 12. Fase 8 — Evaluate Endpoints

**Objetivo:** Los endpoints hot-path que usan los SDKs.

### Estructura de archivos

```
internal/api/handlers/
├── evaluate.go      # POST /evaluate/flags/:key, POST /evaluate/flags
└── evaluate_test.go
```

### Detalle

#### `handlers/evaluate.go`

**`POST /api/v1/evaluate/flags/:key`** — Single flag evaluation:

```
Auth: API Key (scoped a environment)
Content-Type: application/json

Body:
{
  "context": {
    "user_id": "u_abc123",
    "country": "AR",
    "plan": "premium",
    "is_admin": true
  }
}

Response 200:
{
  "key": "new-checkout",
  "value": true,
  "reason": "RULE_MATCH",
  "rule_index": 0,
  "request_id": "req_d4f8a2"
}
```

**Flow**:
1. API key auth -> resolve `environment_id`
2. Load flag config: `SELECT f.*, fe.* FROM flags f JOIN flag_environments fe ON fe.flag_id = f.id WHERE f.key = $1 AND fe.environment_id = $2`
3. Load project segments: `SELECT * FROM segments WHERE project_id = $3 AND archived_at IS NULL`
4. `engine.Evaluate(FlagConfig, segments, context)`
5. Retornar resultado

**Nota**: No hay 404 para flag not found. Un flag inexistente retorna `200 OK` con `value: false` y `reason: "FLAG_NOT_FOUND"`. Esto es intencional (de DESIGN.md) — los SDKs nunca deben crashear porque un flag no existe.

**`POST /api/v1/evaluate/flags`** — Bulk evaluation (SDK bootstrap):

```
Auth: API Key
Content-Type: application/json

Body:
{
  "context": {
    "user_id": "u_abc123",
    "country": "AR",
    "plan": "premium"
  }
}

Response 200:
{
  "flags": {
    "new-checkout": { "value": true, "reason": "RULE_MATCH", "rule_index": 0 },
    "dark-mode": { "value": false, "reason": "DISABLED", "rule_index": -1 },
    "beta-dashboard": { "value": false, "reason": "DEFAULT", "rule_index": -1 }
  },
  "environment": "production",
  "evaluated_at": "2024-11-15T03:22:41Z",
  "request_id": "req_f7a2c4"
}
```

**Flow**:
1. API key auth -> resolve `environment_id`
2. Bulk load ALL flags: single JOIN query (como en Fase 1)
3. Load ALL project segments: single query
4. `engine.EvaluateAll(flags, segments, context)`
5. Retornar mapa de resultados

**Performance** (de DESIGN.md): O(F x R) donde F = numero de flags, R = reglas promedio por flag. Para 50 flags x 5 reglas = < 10ms.

#### Context validation

| Regla | Accion |
|---|---|
| Max 100 keys | 400 Bad Request |
| Max 128 chars per key | 400 Bad Request |
| Max 1024 chars per string value | 400 Bad Request |
| Tipo no permitido (ej: object, null) | 400 Bad Request |
| Tipos permitidos | string, number (float64), boolean, []string |

### Tests Fase 8

| Test | Que verifica |
|---|---|
| Single eval: flag existe | Retorna valor correcto con reason |
| Single eval: flag not found | 200 con `value: false, reason: "FLAG_NOT_FOUND"` |
| Single eval: flag disabled | 200 con `value: false, reason: "DISABLED"` |
| Single eval: rule match | 200 con `value: true, reason: "RULE_MATCH"` |
| Single eval: rollout | Deterministico para mismo user_id |
| Bulk eval: retorna todos los flags activos | Mapa con todos los flags evaluados |
| Bulk eval: formato correcto | environment, evaluated_at, request_id presentes |
| Auth: API key invalido | 401 Unauthorized |
| Auth: API key revocado | 401 Unauthorized |
| Context validation: too many keys | 400 Bad Request |
| Context validation: value too long | 400 Bad Request |
| Context validation: invalid type | 400 Bad Request |

### Checklist Fase 8

- [x] Crear `internal/api/evaluate.go` (single + bulk evaluation)
- [x] Implementar context validation (max 100 keys, 128 chars/key, 1024 chars/value, tipos: string/float64/bool/[]string)
- [x] Implementar eager loading: bulk usa `ListByEnvironment` (JOIN en una query), single carga flag + flagEnv + segments
- [x] `parseDefaultValue` tipado por `flag.Type` para que el engine reciba el tipo correcto
- [x] `buildFlagConfig` + `buildFlagConfigFromJoined` — dos helpers limpios para los dos flujos
- [x] Tests: single eval (flag not found, disabled, rule match, default value)
- [x] Tests: bulk eval (success, formato correcto con environment/evaluated_at)
- [x] Tests: auth (API key invalido → 401)
- [x] Tests: context validation (too many keys, value too long, invalid type)

- [x] Test: API key revocado → 401 (`TestEvaluateFlag_RevokedAPIKey`)
- [x] Test: rollout deterministico end-to-end via HTTP (`TestEvaluateFlag_RolloutDeterministic` — 10 llamadas al mismo user, mismo resultado)
- [x] Test: sin header Authorization → 401 (`TestEvaluateFlag_NoAuthHeader`)

---

## 13. Fase 9 — CI Pipeline + Integracion Final

**Objetivo:** CI verde, smoke tests end-to-end, pipeline funcional.

### Acciones

#### 1. Descomentar `.github/workflows/ci.yml`

El archivo ya existe con un pipeline completo pero todo comentado. Se necesita:
- Descomentar todo
- Ajustar para usar `docker-compose` en lugar de testcontainers
- Agregar servicio de Postgres + Redis como `services` del workflow
- Configurar `DATABASE_URL` y `REDIS_URL` como environment variables

#### 2. Pipeline CI estructura

```yaml
name: CI
on: [push, pull_request]
jobs:
  lint:
    runs-on: ubuntu-latest
    steps:
      - checkout
      - setup Go 1.26
      - go mod download
      - golangci-lint run

  test:
    runs-on: ubuntu-latest
    services:
      postgres: postgres:16-alpine
      redis: redis:7-alpine
    steps:
      - checkout
      - setup Go 1.26
      - go mod download
      - run migrations
      - go test -race ./...

  build:
    runs-on: ubuntu-latest
    steps:
      - checkout
      - setup Go 1.26
      - go build ./cmd/flagstone
```

#### 3. Smoke test manual

Flow completo end-to-end via curl:

```bash
# 1. Bootstrap
curl -X POST http://localhost:8080/api/v1/setup \
  -H "Content-Type: application/json" \
  -d '{"tenant_name":"Test Corp","admin_email":"admin@test.com","admin_password":"test12345678"}'

# 2. Crear project
curl -X POST http://localhost:8080/api/v1/projects \
  -H "Authorization: Bearer $ACCESS_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"name":"My App","slug":"my-app"}'

# 3. Crear API key
curl -X POST http://localhost:8080/api/v1/api-keys \
  -H "Authorization: Bearer $ACCESS_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"name":"test-key","environment_slug":"production"}'

# 4. Crear flag
curl -X POST http://localhost:8080/api/v1/flags \
  -H "Authorization: Bearer $ACCESS_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"key":"new-checkout","name":"New Checkout","type":"boolean"}'

# 5. Configurar flag en environment (enable + rules)
curl -X PUT "http://localhost:8080/api/v1/flags/new-checkout/environments/production" \
  -H "Authorization: Bearer $ACCESS_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"enabled":true,"rules":[{"conditions":{"attribute":"plan","op":"eq","value":"premium"},"value":true}]}'

# 6. Evaluar flag
curl -X POST http://localhost:8080/api/v1/evaluate/flags/new-checkout \
  -H "Authorization: Bearer $API_KEY" \
  -H "Content-Type: application/json" \
  -d '{"context":{"user_id":"u123","plan":"premium"}}'
# Expected: {"key":"new-checkout","value":true,"reason":"RULE_MATCH","rule_index":0}

# 7. Evaluar flag (user no premium)
curl -X POST http://localhost:8080/api/v1/evaluate/flags/new-checkout \
  -H "Authorization: Bearer $API_KEY" \
  -H "Content-Type: application/json" \
  -d '{"context":{"user_id":"u456","plan":"free"}}'
# Expected: {"key":"new-checkout","value":false,"reason":"DEFAULT","rule_index":-1}
```

### Checklist Fase 9

- [ ] Descomentar `.github/workflows/ci.yml`
- [ ] Ajustar CI para usar docker-compose / services de GitHub Actions
- [ ] Configurar DATABASE_URL y REDIS_URL en CI
- [ ] Verificar que `make lint` pasa en CI
- [ ] Verificar que `make test` pasa en CI
- [ ] Verificar que `make build` pasa en CI
- [ ] Ejecutar smoke test manual completo (setup -> projects -> api-keys -> flags -> evaluate)
- [ ] Verificar audit log entries se crearon correctamente
- [ ] Documentar el flow completo como ejemplo en README o docs

---

## 14. Dependencias entre Fases

### Orden recomendado (con engine spike)

```
Fase 0: Foundation
  │
  ▼
Fase 1: Storage Layer  (depende de Postgres)
  │
  ▼
Fase 2: Auth Core  (codigo puro)
  │
  ▼
Fase 3: Middleware + Bootstrap  (depende de Fases 1 + 2)
  │
  ▼
Fase 4: Auth Endpoints  (depende de Fase 3)
  │
  ▼
Fase 4.5: Engine Spike  ← validacion de modelo antes de cementar CRUDs
  │     │
  │     └─ Decision: modelo OK / migracion 000004 / refactor mayor
  ▼
Fase 5: CRUD Endpoints  (depende de Fase 3, requiere spike validado)
  │
  ▼
Fase 7: Rule Engine completo  (productionizacion de lo del spike)
  │
  ▼
Fase 8: Evaluate Endpoints  (depende de Fases 5 + 7)
  │
  ▼
Fase 6: Audit Log Endpoint  (depende de Fase 5)
  │
  ▼
Fase 9: CI Pipeline + Integracion Final
```

### Justificacion del orden

- **5 → 7 → 8 → 6 → 9**: hace primero la superficie CRUD para tener una plataforma administrable, despues productioniza el engine, despues los evaluate endpoints (que combinan stores + engine), despues el audit query (sobre datos reales generados por CRUDs), y al final CI.
- **4.5 antes que 5**: valida el modelo de datos antes de comprometerlo en handlers publicos. Si el spike fuerza una migracion 000004, mejor hacerla antes de tener CRUDs entregados que depender de la forma del JSONB.
- **6 despues que 5**: el audit query es trivial de implementar, pero es mas util tener datos reales para testearlo. Los writes ya se hacen en cada handler de mutacion.

### Paralelizable

- **Fase 2 y Fase 1** se pueden hacer en paralelo si hay dos desarrolladores (auth no depende de storage).
- **Fase 4.5 (spike) y storage tests pendientes de Fase 4** se pueden paralelizar si hay dos desarrolladores.

---

## 15. Estimacion de Esfuerzo

| Fase | Archivos nuevos | Lineas aprox | Complejidad | Tests |
|---|---|---|---|---|
| 0 — Foundation | 2 editados + 1 nuevo | ~150 | Baja | Manual |
| 1 — Storage | 14 + 11 test | ~2000-2500 | Media | ~50 test cases |
| 2 — Auth Core | 7 + 6 test | ~500-600 | Media | ~25 test cases |
| 3 — Middleware | 12 + 9 test | ~700-900 | Media | ~20 test cases |
| 4 — Auth Endpoints | 1 + 1 test | ~600-800 (con T19/T20 + multi-tenant) | Media | ~20 test cases |
| 4.5 — Engine Spike | 6 + 5 test | ~400-600 | **Alta** | ~15 test cases (1 por riesgo) |
| 5 — CRUD | 6 + 6 test | ~900-1200 | Media | ~30 test cases |
| 7 — Engine completo | (los del spike + cierres) | ~300-500 incrementales | **Alta** | ~45 test cases incrementales |
| 8 — Evaluate | 1 + 1 test | ~250-350 | Media | ~12 test cases |
| 6 — Audit | 1 + 1 test | ~100-150 | Baja | ~5 test cases |
| 9 — CI + Smoke | 1 editado | ~50 | Baja | Manual |
| **Total** | **~66 archivos** | **~6500-9000** | — | **~225 test cases** |

---

## Referencias Cruzadas

| Seccion del plan | Referencia en docs |
|---|---|
| Pool sizing | DESIGN.md → Database connection pool sizing |
| Auth model | SECURITY.md → Authentication |
| RBAC | SECURITY.md → Authorization (RBAC) |
| JWT claims | SECURITY.md → Access token structure |
| API key format | SECURITY.md → Key format |
| Bootstrap TOCTOU | SECURITY.md → Threat T18 |
| Error policy | DESIGN.md → Error Policy: Resilience over Correctness |
| Evaluation flow | DESIGN.md → Evaluation Flow (complete) |
| Consistent hashing | DESIGN.md → Consistent hashing for rollouts |
| Rule operators | DESIGN.md → Supported operators |
| API contracts | DESIGN.md → Evaluation API Contracts |
| Health checks | DESIGN.md → Health Check Design |
| Middleware order | DESIGN.md → Middleware stack order |
| OCC | DESIGN.md → Why version in flag_environments |
| Eager loading | DESIGN.md → Eager loading: avoiding N+1 queries |
| Logging | DESIGN.md → Logging |
| Value types | DESIGN.md → Value types for critical IDs |
| Table-driven tests | DESIGN.md → Table-driven tests for the engine |

---

---

# Flagstone — Plan de Implementacion Milestone 2 (Web Dashboard)

> Plan detallado para el dashboard React/Next.js. Cada fase entrega pantallas
> funcionales testeadas. Se avanza pantalla por pantalla en orden.

---

## Tabla de Contenidos M2

1. [Stack y Justificacion](#m2-1-stack-y-justificacion)
2. [Estructura de Archivos](#m2-2-estructura-de-archivos)
3. [Convenciones de Codigo](#m2-3-convenciones-de-codigo)
4. [Estrategia de Testing](#m2-4-estrategia-de-testing)
5. [Buenas Practicas Globales](#m2-5-buenas-practicas-globales)
6. [Fase W0 — Scaffolding](#m2-6-fase-w0--scaffolding)
7. [Fase W1 — Login](#m2-7-fase-w1--login)
8. [Fase W2 — Projects List](#m2-8-fase-w2--projects-list)
9. [Fase W3 — Flags List](#m2-9-fase-w3--flags-list)
10. [Fase W4 — Project Settings](#m2-10-fase-w4--project-settings)
11. [Fase W5 — API Keys](#m2-11-fase-w5--api-keys)
12. [Fase W6 — Audit Log](#m2-12-fase-w6--audit-log)
13. [Fase W7 — Account](#m2-13-fase-w7--account)
14. [Fase W8 — Setup](#m2-14-fase-w8--setup)
15. [Fase W9 — Rule Editor](#m2-15-fase-w9--rule-editor)
16. [Fase W10 — Segments](#m2-16-fase-w10--segments)
17. [Dependencias entre Fases W](#m2-17-dependencias-entre-fases-w)
18. [Estimacion de Esfuerzo](#m2-18-estimacion-de-esfuerzo)

---

## M2-1. Stack y Justificacion

### Framework principal

| Tecnologia | Version | Razon |
|---|---|---|
| Next.js | 16.2.6 | Parchea CVE-2025-55182 (RCE pre-auth CVSS 9.5) y CVE-2025-29927 (middleware auth bypass CVSS 9.5). App Router = Server Components por defecto, file-based routing, Server Actions. |
| React | 19.2.6 | Parchea CVE-2025-55182 en RSC. `useActionState`, `useFormStatus`, `use()` hook. |
| TypeScript | 5.x (strict) | `strict: true`. Sin `any`. Errores en compile-time, no en runtime. Autocomplete exacto para los tipos del API backend. |

**Por que Next.js sobre Vite+React o Remix:**
- App Router permite que cada pagina sea un Server Component que fetchea datos directamente, sin waterfalls cliente ni useEffect para data loading.
- File-based routing mapea 1:1 con las 10 pantallas del wireframe.
- Server Actions para mutaciones (crear flag, archivar, crear API key) sin necesidad de un endpoint BFF separado.
- Middleware de Next.js para proteccion de rutas en el edge sin round-trip al servidor.

### Estilos

| Tecnologia | Version | Razon |
|---|---|---|
| Tailwind CSS | v4 | Utility-first. Sin CSS files propios, sin naming wars, sin especificidad conflicts. `@import "tailwindcss"` + deteccion automatica. |

Color primario: `#364fc7` (indigo). Definido una vez en CSS custom properties, usado en todo el sistema via `text-primary`, `bg-primary`, etc.

**Descartado:** CSS Modules (requiere naming de clases, archivos separados), styled-components (runtime CSS-in-JS, peor performance).

### Componentes

| Tecnologia | Razon |
|---|---|
| shadcn/ui | Los componentes se **copian** al proyecto via `npx shadcn add`, no son una dependencia NPM. Control total, customizable sin hacks, construido sobre Radix UI (accesibilidad ARIA + keyboard nav incluida gratis). |
| lucide-react | Icons SVG como componentes React. Tree-shakeable. Consistente con el estilo del wireframe. |

Componentes shadcn que se van a usar: `Button`, `Input`, `Label`, `Badge`, `Table`, `Dialog`, `Switch`, `Select`, `Tabs`, `Textarea`, `Separator`, `Avatar`, `DropdownMenu`, `Alert`, `Skeleton`.

**Descartado:** MUI / Ant Design (dependencias pesadas con estilos propios que pelean con Tailwind), Radix directo (shadcn ya lo abstrae con Tailwind).

### Forms y validacion

| Tecnologia | Razon |
|---|---|
| react-hook-form | Formularios sin re-renders innecesarios (uncontrolled inputs). Integra con Server Actions y shadcn/ui via `register` + `Controller`. |
| zod | Validacion de schemas. Usado en el cliente para validacion instantanea antes de submit. Los mismos schemas se pueden usar en Server Actions para validar en el servidor tambien. |

Formularios que los usan: Login, Setup, Create Project, Create Flag, Create Segment, Create API Key, Edit rule conditions.

### Auth

- JWT del backend Go llega como `access_token` en body del login response.
- El frontend lo persiste en un **httpOnly cookie** via una Route Handler de Next.js (`app/api/auth/login/route.ts`) — nunca expuesto a `document.cookie` ni `localStorage`.
- El middleware de Next.js (`middleware.ts`) lee el cookie en cada request. **No verifica la firma** (no tiene el JWT_SECRET) — solo parsea el claim `exp` del payload (base64, sin firma) para saber si el token esta expirado. Si expirado o ausente, redirige a `/login`.
- El `access_token` tiene TTL de 15 minutos. Para evitar logouts frecuentes, `apiFetch` en `lib/api.ts` intercepta respuestas 401 con un retry automatico: llama `POST /api/auth/refresh` (Route Handler local que rota el refresh cookie httpOnly) y reintenta el request original una sola vez. Si el refresh tambien falla (refresh expirado, revocado, reuse detectado), redirige a `/login`.
- En rutas publicas (`/login`, `/setup`) el middleware redirige a `/projects` si ya hay sesion valida.
- Sin NextAuth, Lucia, ni libros de auth de terceros — el backend ya hace toda la auth.

**Flujo de tokens en el frontend:**
```
request → middleware lee exp claim del cookie
  ├─ cookie ausente / exp pasado → redirect /login
  └─ cookie presente y vigente → pasa

fetch en Server/Client Component
  └─ apiFetch()
       ├─ 200 → ok
       ├─ 401 → llama /api/auth/refresh (Route Handler)
       │         ├─ refresh ok → nuevo access_token cookie → retry request original
       │         └─ refresh falla → redirect /login
       └─ otros 4xx/5xx → ApiError tipado
```

### Estado

- **Sin Redux, Zustand, Jotai, ni Context global.**
- Server Components fetchean directamente con funciones de `lib/api.ts`.
- Client Components usan `useState` / `useReducer` local.
- `router.refresh()` re-ejecuta Server Components despues de mutaciones (sin full page reload).
- El unico "estado global" es la sesion, que vive en el httpOnly cookie y se lee en Server Components via `cookies()`.

### Comunicacion con el backend Go

- Backend Go: `:8080`. Next.js: `:3000`.
- En dev, `next.config.ts` define `rewrites()` para que `/api/v1/*` proxye a `http://localhost:8080/api/v1/*`. El browser nunca habla directamente al backend (evita CORS config en el backend).
- En produccion, Docker Compose pone ambos en la misma red interna. El proxy Next.js sigue funcionando.
- `lib/api.ts` es el unico lugar que sabe la URL base del backend. Todos los fetches pasan por ahi.

### Contenedor

```dockerfile
FROM node:22-alpine AS builder
WORKDIR /app
COPY package*.json ./
RUN npm ci
COPY . .
RUN npm run build

FROM node:22-alpine AS runner
WORKDIR /app
ENV NODE_ENV=production
COPY --from=builder /app/.next/standalone ./
COPY --from=builder /app/.next/static ./.next/static
COPY --from=builder /app/public ./public
EXPOSE 3000
CMD ["node", "server.js"]
```

`output: "standalone"` en `next.config.ts` produce bundle minimo (~120MB imagen final).

---

## M2-2. Estructura de Archivos

```
web/
├── package.json                    # name: "flagstone-web", port: 3000
├── next.config.ts                  # rewrites /api/v1/* -> backend, output standalone
├── tsconfig.json                   # strict: true, paths alias @/*
├── tailwind.config.ts              # color primario, fuente
├── components.json                 # shadcn config (style, baseColor, etc.)
├── .env.local                      # NEXT_PUBLIC_APP_URL, BACKEND_URL (server-side only)
├── Dockerfile                      # multi-stage, node:22-alpine
│
├── app/                            # App Router
│   ├── layout.tsx                  # Root layout: fuente, html lang, metadata global
│   ├── globals.css                 # @import "tailwindcss", CSS custom properties
│   ├── page.tsx                    # Redirect a /login o /projects segun sesion
│   │
│   ├── login/
│   │   └── page.tsx                # Server Component — si hay sesion, redirect /projects
│   │
│   ├── setup/
│   │   └── page.tsx                # Server Component — si ya hay tenant, redirect /login
│   │
│   ├── projects/
│   │   ├── page.tsx                # Server Component — lista proyectos
│   │   └── [slug]/
│   │       ├── layout.tsx          # Layout con sidebar de proyecto (Server Component)
│   │       ├── flags/
│   │       │   ├── page.tsx        # Server Component — lista flags
│   │       │   └── [key]/
│   │       │       └── page.tsx    # Server Component — rule editor
│   │       ├── segments/
│   │       │   └── page.tsx        # Server Component — lista + editor segmentos
│   │       ├── api-keys/
│   │       │   └── page.tsx        # Server Component — lista API keys
│   │       ├── audit/
│   │       │   └── page.tsx        # Server Component — audit log con filtros
│   │       └── settings/
│   │           └── page.tsx        # Server Component — settings del proyecto
│   │
│   ├── account/
│   │   └── page.tsx                # Server Component — cuenta del usuario
│   │
│   └── api/
│       └── auth/
│           ├── login/
│           │   └── route.ts        # Route Handler: proxya login, setea httpOnly cookie
│           ├── logout/
│           │   └── route.ts        # Route Handler: limpia cookie, llama backend logout
│           └── refresh/
│               └── route.ts        # Route Handler: refresca token, rota cookie
│
├── components/
│   ├── ui/                         # shadcn copies (no tocar salvo customizacion)
│   │   ├── button.tsx
│   │   ├── input.tsx
│   │   ├── badge.tsx
│   │   ├── table.tsx
│   │   ├── dialog.tsx
│   │   ├── switch.tsx
│   │   ├── select.tsx
│   │   ├── tabs.tsx
│   │   ├── label.tsx
│   │   ├── separator.tsx
│   │   ├── avatar.tsx
│   │   ├── dropdown-menu.tsx
│   │   ├── alert.tsx
│   │   └── skeleton.tsx
│   │
│   ├── layout/
│   │   ├── sidebar.tsx             # "use client" — nav activa, highlight segun ruta
│   │   ├── topbar.tsx              # Titulo de pagina + acciones (Server Component)
│   │   └── project-switcher.tsx    # "use client" — dropdown de proyectos
│   │
│   ├── login/
│   │   └── login-form.tsx          # "use client" — form con react-hook-form + zod
│   │
│   ├── projects/
│   │   ├── project-card.tsx        # Server Component — tarjeta de proyecto
│   │   └── create-project-dialog.tsx  # "use client" — dialog + form
│   │
│   ├── flags/
│   │   ├── flags-table.tsx         # Server Component — tabla de flags
│   │   ├── flag-row.tsx            # Server Component — fila de la tabla
│   │   ├── create-flag-dialog.tsx  # "use client" — dialog + form
│   │   ├── archive-flag-button.tsx # "use client" — boton con confirm
│   │   └── env-toggle.tsx          # "use client" — Switch enabled/disabled por env
│   │
│   ├── rules/
│   │   ├── rule-editor.tsx         # "use client" — editor de reglas completo
│   │   ├── rule-card.tsx           # "use client" — una regla con condiciones
│   │   ├── condition-row.tsx       # "use client" — una condicion attr/op/val
│   │   └── rollout-input.tsx       # "use client" — input numerico validado 0-100
│   │
│   ├── segments/
│   │   ├── segments-table.tsx
│   │   ├── segment-row.tsx
│   │   └── create-segment-dialog.tsx
│   │
│   ├── api-keys/
│   │   ├── api-keys-table.tsx
│   │   ├── create-key-dialog.tsx   # "use client" — form + modal de raw key
│   │   └── raw-key-modal.tsx       # "use client" — muestra la key UNA vez
│   │
│   ├── audit/
│   │   ├── audit-table.tsx
│   │   ├── audit-filters.tsx       # "use client" — filtros (actor, accion, fecha)
│   │   └── audit-row.tsx
│   │
│   ├── settings/
│   │   ├── project-settings-form.tsx   # "use client"
│   │   ├── environments-list.tsx
│   │   └── danger-zone.tsx         # "use client" — delete project con confirm
│   │
│   └── account/
│       ├── change-password-form.tsx    # "use client"
│       └── sessions-list.tsx
│
├── lib/
│   ├── api.ts                      # fetch wrapper: base URL, auth header, error handling
│   ├── auth.ts                     # leer/escribir/borrar cookie de sesion (server-side)
│   ├── schemas.ts                  # schemas zod para todos los forms
│   └── utils.ts                    # cn() (clsx + twMerge), formatDate, formatRelative
│
├── middleware.ts                   # proteccion de rutas, redirect /login si no hay sesion
│
└── __tests__/                      # tests (ver seccion M2-4)
    ├── unit/
    │   ├── lib/
    │   │   ├── schemas.test.ts
    │   │   └── utils.test.ts
    │   └── components/
    │       ├── login-form.test.tsx
    │       ├── rule-editor.test.tsx
    │       ├── condition-row.test.tsx
    │       └── rollout-input.test.tsx
    ├── integration/
    │   ├── login.test.tsx
    │   ├── flags-list.test.tsx
    │   ├── rule-editor.test.tsx
    │   └── api-keys.test.tsx
    └── e2e/
        ├── login.spec.ts
        ├── flags.spec.ts
        └── rule-editor.spec.ts
```

---

## M2-3. Convenciones de Codigo

### Server Components vs Client Components

**Regla:** todo es Server Component por defecto. Se agrega `"use client"` solo cuando el componente necesita:
- `useState`, `useReducer`, `useEffect`, `useRef`
- Event handlers (`onClick`, `onChange`, `onSubmit`)
- Browser APIs (`window`, `document`, `localStorage`)
- Hooks de react-hook-form

**Anti-patron:** marcar toda una pagina como `"use client"` porque un boton necesita estado. En su lugar, extraer el boton a un componente client separado y dejarlo como hoja del arbol.

```tsx
// MAL — toda la pagina como client
"use client"
export default function FlagsPage() { ... }

// BIEN — solo el componente interactivo es client
// flags/page.tsx (Server Component)
import { FlagsTable } from "@/components/flags/flags-table"
import { CreateFlagDialog } from "@/components/flags/create-flag-dialog" // "use client"
export default async function FlagsPage() {
  const flags = await getFlags(projectSlug)
  return (
    <>
      <FlagsTable flags={flags} />
      <CreateFlagDialog />
    </>
  )
}
```

### Naming

| Cosa | Convencion | Ejemplo |
|---|---|---|
| Archivos de componentes | kebab-case | `flag-row.tsx` |
| Componentes (funcion) | PascalCase | `FlagRow` |
| Hooks | camelCase con prefijo `use` | `useDebounce` |
| Funciones de lib | camelCase | `getFlags`, `formatDate` |
| Variables | camelCase | `flagKey`, `projectSlug` |
| Constantes | UPPER_SNAKE | `MAX_RULES_PER_FLAG` |
| Tipos/interfaces | PascalCase | `Flag`, `RuleCondition` |
| Archivos de tests | mismo nombre + `.test.tsx` | `flag-row.test.tsx` |

### Tipos

- Todos los tipos del dominio se definen en `lib/types.ts` y matchean exactamente con los modelos del backend Go.
- Sin `any`. Si el tipo no se conoce, usar `unknown` y narrowing explicito.
- Los responses del API se validan con zod en `lib/api.ts` antes de ser usados en el componente.

```ts
// lib/types.ts
export type Flag = {
  id: string
  projectId: string
  key: string
  name: string
  description: string | null
  type: "boolean" | "string" | "number" | "json"
  archivedAt: string | null
  createdAt: string
  updatedAt: string
}

export type FlagEnvironment = {
  flagId: string
  environmentId: string
  enabled: boolean
  rules: Rule[]
  version: number
}

export type Rule = {
  id: string
  conditions: Condition[]
  returnValue: boolean | string | number
  rollout: number  // 0-100
}

export type Condition = {
  attribute: string
  // Must match exactly the operator strings the Go engine accepts.
  // The engine treats unknown operators as false (silent) — typos here
  // would produce flags that never match without any error.
  operator:
    | "eq" | "neq"
    | "gt" | "gte" | "lt" | "lte"
    | "in" | "not_in"
    | "contains" | "starts_with" | "ends_with" | "matches"
    | "exists" | "not_exists"
    | "segment"
  value: string | number | boolean | string[]
}
```

### Manejo de errores

- Cada fetch en `lib/api.ts` puede lanzar un `ApiError` tipado con `code` y `message`.
- Los Server Components usan el archivo `error.tsx` de Next.js para capturar errores no manejados.
- Los Client Components muestran inline errors via react-hook-form o estado local.
- Los errores de red/timeout se muestran via `Alert` de shadcn/ui.
- **Nunca** se muestra un stack trace al usuario.

```ts
// lib/api.ts
export class ApiError extends Error {
  constructor(
    public readonly code: string,
    public readonly status: number,
    message: string,
  ) {
    super(message)
    this.name = "ApiError"
  }
}

export async function apiFetch<T>(
  path: string,
  options?: RequestInit,
): Promise<T> {
  const res = await fetch(`${BACKEND_URL}${path}`, {
    ...options,
    headers: {
      "Content-Type": "application/json",
      ...options?.headers,
    },
  })
  if (!res.ok) {
    const body = await res.json().catch(() => ({}))
    throw new ApiError(
      body?.error?.code ?? "UNKNOWN",
      res.status,
      body?.error?.message ?? res.statusText,
    )
  }
  return res.json() as Promise<T>
}
```

### Accesibilidad

- Todo componente interactivo tiene `aria-label` o texto visible accesible.
- El orden de focus es logico (no se usa `tabindex` positivo).
- Los modales (Dialog de shadcn/Radix) manejan focus trap y `aria-modal` automaticamente.
- Los colores cumplen WCAG AA (contrast ratio >= 4.5:1 para texto normal).
- Los botones de accion destructiva (archivar flag, revocar key) tienen doble confirmacion.

### Performance

- Imagenes con `next/image` (si las hubiera).
- Fuentes con `next/font` (sin layout shift).
- `loading.tsx` en cada segmento de ruta para mostrar Skeleton mientras carga.
- `Suspense` + `loading.tsx` en lugar de spinners manuales.
- Las tablas grandes usan paginacion del servidor (el backend ya lo soporta via `limit`/`offset`).

---

## M2-4. Estrategia de Testing

### Stack de testing

| Herramienta | Rol |
|---|---|
| Vitest | Test runner. Rapido, nativo ESM, API compatible con Jest. |
| @testing-library/react | Render de componentes en tests. Testea comportamiento, no implementacion. |
| @testing-library/user-event | Simula interacciones reales de usuario (tipo, click, tab, etc.). |
| @testing-library/jest-dom | Matchers adicionales: `toBeInTheDocument`, `toHaveValue`, etc. |
| msw (Mock Service Worker) | Intercepta fetch calls en tests para simular el backend sin levantarlo. |
| Playwright | Tests E2E contra el servidor Next.js real. |
| happy-dom | DOM virtual para Vitest (mas rapido que jsdom). |

### Piramide de tests

```
         /\
        /  \       E2E (Playwright)
       /    \      — flujos criticos completos
      /------\     — login, crear flag, toggle, API key flow
     /        \
    /          \   Integracion (Vitest + RTL + MSW)
   /            \  — componentes completos con datos mockeados
  /--------------\ — formularios, validaciones, respuestas de error del API
 /                \
/                  \ Unit (Vitest + RTL)
\------------------/ — funciones puras de lib/
                     — componentes simples sin logica de red
                     — schemas zod
                     — utils
```

### Tests unitarios

**Que se testea:**
- `lib/schemas.ts`: cada schema zod con casos validos e invalidos.
- `lib/utils.ts`: `formatDate`, `formatRelative`, `cn()`, etc.
- Componentes simples: `RolloutInput` (valida 0-100, rechaza letras), `ConditionRow` (renderiza attr/op/val correctamente).
- `LoginForm`: renderiza campos, muestra errores de validacion client-side, deshabilita boton durante submit.
- `RuleEditor`: agregar condicion, eliminar condicion, cambiar operador, toggle AND/OR.

**Convencion:**
```tsx
// __tests__/unit/components/rollout-input.test.tsx
import { render, screen } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { RolloutInput } from "@/components/rules/rollout-input"

describe("RolloutInput", () => {
  it("acepta valores entre 0 y 100", async () => {
    const onChange = vi.fn()
    render(<RolloutInput value={50} onChange={onChange} />)
    await userEvent.clear(screen.getByRole("spinbutton"))
    await userEvent.type(screen.getByRole("spinbutton"), "75")
    expect(onChange).toHaveBeenLastCalledWith(75)
  })

  it("rechaza valores > 100", async () => {
    render(<RolloutInput value={50} onChange={vi.fn()} />)
    await userEvent.clear(screen.getByRole("spinbutton"))
    await userEvent.type(screen.getByRole("spinbutton"), "150")
    expect(screen.getByRole("spinbutton")).toHaveAttribute("aria-invalid", "true")
  })
})
```

### Tests de integracion

**Que se testea:**
- Flujo completo de login: email/password validos → cookie seteado, redirect.
- Login con credenciales incorrectas → mensaje de error visible.
- Login con cuenta bloqueada → mensaje de retry.
- Flags list: renderiza la tabla con datos del MSW handler, muestra skeleton durante carga.
- CreateFlagDialog: submit con datos validos → flag aparece en la lista, dialog se cierra.
- ArchiveFlagButton: confirm dialog aparece, submit llama al API, flag desaparece de la lista activa.
- CreateApiKeyDialog: raw key se muestra exactamente una vez en el modal, no hay forma de volver a verla.
- RuleEditor: agregar regla, agregar condicion, cambiar valores, guardar → llamada al API con el payload correcto.

**MSW handlers:**
```ts
// __tests__/mocks/handlers.ts
import { http, HttpResponse } from "msw"

export const handlers = [
  http.post("/api/v1/auth/login", () =>
    HttpResponse.json({ access_token: "test-jwt", tenant: { slug: "acme" } }),
  ),
  http.get("/api/v1/projects", () =>
    HttpResponse.json({ projects: [{ id: "1", slug: "my-app", name: "My App" }] }),
  ),
  http.get("/api/v1/projects/:slug/flags", () =>
    HttpResponse.json({ flags: mockFlags }),
  ),
  // ... un handler por endpoint que la UI usa
]
```

### Tests E2E (Playwright)

**Que se testea (solo flujos criticos):**

| Test | Flujo |
|---|---|
| login-success | Ingresar email + password → redirect a /projects → titulo "Projects" visible |
| login-invalid | Email/password incorrectos → mensaje de error visible en pantalla |
| login-locked | 5 intentos fallidos → mensaje de cuenta bloqueada con countdown |
| create-flag | /projects/my-app/flags → click "New Flag" → form → submit → flag aparece en tabla |
| toggle-flag | Tabla de flags → click Switch de un flag → toast de confirmacion → estado persistido |
| archive-flag | Tabla de flags → click archivo → confirm dialog → flag desaparece → aparece en archived |
| create-api-key | /api-keys → click "New key" → form → submit → raw key visible en modal → cerrar → ya no visible |
| rule-editor | Abrir flag → tab Rules → agregar condicion → cambiar rollout a 50 → Save → recargar → regla persiste |
| setup-flow | GET /setup (primer run) → form → submit → redirect /login |

**Configuracion Playwright:**
```ts
// playwright.config.ts
export default defineConfig({
  testDir: "./__tests__/e2e",
  use: {
    baseURL: "http://localhost:3000",
    trace: "on-first-retry",
    screenshot: "only-on-failure",
  },
  webServer: {
    command: "npm run dev",
    url: "http://localhost:3000",
    reuseExistingServer: !process.env.CI,
  },
})
```

### Cobertura

- **Meta:** >= 80% de cobertura en `lib/` y en componentes con logica no trivial.
- **No se persigue cobertura en:** componentes puramente visuales sin logica, layouts, paginas que son solo composicion de otros componentes.
- `vitest --coverage` genera el reporte. En CI (cuando se active Fase 9), falla el build si cae de 80%.

### Comandos

```bash
npm run test          # vitest (watch mode en dev)
npm run test:run      # vitest --run (single pass, para CI)
npm run test:coverage # vitest --coverage
npm run test:e2e      # playwright test
npm run test:e2e:ui   # playwright test --ui (modo visual)
```

---

## M2-5. Buenas Practicas Globales

### Seguridad

- El JWT **nunca** toca `localStorage` ni `document.cookie` desde JS del cliente. Solo viaja en httpOnly cookie.
- El cookie tiene `SameSite=Lax` + `Secure` en produccion + `HttpOnly`. Inmune a XSS.
- Las Route Handlers que proxyan al backend validan que el cookie exista antes de hacer el request.
- El raw value de una API key se muestra en el `RawKeyModal` exactamente una vez. El componente no persiste el valor en estado despues de cerrado: la key vive en el `useState` de `CreateKeyDialog` y se limpia a `null` en el `onOpenChange` del Dialog (cuando se cierra). React destruye el estado al desmontar, pero limpiar explicitamente garantiza que si el usuario abre el dialog por segunda vez (sin hacer un nuevo POST) no vea la key anterior.
- Sin `dangerouslySetInnerHTML` en ningun componente.
- El contenido de inputs del usuario nunca se concatena en queries o URLs directamente — siempre via parametros tipados.

### Acciones destructivas

Toda accion irreversible (archivar flag, revocar API key, eliminar proyecto, eliminar segmento) requiere:
1. Un boton primario que abre un confirm dialog.
2. El dialog explica exactamente que va a pasar.
3. Un boton secundario de confirmacion (rojo) dentro del dialog.
4. El boton de cancelar tiene el foco por defecto (anti-fat-finger).

### Loading states

- Cada Server Component tiene su `loading.tsx` sibling con Skeletons que replican el layout de la pantalla cargada.
- Los botones de submit muestran un spinner y se deshabilitan mientras el request esta en vuelo (`useFormStatus` o `isPending` de `useActionState`).
- Las tablas vacías tienen un estado empty explícito con mensaje y CTA (no tabla con 0 filas).

### Error states

- Si el fetch del Server Component falla, el `error.tsx` sibling muestra un mensaje amigable con boton "Try again" que llama `reset()`.
- Los errores de validacion del API (400) se muestran en el campo correspondiente del form.
- Los errores de autorizacion (401) redirigen a `/login` via middleware.
- Los errores 403 muestran una pagina "You don't have permission" con link a la pantalla anterior.
- Los errores 500 muestran un mensaje generico (sin stack trace).

### Linting y formato

```json
// package.json scripts
{
  "lint": "next lint",
  "lint:fix": "next lint --fix",
  "format": "prettier --write .",
  "format:check": "prettier --check .",
  "typecheck": "tsc --noEmit"
}
```

Configuracion de linting:
- `eslint-config-next` como base.
- Regla `no-restricted-imports` para prohibir `import React from "react"` (no necesario en React 19).
- Regla personalizada: prohibir `"use client"` en archivos de `app/*/page.tsx` directamente — las paginas deben ser Server Components. Si se necesita client, extraer componente.
- `prettier` con `printWidth: 100`, `semi: false`, `singleQuote: false`.

### Git workflow

- Cada fase W tiene su propio branch: `feat/w0-scaffold`, `feat/w1-login`, etc.
- Commits atomicos por componente o feature. Mensaje: `feat(w1): login form con validacion zod`.
- PR por fase. No se mergea sin que todos los tests pasen.
- Los archivos generados por shadcn (`components/ui/`) se commitean (son parte del codigo del proyecto).

---

## M2-6. Fase W0 — Scaffolding

**Objetivo:** proyecto Next.js funcional, Tailwind configurado, shadcn/ui inicializado, tests corriendo, proxy al backend funcionando.

### Comandos de inicializacion

```bash
cd web/
npx create-next-app@16.2.6 . \
  --typescript \
  --tailwind \
  --eslint \
  --app \
  --src-dir=no \
  --import-alias="@/*" \
  --use-npm

# Instalar dependencias de produccion
npm install react-hook-form zod lucide-react clsx tailwind-merge

# Instalar dependencias de desarrollo
npm install -D vitest @vitejs/plugin-react @testing-library/react \
  @testing-library/user-event @testing-library/jest-dom \
  msw happy-dom @playwright/test

# Inicializar shadcn/ui
npx shadcn@latest init
# Responder: style=default, baseColor=slate, cssVariables=yes

# Agregar componentes shadcn que se van a necesitar
npx shadcn@latest add button input label badge table dialog \
  switch select tabs textarea separator avatar dropdown-menu alert skeleton
```

### Archivos a crear/modificar en W0

**`next.config.ts`:**
```ts
import type { NextConfig } from "next"

const nextConfig: NextConfig = {
  output: "standalone",
  async rewrites() {
    return [
      {
        source: "/api/v1/:path*",
        destination: `${process.env.BACKEND_URL ?? "http://localhost:8080"}/api/v1/:path*`,
      },
    ]
  },
}

export default nextConfig
```

**`middleware.ts`:**
```ts
import { NextResponse } from "next/server"
import type { NextRequest } from "next/server"

const PUBLIC_PATHS = ["/login", "/setup", "/api/auth"]

export function middleware(request: NextRequest) {
  const { pathname } = request.nextUrl
  const isPublic = PUBLIC_PATHS.some((p) => pathname.startsWith(p))
  const token = request.cookies.get("access_token")?.value

  if (!isPublic && !token) {
    return NextResponse.redirect(new URL("/login", request.url))
  }
  if (isPublic && token && !pathname.startsWith("/api/auth")) {
    return NextResponse.redirect(new URL("/projects", request.url))
  }
  return NextResponse.next()
}

export const config = {
  matcher: ["/((?!_next/static|_next/image|favicon.ico).*)"],
}
```

**`vitest.config.ts`:**
```ts
import { defineConfig } from "vitest/config"
import react from "@vitejs/plugin-react"
import path from "path"

export default defineConfig({
  plugins: [react()],
  test: {
    environment: "happy-dom",
    globals: true,
    setupFiles: ["__tests__/setup.ts"],
    coverage: {
      provider: "v8",
      threshold: { lines: 80, functions: 80, branches: 80 },
      exclude: ["components/ui/**", "**/*.config.*", "__tests__/**"],
    },
  },
  resolve: {
    alias: { "@": path.resolve(__dirname, ".") },
  },
})
```

**`__tests__/setup.ts`:**
```ts
import "@testing-library/jest-dom"
import { server } from "./mocks/server"

beforeAll(() => server.listen({ onUnhandledRequest: "error" }))
afterEach(() => server.resetHandlers())
afterAll(() => server.close())
```

**`lib/utils.ts`:**
```ts
import { clsx, type ClassValue } from "clsx"
import { twMerge } from "tailwind-merge"

export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs))
}

export function formatDate(iso: string): string {
  return new Intl.DateTimeFormat("en-US", {
    year: "numeric", month: "short", day: "numeric",
  }).format(new Date(iso))
}

export function formatRelative(iso: string): string {
  const diff = Date.now() - new Date(iso).getTime()
  const minutes = Math.floor(diff / 60_000)
  if (minutes < 1) return "just now"
  if (minutes < 60) return `${minutes}m ago`
  const hours = Math.floor(minutes / 60)
  if (hours < 24) return `${hours}h ago`
  return `${Math.floor(hours / 24)}d ago`
}
```

### Verificaciones de W0

- `npm run dev` arranca en `:3000` sin errores.
- `npm run build` completa sin errores de TypeScript.
- `npm run lint` pasa sin warnings.
- `npm run typecheck` pasa.
- `npm run test:run` pasa (aunque no haya tests aun, setup no crashea).
- `curl http://localhost:3000/login` devuelve HTML (la pagina de login, aunque este vacia).
- `curl http://localhost:3000/projects` redirige a `/login` (middleware activo).
- `curl http://localhost:3000/api/v1/healthz` (con backend corriendo) devuelve `{"status":"ok"}` — proxy funciona.

### Checklist W0

- [x] `npm create next-app@16.2.6` con flags correctos
- [x] `next.config.ts` con `output: "standalone"` y `rewrites()` al backend
- [x] `proxy.ts` con proteccion de rutas y redirect logico (Next.js 16 renombró `middleware.ts` → `proxy.ts` y la función `middleware` → `proxy`)
- [x] Color primario `#364fc7` en CSS custom properties (`globals.css` via `@theme` block — Tailwind v4 no requiere `tailwind.config.ts` para tokens)
- [x] `components.json` de shadcn configurado
- [x] Todos los componentes shadcn instalados (button, input, label, badge, table, dialog, switch, select, tabs, textarea, separator, avatar, dropdown-menu, alert, skeleton)
- [x] `lib/utils.ts`: `cn()`, `formatDate()`, `formatRelative()`
- [x] `lib/types.ts`: tipos del dominio (Flag, Rule, Condition, Project, Environment, APIKey, AuditEntry, Segment)
- [x] `lib/api.ts`: `apiFetch<T>()`, `ApiError`, funciones por recurso
- [x] `lib/schemas.ts`: schemas zod para todos los forms (login, create-project, create-flag, create-segment, create-api-key)
- [x] `vitest.config.ts` con happy-dom, paths alias, coverage thresholds
- [x] `playwright.config.ts` con webServer configurado
- [x] `__tests__/setup.ts` con MSW setup
- [x] `__tests__/mocks/handlers.ts` con handlers para todos los endpoints usados
- [x] `__tests__/mocks/server.ts` con MSW node server
- [x] Tests unitarios de `lib/utils.ts` (formatDate, formatRelative, cn)
- [x] Tests unitarios de `lib/schemas.ts` (casos validos e invalidos por schema)
- [x] `npm run test:run` pasa
- [x] `npm run build` pasa
- [x] `npm run typecheck` pasa
- [x] `Dockerfile` en `web/` funciona
- [x] Actualizar `docker-compose.yml` raiz para incluir servicio `web` (+ servicio `api`)

---

## M2-7. Fase W1 — Login

**Objetivo:** pantalla de login funcional con validacion, manejo de errores del API y redirect post-login. Cubre la pantalla 1 del wireframe.

### Componentes

**`app/login/page.tsx`** (Server Component):
- Lee cookie de sesion. Si existe, redirect a `/projects`.
- Renderiza `LoginForm`.

**`components/login/login-form.tsx`** (`"use client"`):
- Form con `react-hook-form` + schema zod (`loginSchema`).
- Campos: `email` (required, email format), `password` (required, min 8 chars).
- Submit llama a `POST /api/auth/login` (Route Handler local, no directamente al backend).
- Maneja: credenciales incorrectas (401) → mensaje de error inline, cuenta bloqueada (423) → mensaje con tiempo de espera, error de red → mensaje generico.
- Boton "Sign in" con spinner durante submit (`useFormStatus` o `isPending`).
- Link "Forgot password?" (MVP: placeholder, no implementado).
- Loading state: boton deshabilitado, campos deshabilitados.

**`app/api/auth/login/route.ts`** (Route Handler):
- Recibe `{ email, password, tenant_slug? }`.
- Hace POST al backend Go `/api/v1/auth/login`.
- Si 200: setea httpOnly cookie `access_token`, retorna `{ ok: true, redirectTo: "/projects" }`.
- Si 409 MULTIPLE_TENANTS: retorna los tenants disponibles para que el form pida elegir.
- Si 4xx: retorna el error para que el form lo muestre.

**`loginSchema`** (en `lib/schemas.ts`):
```ts
export const loginSchema = z.object({
  email: z.string().email("Must be a valid email"),
  password: z.string().min(8, "Password must be at least 8 characters"),
  tenant_slug: z.string().optional(),
})
```

### Tests W1

**Unitarios:**
- `LoginForm` renderiza los dos campos y el boton.
- Submit con campos vacios muestra errores de validacion (sin llamar al API).
- Submit con email invalido muestra error de formato.
- Submit con password < 8 chars muestra error.

**Integracion:**
- Login exitoso → cookie seteado, redirect a `/projects`.
- Login con credenciales incorrectas → mensaje "Invalid email or password" visible.
- Login con cuenta bloqueada → mensaje con retry time visible.
- Login con multiple tenants → step de selector de tenant aparece.
- Boton deshabilitado durante submit (simular request lento con MSW `delay`).

**E2E (Playwright):**
- `login-success.spec.ts`: flujo completo con backend real (o MSW en E2E mode).
- `login-invalid.spec.ts`: mensaje de error visible.

### Checklist W1

- [x] `app/login/page.tsx` (Server Component, redirect si sesion existe)
- [x] `app/login/loading.tsx` (Skeleton del form)
- [x] `app/login/error.tsx` (error boundary)
- [x] `components/login/login-form.tsx` con react-hook-form + zod
- [x] `app/api/auth/login/route.ts` (setea httpOnly cookie)
- [x] `app/api/auth/logout/route.ts` (limpia cookie)
- [x] `app/api/auth/refresh/route.ts` (rota cookie)
- [x] `loginSchema` en `lib/schemas.ts`
- [x] Tests unitarios: LoginForm (renderiza, validacion client-side)
- [x] Tests integracion: login exitoso, credenciales incorrectas, cuenta bloqueada, multiple tenants
- [x] Tests E2E: login-success, login-invalid
- [x] `npm run test:run` pasa
- [x] `npm run typecheck` pasa

---

## M2-8. Fase W2 — Projects List

**Objetivo:** pantalla de lista de proyectos con sidebar, crear proyecto. Cubre la pantalla 2 del wireframe.

### Componentes clave

- `app/projects/page.tsx`: Server Component, fetchea `GET /api/v1/projects`, renderiza `ProjectCard` x N + `CreateProjectDialog`.
- `components/projects/project-card.tsx`: Server Component. Muestra nombre, slug, N environments, N flags, fecha creacion.
- `components/projects/create-project-dialog.tsx`: Client Component. Form con `name` (required) y `slug` (auto-generado desde name, editable). Submit llama `POST /api/v1/projects`, luego `router.refresh()`.
- `components/layout/sidebar.tsx`: Client Component (necesita saber la ruta activa). Muestra logo, nav items, user email en el fondo. En pantallas de proyecto: muestra el proyecto activo con nav de Flags, Segments, API Keys, Audit, Settings.

**`createProjectSchema`:**
```ts
export const createProjectSchema = z.object({
  name: z.string().min(1).max(100),
  slug: z.string().min(1).max(50).regex(/^[a-z0-9-]+$/, "Only lowercase letters, numbers, hyphens"),
})
```

### Tests W2

**Unitarios:**
- `ProjectCard` renderiza nombre, slug, N environments, N flags.
- `createProjectSchema` acepta slugs validos, rechaza con mayusculas, rechaza con espacios.

**Integracion:**
- Lista de proyectos se renderiza con datos del MSW handler.
- Estado vacio: mensaje "No projects yet" con CTA visible.
- `CreateProjectDialog` abre al hacer click en "New project".
- Submit de form valido → proyecto aparece en la lista, dialog se cierra.
- Submit de form con slug duplicado → error inline del API visible.

**E2E:**
- Proyecto creado persiste tras reload de pagina.

### Checklist W2

- [x] `app/projects/page.tsx`
- [x] `app/projects/loading.tsx` (Skeletons de cards)
- [x] `app/projects/error.tsx`
- [x] `components/projects/project-card.tsx`
- [x] `components/projects/create-project-dialog.tsx`
- [x] `components/layout/sidebar.tsx` (nav global)
- [x] `components/layout/topbar.tsx`
- [x] `createProjectSchema` en `lib/schemas.ts`
- [x] Funciones en `lib/api.ts`: `getProjects()`, `createProject()`
- [x] Tests unitarios: ProjectCard, createProjectSchema
- [x] Tests integracion: lista, empty state, crear proyecto, slug duplicado
- [x] `npm run test:run` pasa

---

## M2-9. Fase W3 — Flags List

**Objetivo:** lista de flags con search, toggle enabled/disabled por entorno, archivar. Cubre la pantalla 3 del wireframe.

### Componentes clave

- `app/projects/[slug]/flags/page.tsx`: Server Component. Fetchea flags + environments del proyecto. Renderiza tabla.
- `components/flags/flags-table.tsx`: Server Component. Tabla con columnas: key, name, type, enabled (por env), last updated, actions.
- `components/flags/env-toggle.tsx`: Client Component. Switch que llama `PATCH /api/v1/projects/:slug/flags/:key/environments/:env` para toggle enabled. `router.refresh()` tras exito.
- `components/flags/create-flag-dialog.tsx`: Client Component. Fields: key (slug), name, type (boolean/string/number/json), description (opcional).
- `components/flags/archive-flag-button.tsx`: Client Component. Boton con confirm dialog. Llama `DELETE /api/v1/projects/:slug/flags/:key` (soft delete = archivedAt).

**`createFlagSchema`:**
```ts
export const createFlagSchema = z.object({
  key: z.string().min(1).max(100).regex(/^[a-z0-9-_]+$/, "Lowercase, numbers, hyphens, underscores"),
  name: z.string().min(1).max(200),
  type: z.enum(["boolean", "string", "number", "json"]),
  description: z.string().max(500).optional(),
})
```

### Tests W3

**Unitarios:**
- `FlagRow` renderiza key, name, type badge, enabled switch, fecha.
- `createFlagSchema` valida tipos correctamente, rechaza keys con espacios.
- `EnvToggle` llama `onChange` con el valor correcto.

**Integracion:**
- Tabla renderiza 5 flags del mock.
- `EnvToggle` deshabilita durante el request, re-habilita tras exito.
- `EnvToggle` muestra error inline si el API falla.
- `ArchiveFlagButton` abre confirm dialog, flag desaparece tras confirmar.
- `CreateFlagDialog` valida duplicados (la API retorna 409).

**E2E:**
- `flags.spec.ts`: toggle-flag persiste tras reload. create-flag aparece en lista.

### Checklist W3

- [x] `app/projects/[slug]/flags/page.tsx`
- [x] `app/projects/[slug]/flags/loading.tsx`
- [x] `app/projects/[slug]/layout.tsx` (sidebar con nav de proyecto)
- [x] `components/flags/flags-table.tsx`
- [x] `components/flags/flag-row.tsx`
- [x] `components/flags/env-toggle.tsx` (refactored con `useOptimistic` + `useTransition`)
- [x] `components/flags/create-flag-dialog.tsx`
- [x] `components/flags/archive-flag-button.tsx`
- [x] `createFlagSchema` en `lib/schemas.ts`
- [x] Funciones en `lib/api.ts`: `getFlags()`, `createFlag()`, `archiveFlag()`, `toggleFlagEnv()`
- [x] Tests unitarios: FlagRow, createFlagSchema, EnvToggle
- [x] Tests integracion: tabla, toggle, archive, create, 409
- [x] Tests E2E: toggle-flag, create-flag
- [x] `npm run test:run` pasa

---

## M2-10. Fase W4 — Project Settings

**Objetivo:** settings del proyecto: rename, gestionar environments, zona de peligro (delete). Cubre la pantalla de Project Settings del wireframe.

### Componentes clave

- `app/projects/[slug]/settings/page.tsx`: Server Component.
- `components/settings/project-settings-form.tsx`: Client. Edita name del proyecto. `PATCH /api/v1/projects/:slug`.
- `components/settings/environments-list.tsx`: Client. Lista envs, permite crear nuevo env (`POST /api/v1/projects/:slug/environments`) y eliminar (con confirm).
- `components/settings/danger-zone.tsx`: Client. Delete project con confirm + tipear el slug del proyecto para confirmar. Llama `DELETE /api/v1/projects/:slug`. Redirect a `/projects` tras exito.

### Tests W4

**Unitarios:**
- `DangerZone` no habilita el boton confirm hasta que se tipee el slug correcto.

**Integracion:**
- `ProjectSettingsForm` muestra el nombre actual, permite editarlo, submit guarda.
- `EnvironmentsList` renderiza los envs, crear nuevo env aparece en la lista.
- `DangerZone` delete con slug incorrecto → boton sigue deshabilitado.
- `DangerZone` delete con slug correcto → redirect a /projects.

### Checklist W4

- [x] `app/projects/[slug]/settings/page.tsx`
- [x] `components/settings/project-settings-form.tsx`
- [x] `components/settings/environments-list.tsx`
- [x] `components/settings/danger-zone.tsx`
- [x] Funciones en `lib/api.ts`: `updateProject()`, `deleteProject()`, `getEnvironments()`, `createEnvironment()`, `deleteEnvironment()`
- [x] Tests unitarios: DangerZone (slug validation)
- [x] Tests integracion: rename, create env, delete project
- [x] `npm run test:run` pasa

---

## M2-11. Fase W5 — API Keys

**Objetivo:** gestionar API keys por entorno. Raw value una sola vez. Cubre la pantalla de API Keys del wireframe.

### Componentes clave

- `app/projects/[slug]/api-keys/page.tsx`: Server Component. Fetchea keys por proyecto (sin raw value).
- `components/api-keys/api-keys-table.tsx`: Server Component. Columnas: nombre, prefix (`fs_live_xxxx…`), env, creado, ultimo uso, expira, acciones.
- `components/api-keys/create-key-dialog.tsx`: Client. Form: nombre, environment (select), expiry (optional). Submit llama `POST /api/v1/projects/:slug/environments/:env/api-keys`. La respuesta incluye el raw key **una sola vez**. Si la response tiene `raw_key`, abre `RawKeyModal` automaticamente.
- `components/api-keys/raw-key-modal.tsx`: Client. Muestra la key completa con boton de copiar. Al cerrar, el componente elimina la key de su estado local. No hay forma de volver a verla desde la UI. Advertencia explícita: "This key will not be shown again."
- `components/api-keys/revoke-key-button.tsx`: Client. Confirm dialog + `DELETE /api/v1/projects/:slug/environments/:env/api-keys/:id`.

**`createApiKeySchema`:**
```ts
export const createApiKeySchema = z.object({
  name: z.string().min(1).max(100),
  environment_id: z.string().uuid(),
  expires_at: z.string().datetime().optional(),
})
```

### Tests W5

**Unitarios:**
- `RawKeyModal` muestra la key en un input readonly.
- `RawKeyModal` boton de copiar llama `navigator.clipboard.writeText`.
- `RawKeyModal` al cerrar, la key no esta en el DOM.
- `createApiKeySchema` valida UUID del environment, acepta expires_at opcional.

**Integracion:**
- `CreateKeyDialog` submit → API call → `RawKeyModal` se abre con la key.
- Cerrar `RawKeyModal` → la key ya no esta en el DOM (no hay forma de recuperarla).
- Abrir `CreateKeyDialog` por segunda vez (sin crear key) → el modal no muestra la key de la creacion anterior (estado limpiado en `onOpenChange`).
- `RevokeKeyButton` confirm → key desaparece de la tabla.
- Tabla muestra "No API keys" cuando no hay keys.

**E2E:**
- `create-api-key.spec.ts`: crear key → key visible en modal → cerrar → key no visible en tabla (solo prefix).

### Checklist W5

- [x] `app/projects/[slug]/api-keys/page.tsx`
- [x] `components/api-keys/api-keys-table.tsx`
- [x] `components/api-keys/create-key-dialog.tsx`
- [x] `components/api-keys/raw-key-modal.tsx`
- [x] `components/api-keys/revoke-key-button.tsx`
- [x] `createApiKeySchema` en `lib/schemas.ts`
- [x] Funciones en `lib/api.ts`: `getApiKeys()`, `createApiKey()`, `revokeApiKey()`
- [x] Tests unitarios: RawKeyModal (muestra, copia, limpia al cerrar), createApiKeySchema
- [x] Tests integracion: crear key, raw key modal, revocar key, estado vacio
- [x] Tests E2E: create-api-key flujo completo (`__tests__/e2e/api-keys.spec.ts`)
- [x] `npm run test:run` pasa

---

## M2-12. Fase W6 — Audit Log

**Objetivo:** tabla de audit log con filtros. Cubre la pantalla de Audit del wireframe.

### Componentes clave

- `app/projects/[slug]/audit/page.tsx`: Server Component. Lee filtros de `searchParams` (actor, action, desde, hasta). Fetchea `GET /api/v1/audit?...`. Renderiza tabla + filtros.
- `components/audit/audit-filters.tsx`: Client. Controles de filtro con URL state (actualiza `searchParams` al cambiar). Inputs: actor (email), action (select con enum), date range.
- `components/audit/audit-table.tsx`: Server Component. Paginacion del servidor.
- `components/audit/audit-row.tsx`: Server Component. Icono por tipo de accion, actor, recurso, timestamp relativo + absoluto en hover (tooltip).

### Tests W6

**Unitarios:**
- `formatRelative` retorna "just now", "5m ago", "2h ago", "3d ago" correctamente.
- `AuditRow` renderiza el actor y la accion.

**Integracion:**
- Tabla renderiza entradas del mock.
- Filtro por action filtra las entradas visibles.
- Paginacion: "Load more" carga la siguiente pagina.

### Checklist W6

- [x] `app/projects/[slug]/audit/page.tsx`
- [x] `components/audit/audit-filters.tsx`
- [x] `components/audit/audit-table.tsx`
- [x] `components/audit/audit-row.tsx`
- [x] Funciones en `lib/api.ts`: `getAuditLog(filters)`
- [x] Tests unitarios: formatRelative, AuditRow
- [x] Tests integracion: tabla, filtros, paginacion
- [x] `npm run test:run` pasa

---

## M2-13. Fase W7 — Account

**Objetivo:** configuracion de cuenta del usuario: cambiar password, ver sesiones activas. Cubre la pantalla Account del wireframe.

### Componentes clave

- `app/account/page.tsx`: Server Component. Fetchea datos del usuario actual + sesiones activas.
- `components/account/change-password-form.tsx`: Client. Fields: current password, new password, confirm new password. Validacion local: new === confirm, new >= 8 chars. Submit `PATCH /api/v1/auth/me/password`.
- `components/account/sessions-list.tsx`: Client. Lista de sesiones activas con device/IP/fecha. Boton "Revoke" por sesion. Boton "Revoke all other sessions".

**`changePasswordSchema`:**
```ts
export const changePasswordSchema = z.object({
  current_password: z.string().min(1),
  new_password: z.string().min(8),
  confirm_password: z.string(),
}).refine((d) => d.new_password === d.confirm_password, {
  message: "Passwords do not match",
  path: ["confirm_password"],
})
```

### Tests W7

**Unitarios:**
- `changePasswordSchema` rechaza cuando new !== confirm.
- `changePasswordSchema` rechaza new < 8 chars.
- `changePasswordSchema` acepta caso valido.

**Integracion:**
- `ChangePasswordForm` valida antes de submit (sin llamar al API).
- Submit exitoso → mensaje de confirmacion visible.
- `SessionsList` muestra sesiones. Revocar una sesion la elimina de la lista.

### Checklist W7

- [x] `app/account/page.tsx`
- [x] `components/account/change-password-form.tsx`
- [x] `components/account/sessions-list.tsx` (fix: handleRevokeAll preserva sesion actual)
- [x] `changePasswordSchema` en `lib/schemas.ts`
- [x] Funciones en `lib/api.ts`: `getMe()`, `changePassword()`, `getSessions()`, `revokeSession()`
- [x] Tests unitarios: changePasswordSchema
- [x] Tests integracion: cambiar password, revocar sesion
- [x] `npm run test:run` pasa

---

## M2-14. Fase W8 — Setup

**Objetivo:** pantalla de primer run (setup inicial). Solo accesible si no existe ningun tenant. Cubre la pantalla Setup del wireframe.

### Componentes clave

- `app/setup/page.tsx`: Server Component. Llama `GET /api/v1/setup/status`. Si ya hay tenant, redirect a `/login`. Si no, renderiza `SetupForm`.
- `components/setup/setup-form.tsx`: Client. Fields: tenant name, tenant slug, admin email, admin password, confirm password. Submit `POST /api/v1/setup`. Redirect a `/login` con mensaje "Setup complete. Please sign in."

**`setupSchema`:**
```ts
export const setupSchema = z.object({
  tenant_name: z.string().min(1).max(100),
  tenant_slug: z.string().min(1).max(50).regex(/^[a-z0-9-]+$/),
  admin_email: z.string().email(),
  admin_password: z.string().min(8),
  confirm_password: z.string(),
}).refine((d) => d.admin_password === d.confirm_password, {
  message: "Passwords do not match",
  path: ["confirm_password"],
})
```

### Tests W8

**Unitarios:**
- `setupSchema` valida todos los campos.
- `setupSchema` rechaza cuando passwords no coinciden.

**Integracion:**
- `SetupForm` muestra todos los campos, valida antes de submit.
- Submit exitoso → redirect a `/login`.
- Si el backend retorna 409 (ya existe tenant) → mensaje de error.

**E2E:**
- `setup-flow.spec.ts`: flujo completo desde `/setup` hasta `/login`.

### Checklist W8

- [x] `app/setup/page.tsx` (Server Component, importa SetupForm)
- [x] `components/setup/setup-form.tsx`
- [x] `setupSchema` en `lib/schemas.ts`
- [x] Funciones en `lib/api.ts`: `getSetupStatus()`, `runSetup()`
- [x] Tests unitarios: setupSchema
- [x] Tests integracion: form valido, passwords no coinciden, tenant ya existe
- [x] Tests E2E: setup-flow (`__tests__/e2e/setup.spec.ts`)
- [x] `npm run test:run` pasa

---

## M2-15. Fase W9 — Rule Editor

**Objetivo:** editor de reglas de un flag por entorno. El mas complejo del dashboard. Cubre la pantalla de Flag Rule Editor del wireframe.

### Componentes clave

- `app/projects/[slug]/flags/[key]/page.tsx`: Server Component. Fetchea flag + environments + flag_environments (rules por env). Renderiza `RuleEditor` con los datos del entorno seleccionado.
- `components/rules/rule-editor.tsx`: Client Component completo. Estado: lista de reglas, entorno seleccionado, enabled toggle. Botones: "Add rule", "Save". Submit hace `PUT /api/v1/projects/:slug/flags/:key/environments/:env/rules` con optimistic update.
- `components/rules/rule-card.tsx`: Client. Una regla con badge "Rule N", boton eliminar (✕), lista de condiciones, seccion de return value + rollout.
- `components/rules/condition-row.tsx`: Client. Tres selects/inputs en linea: attribute (free text), operator (select: eq, neq, gt, gte, lt, lte, contains, in, nin), value (free text, o lista para `in`/`nin`). Boton AND toggle: si hay multiples condiciones, muestra "AND" entre ellas (clickable para toggle a OR — aunque el MVP solo soporta AND).
- `components/rules/rollout-input.tsx`: Client. Input numerico 0-100 con sufijo "%". Validacion: integer, 0-100. Muestra error si fuera de rango.

**Logica de estado del RuleEditor:**
```ts
type RuleState = {
  rules: Rule[]
  enabled: boolean
  selectedEnv: string
  isDirty: boolean  // hay cambios sin guardar
}
```

Al cambiar de entorno (selector), si `isDirty`, muestra confirm dialog "You have unsaved changes. Discard them?".

Al hacer "Save":
1. Validar que cada regla tenga al menos una condicion.
2. Validar que todos los rollouts sumen <= 100 (warning si > 100, error si una regla sola > 100).
3. POST/PUT al API.
4. Si exito: `isDirty = false`, toast de confirmacion.
5. Si 409 (version conflict — OCC del backend): mostrar "This flag was modified by someone else. Reload to see latest version."

**`ruleSchema`:**
```ts
const conditionSchema = z.object({
  attribute: z.string().min(1),
  operator: z.enum(["eq", "neq", "gt", "gte", "lt", "lte", "contains", "in", "nin"]),
  value: z.union([z.string(), z.number(), z.boolean(), z.array(z.string())]),
})

const ruleSchema = z.object({
  id: z.string(),
  conditions: z.array(conditionSchema).min(1, "At least one condition required"),
  returnValue: z.union([z.boolean(), z.string(), z.number()]),
  rollout: z.number().int().min(0).max(100),
})

export const rulesPayloadSchema = z.array(ruleSchema)
```

### Tests W9

**Unitarios:**
- `RolloutInput` acepta 0, 50, 100. Rechaza 101, -1, letras.
- `ConditionRow` renderiza los tres campos, llama onChange con el valor correcto.
- `conditionSchema` valida todos los operadores.
- `ruleSchema` rechaza reglas sin condiciones.
- `rulesPayloadSchema` valida el payload completo.

**Integracion:**
- `RuleEditor` renderiza la regla existente del mock.
- Agregar condicion: click "Add condition" → nueva fila de condicion aparece.
- Eliminar condicion: click ✕ → fila desaparece.
- Agregar regla: click "Add rule" → nuevo `RuleCard` aparece.
- Cambiar rollout a valor invalido → error visible, boton Save deshabilitado.
- Save exitoso → `isDirty = false`, toast visible.
- Save con version conflict (409 mock) → mensaje de conflict visible.
- Cambiar env con cambios sin guardar → confirm dialog aparece.

**E2E:**
- `rule-editor.spec.ts`: flujo completo — abrir flag → cambiar rollout → save → reload → valor persiste.

### Checklist W9

- [x] `app/projects/[slug]/flags/[key]/page.tsx`
- [x] `components/rules/rule-editor.tsx` (fix: eliminado `showEnvConfirm` redundante, `pendingEnv !== null` es suficiente)
- [x] `components/rules/rule-card.tsx`
- [x] `components/rules/condition-row.tsx`
- [x] `components/rules/rollout-input.tsx`
- [x] `ruleSchema`, `conditionSchema`, `rulesPayloadSchema` en `lib/schemas.ts`
- [x] Funciones en `lib/api.ts`: `getFlagDetail()`, `getFlagEnvironment()`, `saveRules()`
- [x] Tests unitarios: RolloutInput, ConditionRow, ruleSchema, rulesPayloadSchema
- [x] Tests integracion: render, agregar/eliminar condicion, agregar/eliminar regla, save exitoso, 409 conflict, cambiar env con dirty state
- [x] Tests E2E: rule-editor flujo completo
- [x] `npm run test:run` pasa

---

## M2-16. Fase W10 — Segments

**Objetivo:** lista de segmentos y editor de reglas de segmento. Similar al rule editor pero para segmentos. Cubre la pantalla de Segments del wireframe.

### Componentes clave

- `app/projects/[slug]/segments/page.tsx`: Server Component. Lista de segmentos + editor inline del seleccionado.
- `components/segments/segments-table.tsx`: Server Component.
- `components/segments/create-segment-dialog.tsx`: Client. Fields: key, name, description.
- `components/segments/segment-rule-editor.tsx`: Client. Similar al `RuleEditor` pero para segmentos (las reglas de un segmento definen que usuarios pertenecen a el). Sin rollout (los segmentos son todo o nada).

**`createSegmentSchema`:**
```ts
export const createSegmentSchema = z.object({
  key: z.string().min(1).max(100).regex(/^[a-z0-9-_]+$/),
  name: z.string().min(1).max(200),
  description: z.string().max(500).optional(),
})
```

### Tests W10

**Unitarios:**
- `createSegmentSchema` valida key format.
- `SegmentRuleEditor` renderiza reglas existentes.

**Integracion:**
- Lista renderiza segmentos del mock.
- Crear segmento → aparece en la lista.
- Editar reglas de segmento → save → reglas persisten.

### Checklist W10

- [x] `app/projects/[slug]/segments/page.tsx` + `[key]/page.tsx`
- [x] `components/segments/segments-table.tsx`
- [x] `components/segments/create-segment-dialog.tsx`
- [x] `components/segments/segment-rule-editor.tsx`
- [x] `createSegmentSchema` en `lib/schemas.ts`
- [x] Funciones en `lib/api.ts`: `getSegments()`, `createSegment()`, `archiveSegment()`, `saveSegmentRules()`
- [x] Tests unitarios: createSegmentSchema, SegmentRuleEditor
- [x] Tests integracion: lista, crear, editar reglas
- [x] `npm run test:run` pasa

---

## M2-17. Dependencias entre Fases W

```
W0 (Scaffold)
  └── W1 (Login)
        └── W2 (Projects)
              └── W3 (Flags List)
                    ├── W4 (Project Settings)
                    ├── W5 (API Keys)
                    ├── W6 (Audit Log)
                    └── W9 (Rule Editor)
              └── W7 (Account)   [depende de W2 solo por el sidebar]
W0 ──────────── W8 (Setup)       [independiente, no necesita sesion]
W3 + W9 ─────── W10 (Segments)  [el editor de reglas es el mismo patron]
```

W8 (Setup) es la unica pantalla que NO requiere sesion y puede desarrollarse en paralelo con W1.
W9 (Rule Editor) requiere W3 (Flags List) porque se accede desde ahi.
W10 (Segments) es mas facil despues de W9 porque reusan los componentes de rules.

---

## M2-18. Estimacion de Esfuerzo

| Fase | Pantallas | Componentes nuevos | Tests aprox. | Complejidad |
|---|---|---|---|---|
| W0 — Scaffold | 0 | ~10 (utils, layout base) | ~15 | Baja |
| W1 — Login | 1 | 3 | ~12 | Baja |
| W2 — Projects | 1 | 4 | ~10 | Baja |
| W3 — Flags List | 1 | 5 | ~15 | Media |
| W4 — Project Settings | 1 | 3 | ~8 | Baja |
| W5 — API Keys | 1 | 4 | ~12 | Media |
| W6 — Audit Log | 1 | 3 | ~8 | Baja |
| W7 — Account | 1 | 2 | ~8 | Baja |
| W8 — Setup | 1 | 2 | ~8 | Baja |
| W9 — Rule Editor | 1 | 5 | ~20 | Alta |
| W10 — Segments | 1 | 4 | ~12 | Media |
| **Total** | **10** | **~45** | **~128** | — |
