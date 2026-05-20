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
9. [Fase 5 — CRUD Endpoints](#9-fase-5--crud-endpoints)
10. [Fase 6 — Audit Log Endpoint](#10-fase-6--audit-log-endpoint)
11. [Fase 7 — Rule Evaluation Engine](#11-fase-7--rule-evaluation-engine)
12. [Fase 8 — Evaluate Endpoints](#12-fase-8--evaluate-endpoints)
13. [Fase 9 — CI Pipeline + Integracion Final](#13-fase-9--ci-pipeline--integracion-final)
14. [Dependencias entre Fases](#14-dependencias-entre-fases)
15. [Estimacion de Esfuerzo](#15-estimacion-de-esfuerzo)

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
- **Web dashboard**: Next.js 15 + TypeScript + Tailwind + shadcn/ui (Milestone 2). Container separado del API.
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
1. Importar `"github.com/thomas-vilte/flagstone/internal/config"`
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

- [ ] Crear `internal/storage/postgres.go` con `NewPool()` (config pool, retry)
- [ ] Crear `internal/storage/models.go` con todos los structs del dominio
- [ ] Crear `internal/storage/errors.go` con sentinel errors
- [ ] Crear `internal/storage/types.go` con value types para IDs
- [ ] Crear `internal/storage/tenant_store.go` (4 metodos)
- [ ] Crear `internal/storage/user_store.go` (5 metodos)
- [ ] Crear `internal/storage/session_store.go` (5 metodos)
- [ ] Crear `internal/storage/member_store.go` (5 metodos)
- [ ] Crear `internal/storage/project_store.go` (4 metodos)
- [ ] Crear `internal/storage/environment_store.go` (4 metodos)
- [ ] Crear `internal/storage/apikey_store.go` (5 metodos)
- [ ] Crear `internal/storage/flag_store.go` (5 metodos)
- [ ] Crear `internal/storage/flag_env_store.go` (4 metodos, bulk JOIN, OCC)
- [ ] Crear `internal/storage/segment_store.go` (6 metodos)
- [ ] Crear `internal/storage/audit_store.go` (2 metodos: Insert, Query)
- [ ] Tests: `postgres_test.go` — pool creation, ping
- [ ] Tests: `tenant_store_test.go` — CRUD, ExistsAny
- [ ] Tests: `user_store_test.go` — CRUD, CITEXT case-insensitive
- [ ] Tests: `session_store_test.go` — CRUD, DeleteExpired
- [ ] Tests: `member_store_test.go` — Add, GetRole, UpdateRole, Remove
- [ ] Tests: `project_store_test.go` — CRUD, tenant-scoped
- [ ] Tests: `environment_store_test.go` — CRUD, project-scoped
- [ ] Tests: `apikey_store_test.go` — CRUD, Revoke, active index
- [ ] Tests: `flag_store_test.go` — CRUD, Archive
- [ ] Tests: `flag_env_store_test.go` — Upsert, OCC conflict, bulk JOIN
- [ ] Tests: `segment_store_test.go` — CRUD, Archive
- [ ] Tests: `audit_store_test.go` — Insert, Query con filtros

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

- [ ] Crear `internal/auth/jwt.go` (GenerateAccessToken, ValidateAccessToken, HS256)
- [ ] Crear `internal/auth/claims.go` (Claims struct con sub, tid, role, exp, iss)
- [ ] Crear `internal/auth/password.go` (HashPassword, VerifyPassword, bcrypt cost=12)
- [ ] Crear `internal/auth/apikey.go` (GenerateAPIKey, crypto/rand, base62, SHA-256)
- [ ] Crear `internal/auth/token.go` (GenerateRefreshToken, crypto/rand, base62, SHA-256)
- [ ] Crear `internal/auth/roles.go` (Role type, Level, AtLeast)
- [ ] Crear `internal/auth/base62.go` (Encode base62)
- [ ] Tests: `jwt_test.go` — round-trip, expired, wrong secret, claims
- [ ] Tests: `password_test.go` — round-trip, wrong password, cost
- [ ] Tests: `apikey_test.go` — formato, hash deterministico, prefix, unicidad
- [ ] Tests: `token_test.go` — round-trip, unicidad
- [ ] Tests: `roles_test.go` — hierarchy, AtLeast, unknown role
- [ ] Tests: `base62_test.go` — round-trip, edge cases

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

- [ ] Crear `internal/api/server.go` (Server struct, Routes, middleware chain)
- [ ] Crear `internal/api/response.go` (JSON, Error, ErrorFromDomain)
- [ ] Crear `internal/api/request.go` (DecodeJSON con body limit + content-type)
- [ ] Crear `internal/api/context.go` (helpers para extraer del context)
- [ ] Crear `internal/api/middleware/recover.go`
- [ ] Crear `internal/api/middleware/request_id.go`
- [ ] Crear `internal/api/middleware/logger.go`
- [ ] Crear `internal/api/middleware/body_limit.go`
- [ ] Crear `internal/api/middleware/content_type.go`
- [ ] Crear `internal/api/middleware/auth_jwt.go`
- [ ] Crear `internal/api/middleware/auth_apikey.go`
- [ ] Crear `internal/api/middleware/rbac.go` (RequireRole)
- [ ] Crear `internal/api/handlers/setup.go` (POST /api/v1/setup)
- [ ] Tests: `recover_test.go`
- [ ] Tests: `request_id_test.go`
- [ ] Tests: `logger_test.go`
- [ ] Tests: `body_limit_test.go`
- [ ] Tests: `content_type_test.go`
- [ ] Tests: `auth_jwt_test.go`
- [ ] Tests: `auth_apikey_test.go`
- [ ] Tests: `rbac_test.go`
- [ ] Tests: `setup_test.go` (bootstrap exitoso, 409, validacion)

---

## 8. Fase 4 — Auth Endpoints

**Objetivo:** Login, refresh, logout funcionales con token rotation.

### Estructura de archivos

```
internal/api/handlers/
├── auth.go            # login, refresh, logout handlers
└── auth_test.go       # integration tests
```

### Detalle por archivo

#### `handlers/auth.go`

**`POST /api/v1/auth/login`**:

```
Body: { "email": "admin@acme.com", "password": "..." }

Server:
  1. Validar email + password (formato, largo)
  2. Lookup user by email (CITEXT, case-insensitive)
  3. bcrypt.CompareHashAndPassword(stored_hash, password)
  4. Si falla -> 401 (mensaje generico, no distingue user no existe vs password incorrecto)
  5. Generar JWT access token (15 min)
  6. Generar refresh token (opaque, 7 dias)
  7. INSERT INTO sessions (SHA-256 del refresh token)
  8. UPDATE users SET last_login_at = NOW()
  9. INSERT INTO audit_log (auth.login)
  10. Retornar { access_token, token_type: "Bearer", expires_in: 900 }
  11. Set-Cookie: refresh_token=<opaque>; HttpOnly; Secure; SameSite=Strict; Path=/api/v1/auth
```

**`POST /api/v1/auth/refresh`**:

```
Cookie: refresh_token=<opaque>

Server:
  1. Extraer refresh token del cookie httpOnly
  2. SHA-256(token) -> lookup en sessions
  3. Verificar expires_at > NOW()
  4. DELETE old session (token rotation)
  5. INSERT new session con nuevo refresh token
  6. Generar nuevo JWT access token
  7. INSERT INTO audit_log (auth.refresh)
  8. Retornar nuevo access_token + Set-Cookie nuevo refresh_token
```

**Token rotation** (de SECURITY.md): Cada refresh invalida el viejo refresh token y emite uno nuevo. Si un atacante roba un refresh token y el usuario legitimo tambien hace refresh, uno de los dos recibe "invalid token" — senalando la compromision.

**`POST /api/v1/auth/logout`**:

```
Authorization: Bearer <access_token>
Cookie: refresh_token=<opaque>

Server:
  1. DELETE session matching refresh_hash
  2. Clear refresh_token cookie (Set-Cookie con MaxAge=-1)
  3. INSERT INTO audit_log (auth.logout)
```

El access token JWT queda valido hasta su expiracion de 15 min (stateless). Para invalidacion inmediata se podria agregar un blocklist en Redis despues, pero para 15 minutos de exposicion el tradeoff es aceptable (de SECURITY.md).

### Tests Fase 4

| Test | Que verifica |
|---|---|
| Login exitoso | 201, access_token valido, refresh_token cookie set, audit log entry |
| Login password incorrecto | 401, mensaje generico (no dice "password incorrect") |
| Login email no existe | 401, mismo mensaje generico (previene enumeration) |
| Login email invalido | 400 |
| Refresh exitoso | Nuevo access_token, nuevo refresh_token cookie, old session deleted |
| Refresh con token expirado | 401 |
| Refresh con token ya usado (replay) | 401 (token rotation lo invalido) |
| Logout exitoso | Session deleted, cookie cleared |

### Checklist Fase 4

- [ ] Crear `internal/api/handlers/auth.go` (login, refresh, logout)
- [ ] Tests: login exitoso, password incorrecto, email no existe, email invalido
- [ ] Tests: refresh exitoso, token expirado, replay detection
- [ ] Tests: logout exitoso, session deleted

---

## 9. Fase 5 — CRUD Endpoints

**Objetivo:** Gestion completa de flags, segments, projects, environments, API keys desde la dashboard.

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

- [ ] Crear `internal/api/handlers/projects.go` (POST, GET, GET/:slug, PUT/:slug)
- [ ] Crear `internal/api/handlers/environments.go` (GET, POST)
- [ ] Crear `internal/api/handlers/apikeys.go` (POST, GET, DELETE/:id)
- [ ] Crear `internal/api/handlers/flags.go` (POST, GET, GET/:key, PUT/:key, DELETE/:key)
- [ ] Crear `internal/api/handlers/flag_envs.go` (PUT con OCC)
- [ ] Crear `internal/api/handlers/segments.go` (POST, GET, GET/:key, PUT/:key, DELETE/:key)
- [ ] Tests: projects CRUD, auto-env creation, tenant-scoped
- [ ] Tests: environments CRUD
- [ ] Tests: API keys CRUD, raw key solo una vez, revoke
- [ ] Tests: flags CRUD, auto-flag_envs, key validation
- [ ] Tests: flag_envs OCC version conflict
- [ ] Tests: segments CRUD
- [ ] Tests: RBAC enforcement por endpoint
- [ ] Tests: cross-tenant isolation

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

- [ ] Crear `internal/api/handlers/audit.go` (GET /api/v1/audit con filtros)
- [ ] Tests: query sin filtros, con filtros, pagination
- [ ] Tests: cross-tenant isolation
- [ ] Tests: verificar que mutations de fases anteriores generaron audit entries

---

## 11. Fase 7 — Rule Evaluation Engine

**Objetivo:** El core algoritmico del sistema. Funciones puras, sin I/O, altamente testeables.

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

- [ ] Crear `internal/engine/types.go` (EvaluateRequest, EvaluateResult, FlagConfig, Reason, Rule, ConditionNode, RolloutConfig)
- [ ] Crear `internal/engine/engine.go` (Evaluate, EvaluateAll, panic recovery)
- [ ] Crear `internal/engine/conditions.go` (evaluateNode, all/any/not/leaf)
- [ ] Crear `internal/engine/operators.go` (15 operadores, type coercion)
- [ ] Crear `internal/engine/rollout.go` (FNV-1a, consistent hashing)
- [ ] Crear `internal/engine/segments.go` (resolveSegment, cycle detection)
- [ ] Tests: `engine_test.go` — default, disabled, rule match, first match wins, fall through
- [ ] Tests: `conditions_test.go` — all, any, not, nested, short-circuit, max depth
- [ ] Tests: `operators_test.go` — cada operador x (match, no match, type mismatch, missing attr)
- [ ] Tests: `rollout_test.go` — 0%, 100%, deterministico, monotonic, no user_id, custom seed
- [ ] Tests: `segments_test.go` — normal, not found, circular, transitive

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

- [ ] Crear `internal/api/handlers/evaluate.go` (single + bulk evaluation)
- [ ] Implementar context validation (max keys, max length, tipos)
- [ ] Implementar eager loading (flags JOIN flag_environments, segments)
- [ ] Tests: single eval (existe, not found, disabled, rule match, rollout)
- [ ] Tests: bulk eval (todos los flags, formato correcto)
- [ ] Tests: auth (API key invalido, revocado)
- [ ] Tests: context validation (too many keys, value too long, invalid type)

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

```
Fase 0: Foundation
  │
  ├──▶ Fase 1: Storage Layer (depende de Fase 0 — necesita Postgres)
  │     │
  │     ├──▶ Fase 2: Auth Core (independiente de storage, pero se hace despues)
  │     │     │
  │     │     ▼
  │     │   Fase 3: Middleware + Bootstrap (depende de Fases 1 + 2)
  │     │     │
  │     │     ▼
  │     │   Fase 4: Auth Endpoints (depende de Fase 3)
  │     │     │
  │     │     ▼
  │     │   Fase 5: CRUD Endpoints (depende de Fase 3)
  │     │     │
  │     │     ▼
  │     │   Fase 6: Audit Log Endpoint (depende de Fase 5)
  │     │
  │     └──▶ Fase 7: Rule Engine (independiente — pure code, sin I/O)
  │
  └──▶ Fase 8: Evaluate Endpoints (depende de Fases 3 + 7 + 1)
         │
         ▼
       Fase 9: CI Pipeline + Integracion Final (depende de todo lo anterior)
```

**Nota**: La Fase 7 (Engine) se puede hacer en paralelo con las Fases 2-6 porque es codigo puro sin dependencias de storage ni HTTP. Si hay dos developers, uno podria hacer el engine mientras el otro hace auth + API.

---

## 15. Estimacion de Esfuerzo

| Fase | Archivos nuevos | Lineas aprox | Complejidad | Tests |
|---|---|---|---|---|
| 0 — Foundation | 2 editados + 1 nuevo | ~150 | Baja | Manual |
| 1 — Storage | 14 + 11 test | ~2000-2500 | Media | ~50 test cases |
| 2 — Auth Core | 7 + 6 test | ~500-600 | Media | ~25 test cases |
| 3 — Middleware | 12 + 9 test | ~700-900 | Media | ~20 test cases |
| 4 — Auth Endpoints | 1 + 1 test | ~250-350 | Baja | ~8 test cases |
| 5 — CRUD | 6 + 6 test | ~900-1200 | Media | ~30 test cases |
| 6 — Audit | 1 + 1 test | ~100-150 | Baja | ~5 test cases |
| 7 — Engine | 6 + 5 test | ~700-900 | **Alta** | ~60 test cases |
| 8 — Evaluate | 1 + 1 test | ~250-350 | Media | ~12 test cases |
| 9 — CI + Smoke | 1 editado | ~50 | Baja | Manual |
| **Total** | **~60 archivos** | **~6000-8000** | — | **~210 test cases** |

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
