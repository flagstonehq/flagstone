# Flagstone

> Self-hosted feature flag server with native OpenTelemetry observability. Built in Go.

[![Go Version](https://img.shields.io/badge/go-1.22+-00ADD8?logo=go)]()
[![License](https://img.shields.io/badge/license-AGPL--3.0-blue)](LICENSE)
[![Status](https://img.shields.io/badge/status-WIP-orange)]()

---

## Table of Contents

- [What is Flagstone?](#what-is-flagstone)
- [The Problem](#the-problem)
- [Why Flagstone?](#why-flagstone)
- [Architecture](#architecture)
- [Tech Stack](#tech-stack)
- [Features](#features)
- [Usage Examples](#usage-examples)
- [Authentication & Security](#authentication--security)
- [Infrastructure & Costs](#infrastructure--costs)
- [Roadmap](#roadmap)
- [Project Structure](#project-structure)
- [Local Development](#local-development)
- [Deploy to AWS](#deploy-to-aws)
- [Additional Docs](#additional-docs)
- [Contributing](#contributing)
- [License](#license)

---

## What is Flagstone?

Flagstone is a **feature flag** server (also known as feature toggles or feature switches). It lets you **enable or disable features in your application without deploying new code**, control which users see what, and run gradual rollouts or A/B tests from a central control plane.

Instead of hardcoding `if userIsBeta { ... }`, you write `if flagstone.IsEnabled("new-feature", user) { ... }` and the decision is made at runtime from a central server.

## The Problem

At some point every team hits the same wall: you want to ship code without exposing a feature, turn off one behavior without rolling back the whole release, or limit access to a subset of users without hardcoding conditions per-service. The options available have real tradeoffs:

| Tool | Issue |
|---|---|
| **LaunchDarkly** | Excellent, but starts at ~$300 USD/month |
| **Unleash** | Open source but operationally heavy (TypeScript, many moving parts) |
| **Flagsmith** | Good, but self-hosted version lacks key SaaS features (Python) |
| **Flipt** | Closest competitor — also Go + OTel. Moved to Git-native config in v2, adding complexity for teams that want a simple API-first approach |
| **Roll your own** | You end up writing the same thing at every company |

**Flagstone targets the gap**: one binary + Postgres, API-first, with native OpenTelemetry observability. Compared to Flipt, it keeps a traditional database backend — no Git-based config workflows.

## Why Flagstone?

| Feature | Flagstone | LaunchDarkly | Unleash | Flagsmith | Flipt |
|---|---|---|---|---|---|
| Self-hosted | Yes | No | Yes | Yes (limited) | Yes |
| Open source | Yes | No | Yes | Yes | Yes |
| Setup < 5 min | Yes | Yes | No | Maybe | Yes |
| Native OTel observability | **Yes** | Partial | No | No | Yes |
| Real-time streaming | Yes (SSE) | Yes | Yes | Partial | Yes |
| Operational footprint | Low | N/A | High | Medium | Low |
| Config model | API-first | API | API | API | Git-native (v2) |
| Language | Go | ? | TypeScript | Python | Go |
| Cost | Free | $$$ | Free/$$$ | Free/$$$ | Free |

Every evaluation emits OpenTelemetry traces and metrics — so you can correlate a flag change with a latency spike in Grafana or Honeycomb without switching tools. One binary, one Postgres, optional Redis.

## Architecture

```mermaid
flowchart TB
    subgraph Clients["Clients"]
        SDK["SDK\n(Go / TS / Python)"]
        Cache[("Local cache\nin-memory")]
        Dashboard["Dashboard\n(Web UI)"]
        SDK --- Cache
    end

    subgraph FlagstoneServer["Flagstone Server (Go)"]
        direction TB
        Auth["Auth Middleware\n(API Key · JWT)"]
        API["REST API\n/api/v1/..."]
        Engine["Rule Engine\n(JSONB · FNV hashing)"]
        SSE["SSE Hub\n/stream"]
        Auth --> API
        API --> Engine
        API --> SSE
    end

    subgraph Storage["Persistence"]
        PG[("PostgreSQL\nsource of truth")]
        Redis[("Redis\ncache + pub/sub")]
    end

    subgraph Obs["Observability"]
        OTel["OTel Collector"]
        Prom["Prometheus"]
        Tempo["Grafana Tempo"]
        Grafana["Grafana"]
        OTel --> Prom & Tempo
        Prom & Tempo --> Grafana
    end

    SDK     -->|"API Key · POST /evaluate"| Auth
    Dashboard -->|"JWT · CRUD /flags /segments"| Auth
    SSE     -->|"SSE — flag change events"| SDK
    Engine  --> Redis & PG
    Redis   -->|"pub/sub · invalidation"| SSE
    FlagstoneServer -->|"traces + metrics"| OTel
```

### Visual overview

The [`diagrams/`](./diagrams/) folder contains a full architecture canvas exported from Excalidraw, covering the data model, auth flows, rule evaluation engine, pub/sub propagation, observability stack, AWS infrastructure, API design, and the complete SDK↔API sequence.

[![Flagstone Architecture Overview](./diagrams/flagstone-overview.svg)](./diagrams/flagstone-overview.svg)

### Components

**Server (Go core)**: REST API, rule evaluation engine, SSE streaming, and persistence with Postgres + Redis. Authentication via hashed API keys (SDKs) and JWT sessions (dashboard).

**SDKs**: Lightweight client libraries. Cache evaluations locally, listen for changes via SSE streaming, and degrade gracefully if the server goes down. Priority languages: Go, TypeScript/JavaScript, Python.

**Dashboard web**: UI for creating flags, defining rules, viewing evaluations in real-time, and auditing changes.

**Storage**: PostgreSQL as source of truth. Redis for distributed cache and internal pub/sub between server instances.

> See [DESIGN.md](./DESIGN.md) for full architectural rationale and [SECURITY.md](./SECURITY.md) for the auth and threat model.

## Tech Stack

| Layer | Technology | Why |
|---|---|---|
| Server language | **Go 1.22+** | Cheap concurrency, static binaries, easy deploy |
| API | **HTTP/REST** | Simple, curl-able, debuggable. gRPC planned for v2 SDKs |
| Streaming | **Server-Sent Events** | Simpler than WebSockets, sufficient for unidirectional push |
| Storage | **PostgreSQL 16+** | ACID, JSONB for complex rules, ubiquitous |
| Cache | **Redis 7+** | Distributed cache + pub/sub for multi-instance coordination |
| Logging | **zap** (`go.uber.org/zap`) | High-performance structured logging in the eval hot path |
| Observability | **OpenTelemetry** | Standardized traces + metrics + logs |
| Web dashboard | **Next.js 15 + TypeScript** | Largest AI training-data corpus → best AI-assisted development |
| UI components | **shadcn/ui + Tailwind CSS** | Designer-quality components without being a designer |
| Containers | **Docker** (split: API + Web) | Independent build/scale, single `docker-compose.yml` for self-host |
| IaC | **Terraform** | Reproducible, versioned infrastructure |
| Cloud | **AWS** | Generous free tier + learning value |
| CI/CD | **GitHub Actions** | Community standard, free for OSS |
| Tests | **testify + testcontainers-go** | Unit tests + integration with real DB |
| Migrations | **golang-migrate** | Schema versioning |
| Standards | **OpenFeature** (planned) | CNCF standard for feature flag evaluation |

## Features

### Core

- [x] Boolean flags (on/off)
- [x] User and attribute targeting
- [x] Reusable segments
- [x] Percentage-based rollout with consistent hashing
- [x] Rules with AND/OR/NOT logic
- [x] Per-environment overrides (dev/staging/prod)
- [x] REST API + Go SDK
- [x] JWT auth for dashboard, API key auth for SDKs
- [x] Audit log (append-only, DB-enforced immutability)

### Advanced

- [ ] Multivariate flags (string/number/json variants)
- [x] Real-time streaming (SSE)
- [x] Multi-tenancy with isolation
- [x] Dashboard web (CRUD + real-time)
- [ ] One-click rollback
- [ ] TypeScript and Python SDKs
- [ ] Webhooks on changes
- [x] Per-flag usage metrics (via OTel)
- [ ] Rate limiting on API

### Differentiators

- [x] OpenTelemetry traces on every evaluation with flag attributes
- [x] Pre-configured Prometheus metrics
- [ ] Example Grafana dashboard included
- [x] Automatic log-trace correlation
- [ ] Shadow mode: evaluate but don't apply, to measure impact before activating
- [ ] OpenFeature provider (CNCF standard compatibility)

## Usage Examples

### Create a flag (REST API)

```bash
curl -X POST http://localhost:8080/api/v1/flags \
  -H "Authorization: Bearer $API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "key": "new-checkout",
    "description": "New checkout flow with Apple Pay",
    "type": "boolean",
    "default": false,
    "rules": [
      {
        "conditions": {
          "attribute": "email", "op": "ends_with", "value": "@company.com"
        },
        "value": true
      },
      {
        "conditions": {
          "all": [
            { "attribute": "country", "op": "in", "value": ["AR", "BR", "CL"] },
            { "attribute": "plan", "op": "neq", "value": "free" }
          ]
        },
        "rollout": { "percentage": 25 },
        "value": true
      }
    ]
  }'
```

### Evaluate from Go

```go
package main

import (
    "context"
    "net/http"
    "os"

    "github.com/flagstonehq/flagstone/pkg/sdk"
)

func main() {
    client, err := sdk.New(
        sdk.WithEndpoint("http://localhost:8080"),
        sdk.WithAPIKey(os.Getenv("FLAGSTONE_API_KEY")),
    )
    if err != nil {
        panic(err)
    }
    defer func() { _ = client.Close() }()

    ctx := context.Background()
    _ = client.Start(ctx) // loads the snapshot, then subscribes to SSE for live updates

    http.HandleFunc("/checkout", func(w http.ResponseWriter, r *http.Request) {
        // Safe-default API: pass the value to serve if the flag is missing
        // or not loaded. Never returns an error, never blocks on the network.
        enabled := client.Bool(r.Context(), "new-checkout", false, map[string]any{
            "user_id": r.URL.Query().Get("user_id"),
            "plan":    "premium",
        })
        if enabled {
            renderNewCheckout(w)
        } else {
            renderClassicCheckout(w)
        }
    })
    _ = http.ListenAndServe(":9000", nil)
}
```

The SDK fetches a single snapshot of all flags and segments in your
environment, caches it in memory, and re-fetches automatically when
Flagstone pushes a `flag_change` or `segment_change` event over SSE.
No more polling, no more stale flags.

## Authentication & Security

Flagstone uses a **dual auth model**:

| Who | Method | Token type | Scope |
|---|---|---|---|
| SDKs / machines | API Key | SHA-256 hashed, prefix-visible | Single environment |
| Dashboard users | Email + password | JWT (short-lived) + refresh token | Tenant-scoped, role-based |

**API keys** are scoped to one environment. If the production key leaks, dev and staging remain safe. Keys are stored as SHA-256 hashes — the raw key is shown once at creation and never stored.

**Dashboard auth** uses bcrypt for password hashing, JWT access tokens (15 min TTL), and HTTP-only refresh tokens (7 day TTL). Role-based access: `owner > admin > member > viewer`.

**Infrastructure security**: RDS in private subnets, security groups by reference (not IP), IMDSv2 enforced, EBS + RDS encryption at rest, IAM roles (no access keys).

> Full threat model and auth architecture in [SECURITY.md](./SECURITY.md).

## Infrastructure & Costs

Designed to run **free on AWS for 12-24 months** combining free tier + credits.

### Current setup (free tier)

| Resource | Type | Monthly cost |
|---|---|---|
| Compute | EC2 t4g.small (ARM Graviton2) | $0 until Dec 2026 |
| Database | RDS db.t3.micro Postgres | $0 first 12 months |
| Storage | 20GB EBS gp3 + 20GB RDS | $0 |
| Transfer | < 100GB/month outbound | $0 |
| **Total** | | **$0/month** |

### After free tier

- **Month 13-24**: ~$20/month (RDS stops being free, EC2 still free until Dec 2026).
- With $300 AWS credits: **15 additional months at no cost**.
- **Month 24+**: evaluate. If there's traction, cost is justified. If not, migrate to Hetzner (~$5-8/month).

> Full cost breakdown and infrastructure rationale in [DESIGN.md](./DESIGN.md#cost-strategy).

## Roadmap

### Milestone 1 — Local MVP (weeks 1-4)

Goal: a running server with secure auth, tenant-scoped flag CRUD, and rule evaluation.

- Database schema and migrations (all core tables, including `sessions`)
- Tenant/project/environment bootstrap flow (`POST /setup`, `POST /projects`)
- Dashboard auth: JWT + refresh tokens + bcrypt password hashing
- API key authentication for SDKs
- RBAC middleware (owner/admin/member/viewer)
- Tenant-scoped queries in storage layer (every query joins on `tenant_id`)
- REST API for flag CRUD, protected by JWT + RBAC
- Audit log writes from all mutation endpoints (table + trigger already in migration)
- Rule evaluation engine
- Basic Go SDK (no cache)
- Unit tests

### Milestone 2 — Real client (weeks 5-8)

Goal: a production-ready client experience with streaming, caching, and a usable web UI.

- SDK with local cache + SSE streaming (with `Last-Event-ID` replay)
- Web dashboard in `web/` — **Next.js 15 + TypeScript + Tailwind + shadcn/ui** (see [DESIGN.md](./DESIGN.md#why-nextjs--shadcnui-for-the-web-dashboard))
- Visual rule builder + inline "Try it" evaluation panel
- Split container images: `Dockerfile.api` (Go, ~25MB) + `web/Dockerfile` (Next.js, ~120MB)
- `docker-compose.yml` for one-command self-host (api + web + postgres + redis)
- Rate limiting (in-process token bucket)
- HTTPS / TLS termination (Caddy or ALB)
- CORS configuration
- Security headers (CSP, HSTS, X-Frame-Options)
- Integration tests with testcontainers + Playwright E2E for the dashboard

### Milestone 3 — Production ready (weeks 9-14)

Goal: differentiation features and operations.

- OpenTelemetry traces + metrics on every evaluation (the headline differentiator)
- Pre-configured Grafana dashboard
- Automated deploy to AWS via Terraform
- Multi-tenant management UI (tenant switching, member invites, role changes)
- OpenFeature Go provider
- Complete documentation
- API key expiration enforcement

### Milestone 4 — Community (weeks 15-24)

- TypeScript SDK
- Python SDK
- Webhooks
- Helm chart for Kubernetes
- Show HN, technical posts, awesome-go

## Project Structure

```
flagstone/
├── cmd/
│   ├── flagstone/          # Server entry point (Go binary)
│   └── seed/               # Demo data seeder (idempotent)
├── internal/
│   ├── api/                # HTTP handlers and routing
│   ├── auth/               # API keys, JWT, sessions, RBAC, plan enforcement
│   ├── config/             # Configuration loading
│   ├── engine/             # Rule evaluation engine (pure, no I/O)
│   ├── storage/            # Persistence layer (Postgres + decorators for Redis & in-memory)
│   ├── streaming/          # Server-Sent Events
│   └── telemetry/          # OpenTelemetry setup
├── pkg/
│   └── sdk/                # Go SDK (importable by third parties)
├── web/                    # Next.js 15 dashboard (TypeScript + Tailwind + shadcn/ui)
│   ├── app/                # App Router pages and layouts
│   ├── components/         # shadcn/ui components + custom components
│   ├── lib/                # API client, hooks, utilities
│   ├── package.json
│   └── Dockerfile          # Next.js production image
├── migrations/             # SQL migrations (golang-migrate)
│   ├── 000001_init.up.sql
│   └── 000002_email_flows.up.sql
├── deploy/
│   └── terraform/          # AWS infrastructure as code
├── diagrams/               # Excalidraw architecture diagrams (SVG + PNG)
├── .github/
│   └── workflows/          # GitHub Actions CI/CD
├── docker-compose.yml      # One-command local dev (api + web + postgres + redis + migrate + seed)
├── Dockerfile              # Multi-stage Go build (API + seed + migrate CLI)
├── Makefile                # Dev commands (build, test, migrate, lint)
├── DESIGN.md               # Architecture decisions and rationale
├── SECURITY.md             # Auth model, threat model, restore runbook
└── README.md               # This file
```

> The `web/` directory is a standalone Next.js project with its own `package.json` and `Dockerfile`. The Go API and the dashboard are built and deployed as **separate container images**. They communicate over HTTP via the REST API. See [DESIGN.md → Container Strategy](./DESIGN.md#container-strategy-separate-images-single-docker-composeyml) for the rationale.

## Local Development

### Quickstart (Docker — recommended)

```bash
git clone https://github.com/flagstonehq/flagstone
cd flagstone
docker compose up -d                        # postgres, redis, migrate, api, web
docker compose --profile seed run --rm seed  # one-time demo data
open http://localhost:3000                   # Login: admin@acme.com / password123
```

This starts everything: Postgres, Redis, runs migrations, the Go API, and the Next.js dashboard.
The `seed` service provisions a demo tenant, project, flags, segments, and API keys.

### Developer setup (Go + Node)

If you prefer running Go and Node locally for faster iteration:

```bash
# Requirements: Go 1.22+, Node 18+, Make, golang-migrate CLI

# Start only Postgres + Redis
docker compose up -d postgres redis

# Run migrations and start the API
make migrate && make run

# In another terminal — start the dashboard
cd web && npm install && npm run dev
```

### Health checks

```bash
# Liveness — is the process alive?
curl http://localhost:8080/healthz
# {"status":"ok","version":"dev","uptime_seconds":12}

# Readiness — can it serve traffic? (checks Postgres + Redis)
curl http://localhost:8080/readyz
# {"status":"ready","checks":{"postgres":{"status":"up","latency_ms":2},"redis":{"status":"up","latency_ms":1}}}
```

### Common commands

```bash
make help        # Show all available targets
make build       # Build the binary
make test        # Run unit tests
make test-int    # Integration tests (needs Postgres + Redis)
make lint        # Run golangci-lint
make fmt         # Format code
make migrate     # Run migrations up
make seed        # Populate dev DB with demo data (server must be running)
make down        # Stop docker dependencies
make clean       # Stop + delete volumes (fresh DB)
```

## Deploy to AWS

The `deploy/terraform/` directory contains the complete infrastructure as code.

### Prerequisites

1. AWS account with free tier / credits available
2. AWS CLI configured (`aws configure`)
3. Terraform 1.5+ installed
4. An EC2 Key Pair created in the console (`.pem` downloaded)

### Steps

```bash
cd deploy/terraform/

# Copy example and fill in your values
cp terraform.tfvars.example terraform.tfvars
# Edit terraform.tfvars — you MUST set ssh_allowed_cidr to your IP

# Initialize
terraform init

# Review what will be created
terraform plan

# Apply
terraform apply

# Connect to the server
$(terraform output -raw ssh_command)
```

### What it creates

- VPC with public and private subnets in 2 AZs
- EC2 t4g.small with Docker preinstalled
- RDS PostgreSQL 16 in private subnets
- Security groups (HTTP/HTTPS open, SSH restricted to your IP)
- IAM role with CloudWatch and SSM permissions

### Traps to avoid

- **`ssh_allowed_cidr` has no default** — Terraform will refuse to run until you set your IP. This is intentional (the previous `0.0.0.0/0` default was a security risk).
- **NEVER commit `terraform.tfvars`** (contains secrets). The `.gitignore` already excludes it.
- Configure billing alarms in the AWS console before running `apply`. $5 and $20 are good starting thresholds.

## Additional Docs

- **[DESIGN.md](./DESIGN.md)** — Architectural decisions: database design, rule engine, caching, infrastructure, costs.
- **[SECURITY.md](./SECURITY.md)** — Authentication model, authorization (RBAC), API key handling, threat model, compliance considerations.
- **[migrations/](./migrations/)** — Complete SQL database schema.
- **[deploy/terraform/](./deploy/terraform/)** — AWS infrastructure as code.

## Contributing

Currently a solo-dev project in early development. Once it reaches Milestone 2, I'll open issues tagged `good first issue` for anyone who wants to help.

Areas where help will be especially welcome:

- SDKs in new languages
- Documented use cases
- Performance benchmarks
- OpenFeature provider implementation

## License

AGPL-3.0. You can use, self-host, and modify Flagstone freely, but if you run a modified version as a network service you must publish the source. See [LICENSE](LICENSE) for the full terms.

Flagstone Cloud (the hosted version) is operated by the copyright holder under a separate commercial arrangement.

---

**Built with Go from Argentina**
