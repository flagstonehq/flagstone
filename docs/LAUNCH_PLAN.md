# Flagstone Launch Plan

> **Documento vivo**: este plan se actualiza cada mes conforme ejecutamos. La realidad siempre gana a la teoría.

---

## Tabla de contenidos

1. [Visión del lanzamiento](#visión-del-lanzamiento)
2. [Fases de ejecución](#fases-de-ejecución)
3. [Timeline detallado](#timeline-detallado)
4. [Estrategia de comunicación](#estrategia-de-comunicación)
5. [Métricas y objetivos](#métricas-y-objetivos)
6. [Decisiones críticas](#decisiones-críticas)
7. [Riesgos y contingencias](#riesgos-y-contingencias)

---

## Visión del lanzamiento

### Qué es Flagstone

Flagstone es un **servidor de feature flags** autohospedado y SaaS que permite a los equipos:

- Cambiar el comportamiento de la app en producción sin redeploy
- Hacer rollouts graduales (5% → 25% → 100%)
- Controlar features desde un dashboard sin tocar código
- Observar todo via OpenTelemetry nativo (traces, métricas, logs)

### Por qué ahora

Existen alternativas (LaunchDarkly, Unleash, Flagsmith, Flipt), pero Flagstone ocupa un nicho único:

| Aspecto | Nosotros | Competencia |
|---|---|---|
| **Simplicidad** | ⭐⭐⭐⭐⭐ | ⭐⭐⭐ |
| **OTel nativo** | ✓ | ~ |
| **Self-hosted + Cloud (same code)** | ✓ | ~ |
| **Costo** | $ | $$$ |
| **Madureza** | MVP | 5-10 años |

Vamos por el greenfield: equipos que nunca usaron feature flags y quieren la opción más simple.

### Objetivo en 12 meses

```
GitHub stars:        5,000-6,500
Self-hosted users:   1,500-3,000
Cloud customers:     100-150
MRR (Revenue):       $8,000-12,000
Viabilidad:          Full-time + contratar 1 engineer
```

---

## Fases de ejecución

### Fase 0: Pre-lanzamiento (Semana 1-2, AHORA)

**Duración**: 14 días

**Objetivo**: tener MVP 100% funcional y comunicación lista

#### Lo que DEBE estar funcional para v0.1.0

```
ABSOLUTO MUST HAVE (non-negotiable):
✓ Go API server (evaluación de flags, administración)
✓ PostgreSQL schema + migrations
✓ Redis (cache + pub/sub)
✓ Dashboard Next.js (básico pero funcional)
✓ SDK Go (al menos)
✓ OpenTelemetry integration (spans, metrics, logs)
✓ Docker Compose (one-liner para self-hosted)
✓ README épico + documentación
✓ GitHub Actions CI (tests + builds)

NICE TO HAVE (skip si no está 100% listo):
- Python SDK
- Webhooks
- SAML/OIDC
- Advanced dashboard features
```

#### Checklist técnico

**API Go** (todo debe funcionar):
```
Authentication:
  ✓ POST /api/v1/setup (bootstrap inicial)
  ✓ POST /api/v1/auth/login (email + password)
  ✓ POST /api/v1/auth/refresh (JWT refresh)
  ✓ API Key validation en todas las requests SDK

Evaluación:
  ✓ POST /api/v1/evaluate/flags/:key (single flag)
  ✓ POST /api/v1/evaluate/flags (bulk, todos los flags)
  ✓ Consistent hashing para rollouts
  ✓ Rule evaluation engine (todas las condiciones)
  ✓ SSE /stream (propagación en tiempo real)

Admin:
  ✓ POST /api/v1/flags (crear flag)
  ✓ PUT /api/v1/flags/:key (actualizar)
  ✓ DELETE /api/v1/flags/:key (archivar soft)
  ✓ PUT /api/v1/flags/:key/environments/:env (config por env)
  ✓ POST /api/v1/segments (crear segment)
  ✓ GET /api/v1/segments (listar)

Observability:
  ✓ GET /metrics (Prometheus format)
  ✓ OTel spans en cada evaluación
  ✓ Structured JSON logs
  ✓ Audit log (append-only, DB trigger)

Health:
  ✓ GET /healthz (liveness)
  ✓ GET /readyz (readiness + dependency checks)
```

**Database**:
```
✓ Migration 000001_initial.up.sql crea todo
✓ Audit log con trigger (no UPDATE/DELETE)
✓ Version auto-increment en flag_environments
✓ UUIDs en lugar de bigserial
✓ Índices para performance (api_keys, flags, etc)
```

**Dashboard (Next.js)**:
```
✓ Login / signup flow
✓ Flag list + crear flag
✓ Environment configuration
✓ Rules editor (visual o JSON)
✓ Audit log viewer
✓ API keys management
✓ Settings básicos
```

**SDK Go**:
```
✓ Client.IsEnabled(ctx, flagKey, userContext) → bool
✓ Client.GetVariant() → interface{}
✓ SSE connection + reconnection logic
✓ Last-Event-ID handling
✓ In-process cache with TTL
✓ Exponential backoff + jitter
```

**Deployment**:
```
✓ docker-compose.yml (postgres, redis, api, web)
✓ Dockerfile API (multi-stage, ~25MB final)
✓ Dockerfile web (Next.js standalone)
✓ GitHub Actions CI
✓ Terraform modules (AWS - opcional para v0.1)
```

#### Justificación: por qué esto y no más

Temptation = agregar features. Realidad = la gente prefiere MVP simple que versión completa tarde.

**Por qué NO incluimos**:
- Python SDK: puede esperar a v0.1.1, hoy queremos lanzar
- Webhooks: 30% de users las necesitan, 70% no. Skip.
- SAML/OIDC: solo enterprise, v1.0+
- Advanced analytics: dashboard simple >= "no entiendo este dashboard"

**El principio**: 80/20. Con 20% del trabajo sacamos 80% del valor.

---

### Fase 1: Lanzamiento público (Semana 3)

**Duración**: 1 semana intensiva

**Objetivo**: máxima visibilidad, primeros usuarios, validación de demand

#### Jueves: Launch day

**10:00 AM ART** (hora Argentina):

1. **GitHub**:
   - Pushea repo a público
   - Crea GitHub release v0.1.0
   - Enablea Discussions (para comunidad)
   - Agrega topics: `feature-flags`, `open-source`, `golang`

2. **Código + Docs**:
   - README completo (ver [sección README](#readme-épico))
   - DESIGN.md (ya lo tenés)
   - docs/API.md (referencia de endpoints)
   - docs/SDK.md (cómo usar Go SDK)
   - docs/DEPLOYMENT.md (self-hosted + AWS)
   - CONTRIBUTING.md (cómo contribuir)

3. **Infraestructura**:
   - Discord invite link (pre-creado)
   - Twitter handle @flagstone_dev (pre-creado)
   - Blog: "Introducing Flagstone" (publicado)

**10:30 AM**: Post en Show HN

```
Title: "Flagstone – Feature flags in 60 seconds. One binary, Postgres, OTel."

Text:
Hi HN! I've been working on Flagstone, an open-source feature flag 
server designed for simplicity and observability.

Why Flagstone?
- Deploy in 60 seconds: `docker compose up`
- Single Go binary + Postgres + Redis (no Node.js, no Django)
- Native OpenTelemetry: every evaluation emits traces/metrics/logs
- 10x cheaper than LaunchDarkly, simpler than Unleash/Flipt
- MIT license, no vendor lock-in
- Same codebase: self-hosted (free) + managed SaaS ($79/mo)

GitHub: https://github.com/thomas-vilte/flagstone
Docs: https://github.com/thomas-vilte/flagstone/tree/main/docs
Discord: https://discord.gg/...

Ask me anything about the architecture, design decisions, or roadmap.
```

**11:00 AM**: Tweet 1

```
🚀 Introducing Flagstone

Feature flags, simplified.
One binary, Postgres, done.
Native OpenTelemetry.

Self-hosted: free (MIT)
Cloud: $79/month

github.com/thomas-vilte/flagstone

Launching on Show HN now 👇
```

**12:00 PM - 6:00 PM**: You're glued to the screen

- Responden TODOS los comentarios en HN (witty, technical, honest)
- Fixeás bugs que reportan en tiempo real
- Publicás v0.1.1 si encuentran issues críticos
- Monitoreás GitHub issues

**Resultado esperado en 24h**:
- 200-400 upvotes en HN (top 15-20 ese día)
- 300-500 GitHub stars
- 50-100 comentarios constructivos
- 20-30 issues (mayoría son preguntas)
- 0 customers (aún, es v0.1)
- 2-3 PRs comunitarias

#### Viernes: Momentum

Posteás en más plataformas:

**Reddit**:
- r/golang: "Flagstone – Open source feature flags server"
- r/DevOps: "Simple feature flag infrastructure for teams"
- r/webdev: "Feature flags explained: Flagstone edition"

**Dev.to**:
- Post: "Introducing Flagstone: Feature Flags Done Right"
- Include diagrams, code snippets
- Include discount code para Cloud (cuando exista)

**LinkedIn**:
- Thread: "I built Flagstone because..."
  - Problem: LaunchDarkly caro
  - Solution: open source + simple
  - Why OTel matters
  - Invite to GitHub

**Twitter**:
- Tweet 2: Comparativa visual (Flagstone vs Unleash vs LaunchDarkly)
- Tweet 3: "How we did OTel natively"
- Tweet 4: Testimonial (si alguien lo deployó already)

**Resultado acumulado fin de semana**:
- 800-1,200 stars (aceleración)
- 150-200 Discord members
- 500-1000 clones/mes

#### Por qué este timeline

**Jueves en la mañana ART**: es evening en EU/US, morning en Asia. Máxima cobertura.

**Show HN primero**: tienes 24-48h de hype concentrado. Es el momento.

**Reddit + Dev.to después**: capta long-tail traffic, SEO juice.

**LinkedIn + Twitter**: crea narrativa "founder building in public".

---

### Fase 2: Lanzamiento de Flagstone Cloud (Semana 4)

**Duración**: 1 semana

**Objetivo**: validar demand pagador, primeros ingresos

#### Qué necesitás

```
Backend:
✓ Multi-tenant Postgres (aislamiento de datos)
✓ Stripe integration (pagos)
✓ Webhooks from Stripe (subscription events)
✓ Plan limits enforcement (free: 10 flags, pro: unlimited)
✓ Usage metrics tracking (evals/month)

Frontend:
✓ Billing page (plan selection, card input)
✓ Subscription status page
✓ Usage dashboard
✓ Account settings

Infrastructure:
✓ cloud.flagstone.dev hosteado en AWS
✓ Auto-scaling (básico)
✓ SSL certificates
✓ Backups (diarios)
✓ Monitoring alerts

Documentation:
✓ Getting started (Cloud vs self-hosted)
✓ Pricing page
✓ FAQ
✓ Support email
```

#### Planes iniciales

```
FREE:
- 1 proyecto
- 10 flags máximo
- 100,000 evaluaciones/mes
- 1 usuario
- Email support

PRO ($79/mes):
- 5 proyectos
- 100 flags máximo
- 5,000,000 evaluaciones/mes
- 3 usuarios
- Email support

ENTERPRISE:
- Custom (contactar)
- Unlimited everything
- SSO (SAML/OIDC) — v1.0+
- Dedicated support — v1.0+
```

#### Lanzamiento (Jueves, Semana 4)

**Anuncio**:

```
Tweet: "Flagstone Cloud is live 🎉

Free tier: 1 project, 10 flags, 100k evals/month
Pro: $79/month, unlimited

Same codebase as self-hosted. Zero lock-in.

Sign up: cloud.flagstone.dev"

Blog post: "Introducing Flagstone Cloud"
- Why Cloud?
- Pricing
- How it compares
- Getting started link
```

**Expectativa**:
- 50-150 signups en primer día
- 10-30 trial users (agregan tarjeta)
- 1-3 primeros clientes pagos (si conviertes bien)
- $79-237 en MRR

---

### Fase 3: Aceleración (Mes 2, Semana 8-11)

**Duración**: 4 semanas

**Objetivo**: release v0.2 (more SDKs), content marketing, primeras ventas

#### Semana 8: Content marketing

**Blog 1: "Why We Built Flagstone"**

```markdown
# Why We Built Flagstone

When we looked at feature flag platforms, we saw a gap:

- LaunchDarkly: powerful but $$$
- Unleash: good but complex
- Flipt: excellent but v2 became Git-native (more complexity)

We asked: what if we built the simplest possible flag server?

The constraints:
1. One binary (Go)
2. Postgres (not exotic DB)
3. OTel native (not bolted on)
4. Self-hosted + Cloud (same code)
5. < 1000 LOC for core engine

Result: Flagstone.

Our philosophy:
- Boring tech wins
- Simpler is better
- Observability first
- No vendor lock-in
```

**LinkedIn post** (thread):
- Problem narrative
- Solution
- Design principles
- Invitation to try

**Twitter**: 3-4 tweets sobre design decisions

#### Semana 9: Release v0.2.0

**Nuevas features**:

```
NEW:
+ Python SDK (official)
+ Webhooks (flag changes trigger POST to your URL)
+ Scheduled flags (activate_at, deactivate_at timestamps)
+ Dashboard improvements:
  - Flag search
  - Bulk operations
  - Better UX
+ Performance:
  - Optimized cache invalidation
  - Faster SSE reconnections
  - Reduced latency p99

FIXES:
- SSE connection leaks under high throughput
- Race condition en bootstrap
- Postgres connection pool exhaustion
```

**Marketing**:

```
Blog: "Python SDK for Flagstone"
- Code examples
- Async support
- Integration with FastAPI/Django

Tweet: "v0.2 ships with Python support 🐍
Next: Java (v0.3), more languages"

Discord: Celebra en #announcements
```

**GitHub Release notes** (template):

```markdown
## v0.2.0 - Feature expansion

### New Features
- **Python SDK**: Official Python client for Flagstone
- **Webhooks**: React to flag changes in real-time
- **Scheduled flags**: Activate/deactivate at specific times

### Performance
- 30% faster cache invalidation
- 50ms faster SSE reconnection
- Optimized Postgres queries

### Fixes
- Fixed SSE connection leak under 10k+ QPS
- Fixed race condition in bootstrap flow
- Fixed pool exhaustion on connection churn

### Contributors
- @thomas-vilte (main)
- @community-member (fix #42)

Thanks to everyone who reported issues!
```

#### Semana 10: Sales outreach

**Contratas SDR part-time** (freelancer en Upwork, ~$1-2k/mes):

**Target list**:
- YC companies founded 2022-2023 (no tienen flag infra aún)
- Series A/B startups en Latam, US
- Tech-first companies (Django, FastAPI, Go apps)

**Template outreach**:

```
Asunto: Feature flags for [Company]

Hola [Founder/CTO],

Noté que [Company] está en el espacio [vertical]. 

Estamos building Flagstone — open source feature flags 
server hecho para teams que quieren:
- Simplicity (no LaunchDarkly complexity)
- Cost savings (10x cheaper)
- Self-hosted option (no vendor lock-in)
- Native observability (OTel built-in)

30 min demo?

V/
Thomas

P.S. Si usás flags ya, podemos mostrar cómo migrar.
```

**Goal**: 20-30 meetings/mes, 2-4 conversiones a customers

#### Semana 11: Community building

**Discord office hours**:
- 30 min, weekly, Thursday 3pm ART
- Demo feature, Q&A
- Grabá y postea en YouTube después

**GitHub "Good First Issue"**:
- 3-5 tickets fáciles para newcomers
- Documentá bien qué hace falta
- Está atento a PRs, revisá rápido

**Nombrá contributors**:
- En GitHub release notes
- En Discord #general
- En Twitter (retweet con thanks)

**Resultado en M2**:
- 2,500-3,500 stars
- 400-600 Cloud signups
- 20-35 paying customers
- $1,600-2,800 MRR
- 200+ Discord members

---

### Fase 4: Enterprise reach (Mes 3, Semana 12-15)

**Duración**: 4 semanas

**Objetivo**: primeras ventas enterprise, v1.0 planning

#### Semana 12: Blog + webinar

**Blog: "OpenTelemetry for Feature Flags"**

```markdown
# OpenTelemetry for Feature Flags

Feature flags are usually invisible — they fire and no one knows.

What if every flag evaluation was observable?

Flagstone makes that real:

1. Every evaluation emits an OTel span
2. With attributes: flag.key, flag.value, user.id, rule_matched
3. Metrics: evaluations per flag, latency per flag
4. Logs: structured, correlated with trace

Example: You deploy a flag. It causes latency spike.
You open Grafana, filter by trace_id, see exactly:
- Flagstone evaluated flag in 2ms
- Your app did something slow in 500ms
- Payment API took 2 seconds

Boom: you know what to fix.

Without OTel: you're blind.
With OTel (Flagstone): you see everything.
```

**Webinar: "Feature Flags for Product Teams"**

- 30 minutos live
- Pantalla compartida (demo)
- Q&A
- Grabado para YouTube

---

#### Semana 13-14: Sales targets

**Activa sales team** (1 person, 50% dedicated):

**Strategy**:
- Buscar companies con 50-500 devs (sweet spot)
- Ofrecer: self-hosted + Cloud hybrid
- Competir contra Unleash/Flipt (not LD yet)
- Mensage: simplicity + cost

**Deal size esperado**: $200-400/mes (pro tier o custom)

**Goal**: cierra 10-15 deals, $2-6k/mes adicional

#### Semana 15: Release planning

**v1.0 roadmap** (planeá para M4):

```
NEW:
+ SAML/OIDC authentication
+ Audit log compliance reports (SOC2)
+ SLA 99.9% (Cloud only)
+ Role-based access control (granular)
+ Java SDK (official)
+ Terraform module (easy AWS deploy)
+ API rate limiting
```

**Resultado en M3**:
- 3,500-4,500 stars
- 600-900 Cloud signups
- 40-60 paying customers
- $3,200-4,800 MRR

---

### Fase 5: Enterprise consolidation (Mes 4-6)

**Duración**: 12 semanas

**Objetivo**: v1.0 release, SAML/OIDC, primeros enterprise customers

#### Semana 16-19: v1.0 development

**Major release**:

```
NEW:
+ SAML/OIDC (enterprise auth)
+ Compliance dashboard (audit trail, reports)
+ SLA monitoring (for Cloud)
+ Java SDK
+ Terraform AWS module
+ Advanced role-based access

POLISH:
- Dashboard UX improvements
- Documentation completeness
- SDK parity across languages
```

#### Semana 20: v1.0 launch + webinar

**Webinar: "Flagstone for Enterprises"**

- Case studies (3-5 customers talking)
- SAML/OIDC demo
- Compliance explanation
- SLA guarantee

#### Semana 21-26: Consolidation

**Content creation**:
- 2 blogs/mes
- 1 video tutorial/mes
- Podcast appearances (3-5)
- Conference talks (apply para eventos)

**Community**:
- 500+ Discord members
- 1-2 community contributors regulares

**Resultado en M6**:
- 5,500-6,500 stars
- 1,300-1,800 Cloud signups
- 100-150 paying customers
- $8,000-12,000 MRR
- Viable full-time + contratar 1 engineer

---

## Timeline detallado

### Pre-lanzamiento (Hoy)

```
Lunes:
  [ ] Audita último bug en core
  [ ] Finaliza Docker setup
  [ ] Revisa README una última vez
  
Martes-Miércoles:
  [ ] Prepara Discord (canales, bots básicos)
  [ ] Prepara Twitter (@flagstone_dev)
  [ ] Escribe blog post "Introducing Flagstone"
  [ ] Prepara tweets para programar
  [ ] Revisa que CI/CD funcione
  
Jueves:
  [ ] Último push a main
  [ ] Verifica que todo está funcional
  [ ] Te vas a dormir (necesitás descanso para el launch)
```

### Lanzamiento (Semana 3)

```
Jueves:
  10:00 AM - GitHub repo público + release v0.1.0
  10:30 AM - Show HN post
  11:00 AM - Tweet 1
  12:00-18:00 - Responde comentarios
  
Viernes:
  09:00 AM - Reddit posts (golang, DevOps, webdev)
  14:00 PM - Dev.to post
  16:00 PM - LinkedIn thread
  
Sábado-Domingo:
  - Relaja, responde issues cuando aparezcan
  - Monitorea GitHub stars
```

### Cloud launch (Semana 4)

```
Lunes-Miércoles:
  - QA final de Cloud
  - Prepara landing page
  - Configura Stripe
  
Jueves:
  - Lanza cloud.flagstone.dev
  - Anuncia en Twitter + Discord
  - Blog post
  
Viernes onwards:
  - Onboarda primeros signups
  - Responds questions rápido
```

### Mes 2 (Semana 8-11)

```
Week 8: Blog 1 + LinkedIn content
Week 9: Release v0.2 + Python SDK
Week 10: Sales outreach (SDR)
Week 11: Community building (office hours, contributors)
```

### Mes 3 (Semana 12-15)

```
Week 12: Blog (OTel) + Webinar
Week 13-14: Sales activity
Week 15: v1.0 planning + roadmap
```

### Mes 4-6 (Semana 16-26)

```
Week 16-19: v1.0 development
Week 20: v1.0 launch
Week 21-26: Consolidation + team
```

---

## Estrategia de comunicación

### README épico

El README es lo primero que ve la gente. Debe:

1. **Enganche** (3 segundos)
2. **Value prop** (5 segundos)
3. **Quick start** (30 segundos para entender)
4. **Features** (why bother?)
5. **Comparación** (vs competencia)
6. **Links** (docs, discord, blog)

**Template**:

```markdown
# Flagstone

> Feature flags, simplified. One binary. Postgres. Done.
> Native OpenTelemetry. Zero vendor lock-in.

[![GitHub Stars](https://img.shields.io/github/stars/thomas-vilte/flagstone)](...)
[![License](https://img.shields.io/badge/license-MIT-blue)](LICENSE)
[![Discord](https://img.shields.io/badge/discord-join-blue)](https://discord.gg/...)

## Why Flagstone?

- **Simple**: `docker compose up` = funciona en 60 segundos
- **Single binary**: Go, ~25MB, compilable para cualquier CPU
- **Observability first**: OpenTelemetry nativo (traces, métricas, logs)
- **Cheap**: $0 self-hosted (MIT), $79/mo Cloud
- **No lock-in**: mismo código SaaS + self-hosted

## Quick start

```bash
# Clone
git clone https://github.com/thomas-vilte/flagstone
cd flagstone

# Deploy (local)
docker compose up

# Dashboard
open http://localhost:3000

# Create account
# Email: admin@example.com
# Password: (la que pongas en .env)
```

## Features

| Feature | Status | V |
|---------|--------|---|
| Boolean flags | ✅ | 0.1 |
| Rules engine | ✅ | 0.1 |
| Rollouts (gradual) | ✅ | 0.1 |
| Segments | ✅ | 0.1 |
| Real-time propagation | ✅ | 0.1 |
| OpenTelemetry native | ✅ | 0.1 |
| Audit log | ✅ | 0.1 |
| Go SDK | ✅ | 0.1 |
| Python SDK | 🔄 | 0.2 |
| Webhooks | 🔄 | 0.2 |
| Scheduled flags | 🔄 | 0.2 |
| Multivariate flags | ⏳ | 1.0 |
| SAML/OIDC | ⏳ | 1.0 |
| Java SDK | ⏳ | 1.0 |

## How it works

```
1. Create a flag via dashboard or API
2. Add rules: "if user.country == 'AR' && user.plan == 'premium', enable"
3. Add rollout: "enable for 25% of matching users"
4. Your app evaluates: `sdk.IsEnabled(ctx, "checkout_v2", userContext)`
5. Everything is observable: see traces in Grafana, metrics in Prometheus
```

## Comparisons

| Feature | Flagstone | Unleash | Flipt | LaunchDarkly |
|---------|-----------|---------|-------|--------------|
| Simplicity | ⭐⭐⭐⭐⭐ | ⭐⭐⭐ | ⭐⭐⭐ | ⭐⭐ |
| OTel native | ✓ | ✗ | ✓ | ~ |
| Self-hosted | ✓ | ✓ | ✓ | ✗ |
| SaaS | ✓ | ✓ | ✗ | ✓ |
| Cost (free tier) | $0 | $0 | $0 | $0 |
| Cost (pro) | $79/mo | $150/mo | N/A | $490/mo |

## Getting started

- [Documentation](./docs/README.md)
- [API Reference](./docs/API.md)
- [SDK Guide](./docs/SDK.md)
- [Deployment](./docs/DEPLOYMENT.md)
- [Design decisions](./DESIGN.md)

## Community

- [Discord](https://discord.gg/...) – 🔥 Join us!
- [GitHub Discussions](https://github.com/thomas-vilte/flagstone/discussions)
- [Twitter](https://twitter.com/flagstone_dev)

## Roadmap

See [docs/ROADMAP.md](./docs/ROADMAP.md) for detailed plans.

**Next**:
- Python SDK (v0.2)
- Webhooks (v0.2)
- SAML/OIDC (v1.0)

## License

MIT – See [LICENSE](LICENSE)

---

Built with ❤️ by [thomas-vilte](https://github.com/thomas-vilte)
```

### Estrategia de tweets

**Día 1**: Launch (3 tweets)
```
1. "🚀 Introducing Flagstone..."
2. "Why Flagstone?..."
3. "Show HN live..."
```

**Semana 1**: Momentum (daily)
```
- Comparativa visual (Flagstone vs Unleash)
- "How we did OTel natively"
- Testimonial de user (si existe)
- "1,000 stars 🎉"
```

**Mes 1**: Regular cadence (3x/week)
```
- Blog links
- Feature highlights
- Community highlights
- "Help wanted" (issues para contributors)
```

**Mes 2+**: Consistency (2x/week)
```
- Educational (feature flags best practices)
- Product updates
- Community stories
```

### Blog strategy

**Post 0** (Pre-launch): "Introducing Flagstone"
- Problem statement
- Solution
- Why now
- Link a GitHub

**Post 1** (M1, Week 4): "Why We Built Flagstone"
- Competitive landscape
- Design philosophy
- Open source model

**Post 2** (M1, Week 6): "Python SDK for Flagstone"
- Code examples
- Async support
- Integration patterns

**Post 3** (M2, Week 10): "Feature Flags Best Practices"
- When to use flags
- Rollout strategies
- Common mistakes

**Post 4** (M2, Week 14): "OpenTelemetry for Feature Flags"
- Why OTel matters
- How Flagstone does it
- Dashboard examples

**Post 5** (M3, Week 20): "Flagstone v1.0: Enterprise Ready"
- SAML/OIDC
- Compliance
- SLA

**Cadencia**: 1 blog cada 3-4 semanas en M0-M3, después más frecuente

---

## Métricas y objetivos

### Éxito en cada fase

**Fase 0 (Pre-lanzamiento)**:
```
✓ 0 bugs críticos (testear todo)
✓ README y docs están claros
✓ CI/CD verde
✓ Docker Compose funciona end-to-end
```

**Fase 1 (Launch)**:
```
✓ 300+ stars en 48h
✓ Top 15 en HN
✓ 50+ comentarios constructivos en HN
✓ 0 crashes reportados
✓ 50+ Discord members
```

**Fase 2 (Cloud)**:
```
✓ 1,200+ stars total
✓ 100-200 Cloud signups
✓ 5-10 clientes pagos
✓ $400-800 MRR
✓ 100+ Discord members
✓ Trial → paid conversion > 5%
```

**Fase 3 (Aceleración)**:
```
✓ 2,500+ stars
✓ 400-600 signups totales
✓ 20-35 clientes pagos
✓ $1,600-2,800 MRR
✓ 200+ Discord
✓ Python SDK shipped
✓ 1+ press mention
```

**Fase 4 (Enterprise)**:
```
✓ 3,500+ stars
✓ 600-900 signups
✓ 40-60 clientes
✓ $3,200-4,800 MRR
✓ 5+ companies usando en producción
✓ 1+ community contributor activo
```

**Fase 5 (Consolidation)**:
```
✓ 5,500+ stars
✓ 1,300-1,800 signups
✓ 100-150 clientes
✓ $8,000-12,000 MRR
✓ Viable full-time
✓ Ready to hire engineer
✓ Ready for Series A (si querés)
```

### Dashboard de métricas

Revisas **cada lunes**:

```
GitHub:
  - Stars (goal: +10% week-on-week M1-M3)
  - Issues (goal: <5% unresolved > 48h)
  - PRs (goal: 1-2/week comunitarios)

Cloud:
  - Signups (goal: 50/week M1, 100/week M2)
  - Trial users (goal: 10% de signups)
  - Conversión (goal: 5% trial → paid M1, 7% M2)
  - MRR (goal: $400 week 4, $1k week 8, $3k week 12)
  - Churn (goal: <20% M1, <15% M2)

Community:
  - Discord members (goal: 50 M1, 200 M2, 500 M6)
  - Blog views (goal: 100 M1, 500 M2, 2k M6)
  - Twitter followers (goal: +100/mes)

Product:
  - API uptime (goal: 99.5% M1+)
  - p99 latency (goal: <20ms)
  - Error rate (goal: <0.1%)
```

### Redflags (problemas)

Si ves esto, algo está mal:

```
❌ Signups planateau en <50/mes (M2+)
   → Falta marketing. Incrementá blogs/content.

❌ Conversión trial → paid <2%
   → Falta value prop o pricing muy alto.

❌ GitHub issues acumulados (>50 sin responder)
   → Overwhelmed. Contrata help o foca en críticos.

❌ Churn >25% (M3+)
   → Product no satisface. Habla con clientes.

❌ 0 stars gain en 1 semana (después del launch)
   → Algo roto en el repo o docs no claros.

❌ MRR decreciendo 2+ semanas
   → Churn > new customers. Problema.
```

---

## Decisiones críticas

### Decision 1: Free tier generosity

**Opción A** (Launch aggressive):
```
Free: 100k evals/mes, 1 proyecto, 10 flags, 1 usuario
Pro: $79/mes

Ventaja: más adopción, comunidad crece
Desventaja: menos conversión a paid
```

**Opción B** (Launch restrictive):
```
Free: 10k evals/mes, demo only (no flags guardadas)
Pro: $79/mes

Ventaja: más conversión
Desventaja: menos adopción
```

**Decision**: **Opción A** en M0-M3. Después revisá y cambia si necesitás más conversión.

**Justificación**: en MVP, adopción > revenue. Una vez que tenés 1,000+ usuarios, podés restringir sin perder tracción.

---

### Decision 2: Levantar capital

**Scenario 1** (MRR < $2k en M3):
- Seguí bootstrapping si puedés
- O raise pequeño seed ($100-200k) para accelerate

**Scenario 2** (MRR $2-5k en M3):
- Viably bootstrapping
- No necesitás capital
- Contrata SDR freelance si quieres más sales

**Scenario 3** (MRR > $5k en M3):
- Ya tenes opción: bootstrapping o raise
- Series A es viable después (M12+)
- Tu decision: control vs growth speed

**Mi recomendación**: bootstrapping hasta $5k MRR. Después decide.

**Justificación**: VC dinero es rápido pero requiere unit economics perfectas. Mejor validar primero.

---

### Decision 3: Open source forever

**Sí**:
- Mantené MIT license
- Comunidad crece
- Trust es máximo
- Self-hosted siempre gratis

**Diferenciación** (cómo ganás money):
- Operaciones Cloud (managed service)
- Enterprise features (v1.0+):
  - SAML/OIDC
  - Compliance reports
  - SLA monitoring
  - Dedicated support

(No es "enterprise open source", es "open source core + paid SaaS")

**Decision**: **Sí, open source forever**.

**Justificación**: el modelo de GitLab, Sentry, Cal.com. Probado y funciona.

---

### Decision 4: SDKs multilenguaje

**MVP** (v0.1): Go SDK only
- Razón: Go es nuestro lenguaje
- Usuarios de otros lenguajes: usan OpenFeature provider (workaround)

**v0.2**: Python SDK
- Razón: Python es popular, requests bastantes

**v0.3**: Java SDK
- Razón: Java enterprise

**Post v1.0**: Ruby, PHP, .NET, Rust
- Razón: cuando tenemos community / capital

**Decision**: serial SDKs, no todo a la vez.

**Justificación**: cada SDK es ~2-3 weeks de trabajo. Mejor hacerlas bien que rápido + mal.

---

## Riesgos y contingencias

### Risk 1: Launch con bugs críticos

**Severidad**: Alta

**Mitigación**:
- Rigorous testing (unit + integration + e2e)
- Manual QA antes del launch
- Rollback plan (git revert, instant fix)

**Si pasa**:
- Pusheá fix rapidísimo
- Posteá en HN/Discord: "v0.1.1 fixes critical issue"
- No escondas el problema

---

### Risk 2: Baja adoption (< 50 signups Cloud en M2)

**Severidad**: Media

**Mitigación**:
- Content marketing agresivo
- Sales outreach early
- Gather feedback, improve product

**Si pasa**:
- Pivotar: maybe free tier es demasiado restrictivo?
- Revisar messaging: maybe "feature flags" no resuena?
- Habla con 10 no-adopters: por qué no lo usan?

---

### Risk 3: Competitor lanza algo similar (mejor)

**Severidad**: Baja (improbable)

**Mitigación**:
- Moverse rápido
- OTel nativo es hard to copy
- Comunidad es moat

**Si pasa**:
- Tomar como validación: "el mercado existe"
- Competir en simplicidad + cost

---

### Risk 4: AWS/Postgres va down

**Severidad**: Alta (para Cloud)

**Mitigación**:
- Multi-AZ RDS (M6+)
- Backups diarios
- Disaster recovery plan

**Si pasa**:
- Restore from backup (15-30 min)
- Comunicar a clientes
- Ofrecer credit si fue fault tuyo

---

### Risk 5: Burnout

**Severidad**: Alta (personal)

**Mitigación**:
- No trabajes 16h/día, es insostenible
- Sábado/domingo = off (ningun work)
- Miércoles early end (reset mental)
- Habla con amigos/mentores

**Si pasa**:
- Pause agresivo de features
- Focus: bugs + customer support only
- Contrata help

---

## Lo que NO haces en M0-M3

```
❌ Levantar VC (aún no)
❌ Contratar team grande
❌ Soportar 10 lenguajes de SDKs
❌ Custom integrations
❌ Enterprise 24/7 support
❌ Predictabilidad de SLA (no la tenés)
❌ Resignificación personal (aún)
```

---

## Rollout plan por semana

```
HOY - Semana 1:
  Termina MVP, testa todo, prepara comunicación

Semana 2:
  Final polish, prepara GitHub/Discord

Semana 3 (LAUNCH):
  Jueves: GitHub público
  Viernes: Reddit/Dev.to
  Fin de semana: monitorea

Semana 4:
  Cloud beta
  Sales outreach comienza

Semana 5-8:
  Aceleración, content, más SDKs

Semana 9-12:
  Enterprise focus, v1.0 planning

Semana 13-26:
  v1.0 + consolidación
```

---

## Final checklist pre-launch

```
GITHUB:
[ ] Repo públic, MIT license
[ ] README épico + screenshots
[ ] docs/ carpeta con todo
[ ] DESIGN.md linkeado
[ ] CONTRIBUTING.md presente
[ ] GitHub Issues templates
[ ] GitHub Discussions enabled
[ ] GitHub Actions CI corriendo verde

CODE:
[ ] Zero compiler warnings
[ ] Tests passing (unit + integration)
[ ] Linter clean (golangci-lint)
[ ] No secrets en repo (audit git history)
[ ] .env.example present
[ ] Docker builds clean

DEPLOYMENT:
[ ] docker-compose.yml works end-to-end
[ ] cloud.flagstone.dev prepped (not live yet)
[ ] Stripe account creado
[ ] Terraform módulos (opcional)
[ ] SSL certs ready

COMMUNICATION:
[ ] Twitter account created (@flagstone_dev)
[ ] Discord invite link ready
[ ] Blog post drafted
[ ] Show HN post drafted
[ ] Tweets prepped (10+)
[ ] LinkedIn post drafted
[ ] Email prepared (si tenés lista)

PERSONAL:
[ ] Dormiste bien
[ ] Comiste (no vivís de cafe)
[ ] Respirás (no estés ansioso)
[ ] Mentalidad: curiosidad, no perfección
```

---

## Recursos útiles

- [Show HN: Guidelines](https://news.ycombinator.com/showhn.html)
- [How to get HN frontpage](https://news.ycombinator.com/item?id=30166435)
- [Open source marketing](https://www.kartar.net/2016/01/what-made-mastodon-go-viral/)
- [SaaS metrics that matter](https://www.profitwell.com/blog/saas-metrics)

---

## Autor + contact

**Plan escrito por**: thomas-vilte

**Preguntas?** Abri un issue en GitHub o postea en Discord.

**Versión**: 1.0 (2024)

**Última actualización**: [HOY]

---

> **Nota final**: Este plan es vivo. Se actualiza según la realidad. Lo importante es ejecutar, aprender, iterar. Números son estimaciones. Variancia es normal. Juega para ganar pero disfruta el viaje.
