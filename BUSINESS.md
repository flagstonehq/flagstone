# Flagstone — Business Strategy

> This document describes the commercial strategy for Flagstone: distribution model, pricing, revenue expectations, and execution phases. It complements the technical docs ([DESIGN.md](./DESIGN.md), [SECURITY.md](./SECURITY.md)) by explaining *how this project becomes a sustainable business*.

---

## Table of Contents

1. [Distribution Model](#distribution-model)
2. [Why Dual-Distribution](#why-dual-distribution)
3. [OSS vs Cloud Boundary](#oss-vs-cloud-boundary)
4. [Pricing Strategy](#pricing-strategy)
5. [Revenue Expectations](#revenue-expectations)
6. [Execution Phases](#execution-phases)
7. [Operational Tradeoffs](#operational-tradeoffs)
8. [What Could Go Wrong](#what-could-go-wrong)

---

## Distribution Model

Flagstone is **dual-distribution**: the same codebase ships in two ways.

### Self-hosted (free, MIT license)

The user runs the server themselves on their own infrastructure (Docker, Kubernetes, bare-metal, their own AWS account, etc.). Full functionality, no telemetry, no license check.

Configuration: `FLAGSTONE_DEFAULT_PLAN=enterprise` — all tenants get unlimited usage.

### Flagstone Cloud (managed by us, paid)

We operate a multi-tenant deployment at `cloud.flagstone.dev`. Customers sign up, pay, never touch a server.

Configuration: `FLAGSTONE_DEFAULT_PLAN=free` — new tenants land on the free tier and upgrade explicitly. Plan limits enforced at the storage layer (see [DESIGN.md → Plan enforcement](./DESIGN.md#plan-enforcement-tenant-quotas)).

**The Cloud deployment is the same codebase + a thin overlay** (`cmd/flagstone-cloud/`, not in the OSS repo) containing Stripe webhooks, billing email templates, and cross-tenant admin tools. No feature divergence.

---

## Why Dual-Distribution

### The two alternatives are both worse

**Cloud-only** means:
- Smaller addressable market (companies with data-residency / compliance constraints are out)
- Competing with LaunchDarkly head-on without their resources, track record, or sales team
- 24/7 ops burden on a solo dev
- Slow validation: no users until someone pays

**OSS-only** means:
- No recurring revenue
- Support contracts are hard to sell without a reputation
- Same marketing effort as Cloud, no monetization upside
- Burnout-driven path

Dual-distribution avoids both failure modes.

### It's a proven pattern for solo devs / tiny teams

| Product | Model | Notable outcome |
|---|---|---|
| **Plausible Analytics** | OSS + Cloud | $1M+ ARR (publicly reported), team of 2 |
| **Cal.com** | OSS + Cloud | Series A funding, thousands of self-hosters |
| **Sentry** | OSS + Cloud | $100M+ ARR, started OSS-first |
| **PostHog** | OSS + Cloud | Y Combinator → decacorn track |
| **Supabase** | OSS + Cloud | $80M+ ARR |

The pattern is consistent: critical infrastructure tools succeed when they're OSS-first with a Cloud option for those who don't want to operate them.

### The "no lock-in" argument is a feature

For critical infrastructure (and feature flags **are** critical — they control production behavior), the ability to self-host is a *sales argument*, not a weakness. It removes the biggest objection enterprise customers have to SaaS-only vendors.

> "If your Cloud has issues, we can pull our data and run it ourselves." — every CISO ever.

LaunchDarkly cannot offer this. We can.

### The OSS distribution is free marketing

Every self-hoster who lists Flagstone in an awesome-go README, writes a blog post, or recommends it in a Slack community is unpaid marketing. The customer acquisition cost (CAC) for Cloud signups that came via OSS adoption is effectively zero.

### Validation is cheaper

You don't need someone to pay to know your product solves a real problem. GitHub stars, issues opened, and Show HN comments give you signal months before paid customers.

---

## OSS vs Cloud Boundary

The classic mistake of OSS+Cloud companies is gating *features* behind Cloud. The community detects this immediately and resentment builds (Sentry, GitLab, Elastic have all faced backlash for this).

### The rule

**OSS gets every feature. Cloud sells "not operating it."**

When deciding where something belongs, ask: *is this product functionality, or is this "someone runs it"?* Only the second category is Cloud-only.

### Concrete boundary

| Category | OSS | Cloud-only |
|---|---|---|
| Rule engine, all operators, segments, rollouts | Yes | (same code) |
| All SDKs (Go, TS, Python, etc.) | Yes | (same code) |
| Multi-environment, multi-tenant architecture | Yes | (same code) |
| RBAC, JWT, API keys | Yes | (same code) |
| SSE streaming | Yes | (same code) |
| OpenTelemetry traces & metrics | Yes | (same code) |
| Webhooks | Yes | (same code) |
| Audit log | Yes | (same code) |
| Dashboard web UI | Yes | (same code) |
| **SSO / SAML / OIDC** | Yes (self-hostable) | Pre-configured, supported |
| Database, Redis | self-managed | Managed by us |
| Backups, multi-AZ | self-managed | Managed by us |
| Monitoring, alerting | self-managed | Managed by us |
| SLA-backed uptime | none | 99.9% (Pro), 99.95% (Enterprise) |
| Stripe billing, plan enforcement runtime | disabled | enabled |
| Email/Slack support | community only | included |
| Pre-built Grafana dashboards (hosted) | configure yourself | included |
| Cross-tenant admin tooling | not applicable | Cloud-internal |

### Why include SSO in OSS

Putting SSO behind a paywall is the single most common "OSS feels betrayed" trigger. SSO is a security feature; everyone deserves it. The OSS version supports SAML 2.0 and OIDC out of the box. **What Cloud sells is not having to configure it.**

This is a deliberate decision that costs short-term revenue (we can't upsell SSO to free-tier Cloud users) but builds long-term trust. Trust is the moat.

---

## Pricing Strategy

### The pricing metric

For feature flag servers, the natural usage metric is **evaluations per month**. It scales with the value extracted from the product. Per-flag or per-environment pricing is artificial and frustrating.

Combine with **seats** (limits team size in the dashboard) and **tier features** (SSO, SLA, retention).

### Suggested tiers

> These are illustrative. Final numbers should be set after the first 10 paying customers reveal what people actually pay for.

| Tier | Price/mo | Evaluations | Seats | What you get |
|---|---|---|---|---|
| **Free** | $0 | 100,000 | 1 | 1 project, 10 flags, community support |
| **Hobby** | $9 | 500,000 | 3 | 3 projects, 50 flags, email support, 30d audit |
| **Team** | $39 | 5,000,000 | 10 | Unlimited projects, 200 flags, webhooks, 90d audit |
| **Pro** | $149 | 50,000,000 | Unlimited | Unlimited flags, priority support, 99.9% SLA, 1y audit |
| **Enterprise** | Custom | Unlimited | Unlimited | SSO/SAML, dedicated support, on-prem option, 2y+ audit, custom SLA |

### Why Hobby at $9 is critical

The Hobby tier is the **easy yes** between Free and Team. It converts:
- Solo devs who don't want to run Postgres
- Side-project authors who outgrew Free
- Small bots / Discord apps with modest evaluation volume

It's also the cheapest way to validate that someone will pay *anything* for the Cloud version. If nobody pays $9, the product-market-fit signal is weak. If many people pay $9, you have a base to upsell from.

### Annual discount

Standard 2-month-free for annual prepay (~17% discount). Improves cash flow and reduces churn.

### No "contact us" for anything under Enterprise

Self-serve signup → self-serve upgrade → Stripe handles everything until Enterprise. Solo dev cannot run a sales motion for low-ticket sales; pricing has to be visible and frictionless.

---

## Revenue Expectations

Brutally honest projections for a solo dev executing competently:

### Year 1: $0 – $500/mo

Goals:
- Build the OSS to v1.0
- Show HN successful (500+ upvotes)
- 500+ GitHub stars
- First 5-10 paying Cloud customers (mostly Hobby tier)

Most of this year is **building reputation, not revenue**. If you're chasing revenue in Year 1, you're optimizing the wrong thing.

### Year 2: $500 – $5,000/mo

Goals:
- 2,000+ GitHub stars
- Active community (Discord, GitHub Discussions)
- First Team-tier customers
- First Enterprise lead (probably one of the self-hosters at a larger company)
- Content cadence: 1-2 technical blog posts per month

This is where the model proves itself or doesn't. If Year 2 ends with <$2k MRR, the strategy needs adjustment.

### Year 3: $5,000 – $20,000/mo

Goals:
- 50-200 paying Cloud customers
- 1-3 Enterprise contracts ($1-5k/mo each)
- Possibly first hire (part-time support / dev rel)

At this point Flagstone can be a sustainable solo income or the basis for a small team.

### Year 5+: scaling decisions

If MRR is $30k+/month:
- Hire support/dev rel (10-20 hours/week)
- Consider raising capital to accelerate (or stay bootstrapped)
- Expand into adjacent products (experimentation platform, config management)

If MRR is <$10k/month and growth has flattened:
- Reassess product-market fit
- Reduce time investment, keep maintaining
- Consider acquisition / acquihire as exit

---

## Execution Phases

### What each milestone unlocks (use-case timeline)

The roadmap milestones don't just produce features — they unlock specific use cases for you. Knowing this lets you start *using* Flagstone yourself long before you'd ever ship it to a paying client.

| Milestone | Done at | What you can do |
|---|---|---|
| **M1** | ~Week 4 | Self-host an instance for **your own personal projects**. CRUD via API + Go SDK. No web UI yet. Perfect for: the Discord bot example, side projects, internal tools. |
| **M2** | ~Week 8 | Self-host for **friends and trusted users**. Web dashboard works, SDK has cache + SSE. Stable enough to recommend to people you know personally. |
| **M3** | ~Week 14 | Self-host for **the OSS public release**. OTel native, full security hardening, Terraform deploy. Strangers on the internet can deploy it safely. |
| **Phase 2** | Month 6-9 | Cloud **private beta** — you host it for ~10 early users who asked. Free in exchange for feedback. NOT for paying customers yet. |
| **Phase 3** | Month 9-12 | Cloud **public launch** — anyone can sign up at `cloud.flagstone.dev` and pay. This is when "clients" becomes a real word. |

The key insight: **you can use Flagstone for your own stuff after Week 4, even if Cloud doesn't exist for another 8+ months.** The dogfood loop starts immediately.

> **Terminology note**: "Flagstone Cloud" (capitalized, the product) refers to the future managed SaaS at `cloud.flagstone.dev` that *you* will operate for paying customers — available Phase 3 (~month 9-12).
>
> "Self-hosting Flagstone on AWS" (or any other infra) means running the OSS code in your own AWS account. Even though it physically runs in AWS's cloud, it's not "Flagstone Cloud" the product. It's available from Week 4 onward via the `deploy/terraform/` modules.
>
> When your Discord bot (or any deployed app) needs to call Flagstone, the server has to be reachable from wherever the bot runs. For an AWS-hosted bot, the simplest path is to deploy Flagstone in the same AWS VPC — `terraform apply` from `deploy/terraform/` gives you exactly this. Total cost during the free tier: $0.

### Phase 1 (Months 1-6): Build the OSS — Cloud is vaporware

- Implement Milestones 1 + 2 from the [README roadmap](./README.md#roadmap)
- **You self-host your own Flagstone instance** from week 4 onward and use it on your other projects (this is how you find bugs early)
- Public GitHub repo with good README
- One technical blog post per month about the implementation
- Active responses to early GitHub issues

**Do not launch Cloud during this phase.** A broken Cloud during early reputation-building kills the whole thing. But **do** run your own personal instance — that's how you find the bugs before public users do.

### Phase 2 (Months 6-9): Cloud Private Beta

- Implement Stripe integration + plan enforcement runtime
- Invite 5-10 self-hosters who asked for managed: "I'll host it for you free for 3 months in exchange for feedback"
- Iterate on the onboarding flow until first-flag-evaluated takes < 5 minutes
- Run Cloud in production for 90 days without paid customers to shake out the operational bugs
- **You can offer free Cloud accounts to friends and personal projects** to expand the test population — explicitly framed as "free beta", not "production-ready service"

### Phase 3 (Months 9-12): Public Launch

- Show HN with the Cloud option
- Public pricing on the website
- Self-serve signup
- Hobby + Team tiers live, Pro available via email request, Enterprise via "contact us"
- **Now you can take paying clients** — the SLA, support commitments, and operational quality have been validated by 6 months of private beta

### Phase 4 (Year 2): Growth

- Content marketing (blog, YouTube, conference talks)
- SDK in additional languages (TypeScript, Python — see [README roadmap](./README.md#roadmap))
- OpenFeature provider (CNCF standard) — a passive distribution channel
- First Enterprise sales call (probably inbound from a self-hoster)

### Honest framing of "personal projects" vs "paying clients"

These are very different use cases with very different risk profiles:

| Use case | When safe to do | Why |
|---|---|---|
| **Your own side projects (your code, your data)** | After M1 (~week 4) | You bear the risk. If it breaks, you fix it. |
| **Friends' projects (their code, your hosting)** | After M2 (~week 8) | Web UI makes them self-sufficient; bugs are embarrassing but not catastrophic |
| **OSS users self-hosting (their code, their hosting)** | After M3 (~week 14) | They take operational risk; you just need the code to be correct |
| **Free Cloud beta users** | After Phase 2 starts (~month 6) | You take operational risk but no SLA promised |
| **Paying clients on Cloud** | After Phase 3 launch (~month 9-12) | You take operational risk AND SLA — must be ready |

**The trap to avoid**: charging clients before Phase 3. Once someone pays $9 for Hobby tier, they have legitimate expectations of uptime, support, and feature stability. Setting those expectations before you can meet them burns reputation that's hard to rebuild.

The corollary: **start using Flagstone yourself as soon as possible** (week 4). The earlier you eat your own dog food, the more bugs you catch before paying customers do.

---

## Operational Tradeoffs

Going dual-distribution has real costs. Acknowledging them up front:

### Community support is non-trivial

Expect **20-30% of your time** going to:
- Answering GitHub issues
- Reviewing community PRs
- Triaging bug reports
- Updating docs based on user confusion
- Discord/Slack community moderation

This is non-optional. If you ignore the community, the OSS reputation dies and the Cloud has no funnel.

### Cloud has to be excellent

Self-hosters tolerate bugs ("I can fix it myself"). Cloud customers do not. Any downtime burns the reputation built by the OSS. Operational excellence in Cloud is the price of admission.

Specifically:
- Status page (status.flagstone.dev) from day 1 of public launch
- Postmortems for any incident > 5 min of downtime
- 99.9% uptime SLA on Pro tier means **< 43 min of downtime per month** — operationally serious

### Marketing requires consistent output

OSS doesn't grow by itself. You need:
- 1-2 technical blog posts per month
- Tweet/post about releases
- Comments on relevant HN/Reddit/Lobste.rs threads (when genuinely helpful, not spammy)
- Conference talks once you have traction

If writing is not your strength, this is the part that will hurt.

### Year 1 is mostly unpaid

Plan for 12 months of zero revenue. Have either savings or a day job. The model works, but it doesn't work fast.

---

## What Could Go Wrong

Honest risks:

| Risk | Likelihood | Mitigation |
|---|---|---|
| **Nobody pays for Cloud** | Medium | Hobby tier at $9 lowers the bar; if even that doesn't convert, the product is mispriced or wrong |
| **Flipt v2 reverses to API-first** | Low | Even if they do, they still have Git-native as primary — different audience |
| **LaunchDarkly drops prices aggressively** | Low | Their cost structure prevents this; they monetize big enterprise, not small teams |
| **CNCF standardizes around OpenFeature in a way that commoditizes us** | Medium-positive | We support OpenFeature; commoditization helps OSS underdogs |
| **Solo-dev burnout** | High | Set hard limits on community time; say no to feature requests that don't fit; charge for custom work |
| **OSS gets popular but Cloud doesn't** | Medium | This is fine for 12 months; if it persists, raise prices and lean into Enterprise contracts |
| **First Cloud outage drives churn** | Medium | Operational excellence from day 1 of public launch; status page; transparent postmortems |

---

## TL;DR

- **Dual-distribution** is the right model. Don't pick one or the other.
- **OSS is full-featured**, including SSO. Cloud sells *not operating it*.
- **Pricing**: usage-based (evaluations/month) + seats + tier features. Hobby tier at $9 is critical.
- **Year 1 is plantation, not harvest.** Expect $0-500 MRR. Year 3 target: $5k-20k MRR.
- **20-30% of time on community** is the cost of OSS adoption. Accept it.
- **Cloud quality has to be excellent** from day one of public launch. There's no "early-stage forgiveness" for paid SaaS.
