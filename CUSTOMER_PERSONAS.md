# CUSTOMER_PERSONAS.md: Quién compra Flagstone y por qué

## 1. CUSTOMER PERSONAS

### Persona 1: Alex - El Dev Indie

**Demographics:**
```
Nombre: Alex Rodriguez
Edad: 26
Rol: Founder/Developer
Company: Side project (Discord bot)
Team size: 1 (solo dev)
Location: Buenos Aires
Experience: 3 años coding
```

**El proyecto:**
```
Discord bot que plays music, manages playlists
Usuarios activos: 5K servidores
Revenue: $0 (hobby project)
Presupuesto: $0/mes
```

**Pain points:**
```
1. "Deploying changes without downtime is scary"
   - Cambias código → deploy → todos los usuarios ven cambio
   - Si algo rompe → downtime immediato
   
2. "Can't afford tools"
   - LaunchDarkly: $300/mes = $3.6K/año
   - Alex ingreso: $200/mes from Patreon
   - LaunchDarkly = 18 months de income 😱
   
3. "Kill switch for bugs"
   - Bot has 5K servers
   - If you deploy bug → all 5K affected instantly
   - Rollback requires redeploy (15+ min)
   - Users leave angry
```

**Goals:**
```
✅ Deploy features without downtime
✅ Kill switch for bugs (revert en segundos)
✅ Don't want to spend money
✅ Something simple, no ops overhead
```

**Flagstone fit:**
```
✅ FREE forever (MIT license, self-hosted)
✅ Setup 5 min: docker-compose up
✅ Dashboard web (no CLI joda)
✅ Kill switch: toggle flag, change applies < 100ms
✅ Go-based: lightweight, Alex appreciates
```

**How to reach:**
```
→ GitHub trending
→ Show HN
→ r/golang, r/discordapp
→ Discord servers
→ Side project communities
```

**Messaging:**
```
"Deploy your Discord bot confidently.
Feature flags in 5 minutes, free forever.
Kill switch for bugs. Rollback in seconds."
```

---

### Persona 2: María - El Product Manager (SMB)

**Demographics:**
```
Nombre: María López
Edad: 32
Rol: VP Product
Company: E-commerce SaaS (30 personas)
Team: 1 PM, 1 PM junior, 1 design lead
Location: LATAM
Budget authority: Sí
```

**Pain points:**
```
1. "Can't move fast with engineering approval"
   - Quiero cambiar checkout flow (A/B test)
   - Tengo que pedir a engineering
   - "Tomorrow, we're busy"
   - 1-2 days delay
   - Competitors te pasaron
   
2. "LaunchDarkly is expensive"
   - Actual cost: $15K/año
   - Feels like "hidden tax" on product team
   - CFO asks questions
   
3. "No visibility into what's live"
   - "Is the checkout experiment still running?"
   - Asks slack, gets 3 different answers
```

**Goals:**
```
✅ Autonomy (change flags without asking engineers)
✅ Speed (go from idea to test in hours)
✅ Cost (not break product budget)
✅ Confidence (know what I'm changing)
```

**Flagstone fit:**
```
✅ $39/mes Team tier (vs $15K/año with LD!)
✅ Dashboard: PM-friendly, no code required
✅ Instant changes: no waiting for engineering
✅ RBAC: engineers still have visibility
✅ No vendor lock-in
```

**Key messaging:**
```
"Empower your product team.
Launch experiments in hours, not weeks.
$39/month. 40x cheaper than LaunchDarkly."
```

---

### Persona 3: Carlos - El DevOps Engineer

**Demographics:**
```
Nombre: Carlos Martínez
Edad: 35
Rol: Senior Platform Engineer
Company: FinTech (200 personas)
Team: Platform team (8 people)
```

**Pain points:**
```
1. "No observability into flag changes"
   - Activás flag → latency increases
   - "What changed? Let me check logs"
   - 30+ min debugging
   - Could've fixed by then
   
2. "Canary deployments are manual"
   - Start at 5%
   - Monitor metrics
   - Manually ramp to 25%
   - Someone has to be watching
   - At 3 AM? Nobody's watching
```

**Goals:**
```
✅ Observability: trace each flag eval
✅ Automation: canary without manual steps
✅ API-first: scriptable, no UI clicking
✅ Transparency: audit trail
```

**Flagstone fit:**
```
✅ OTel nativo: traces on every evaluation
✅ REST API: fully scriptable
✅ Webhooks: integrate with CI/CD
✅ Self-hosted: data never leaves VPC
```

**Key messaging:**
```
"Observability for feature flags.
Trace every evaluation, debug faster.
Platform engineers: this is for you."
```

---

### Persona 4: Patricia - El CISO (Enterprise)

**Demographics:**
```
Nombre: Patricia González
Edad: 45
Rol: Chief Information Security Officer
Company: Insurance company (500+ personas)
Budget: $10M+/year for infrastructure
```

**Pain points:**
```
1. "Where is our data with LaunchDarkly?"
   - LD: "In our data centers"
   - Patricia: "Which ones? Where exactly?"
   - LD: "US region"
   - Patricia: "Is that GDPR compliant?"
   → Deal blocker
   
2. "Who can change flags? Who approved?"
   - Need: Segregation of duties
   - Nobody knows who authorized
   - Compliance audit: fail
```

**Goals:**
```
✅ Data control: flags stay in our VPC
✅ Compliance: SOC2, HIPAA-ready
✅ Audit trail: immutable
✅ No lock-in: could migrate if needed
```

**Flagstone fit:**
```
✅ Self-hosted option: data in YOUR VPC
✅ Audit log immutable: DB-enforced
✅ Change approvals: workflow before activation
✅ MIT license: no vendor lock-in
✅ Cost: $50K/año vs LD $300K+
```

**Key messaging:**
```
"Security through control.
Self-hosted feature flags.
Data stays in your VPC. Compliance-ready."
```

---

## 2. MESSAGING BY CHANNEL

### Twitter/X

```
🎯 Devs indie:
"Deploy your side project confidently.
Feature flags in 5 min, free forever.
Kill switch for bugs. Rollback in seconds."

🎯 SMBs:
"Paying $50K/year for feature flags?
We built Flagstone.
Same features. $468/year.
40x cheaper. Self-hosted or Cloud."

🎯 DevOps:
"Your flag is on. Latency spiked. What broke?
LaunchDarkly: black box
Flagstone: Open Grafana. See trace. Done.
OTel native. Self-hosted."

🎯 Enterprise:
"Self-hosted. SOC2-ready. Data in YOUR VPC.
99.9% SLA. Change approval workflows.
No vendor lock-in. $50-100K/year.
Book a call: [link]"
```

### Reddit

**r/golang:**
```
Title: "We built Flagstone: Open-source feature flags in Go"
Pitch: Lightweight, API-first, OTel native.
Link: GitHub repo
```

**r/devops:**
```
Title: "Feature Flags + OTel: Better observability for production"
Pitch: Integrated OTel on every eval. Trace everything.
Link: GitHub
```

**r/startups:**
```
Title: "Leaving LaunchDarkly saved us $30K/year"
Pitch: Cost breakdown. Migration path. Lessons learned.
Link: Blog post
```

### LinkedIn (Sales)

**For Product Managers:**
```
Hi Maria,

I noticed your team uses LaunchDarkly for feature flags.

Most SMBs pay $12-15K/year for LD.
Flagstone does the same for $468/year.

Better yet: Your product team gets autonomy
(change flags from UI, no engineer approval).

Plus: OTel traces show what each flag does.

Free trial: https://flagstone.dev/try

Let me know if curious.
```

**For DevOps Leaders:**
```
Hi Carlos,

Platform team struggling with flag-related incidents?

With Flagstone's OTel native tracing:
- See exactly what flag caused the issue
- Traces show user context, rule matched
- Rollback in seconds

Webhooks + Terraform + Helm included.

Interested in a demo?
```

---

## 3. OBJECTION HANDLING

### Objection: "LD is proven at scale, you're new"

**Response:**
```
True. LD has been around longer.

But: do you need thousands-of-customers-level complexity?

Here's what most teams actually need:
✅ Boolean flags
✅ Targeting
✅ % rollout
✅ Rollback

Flagstone has all of that.

What most teams DON'T need:
❌ Advanced A/B testing w/ statistical significance
❌ Analytics integration
❌ $300K/year price tag

For your use case, Flagstone is "proven enough."

Plus: Open source (MIT).
If we disappear, you own the code.
With LD: you're stuck.
```

### Objection: "Self-hosted means ops"

**Response:**
```
True: self-hosted has ops overhead.

BUT: we designed Flagstone to minimize it.

Flagstone = Postgres + Redis + Go binary
  → All standard, simple infrastructure

Ops overhead: 2-4 hours/month
  → vs LD "no ops": but costs $15K/year

Trade: 2-4h/mo labor vs $15K/year

For most teams: worth it.

Plus: Cloud option coming (month 9).
  → Managed by us, you don't operate
  → $39-149/mo (vs LD $1,250/mo)
  → Still save money, zero ops
```

---

## 4. COMPETITIVE BATTLE CARDS

### When competing vs LaunchDarkly

**Our strength:**
```
✅ Price: 40-100x cheaper
✅ Observability: OTel native (LD doesn't have)
✅ Control: Self-hosted, MIT
✅ Speed: Setup in 5 min
✅ No lock-in
```

**Their strength:**
```
✅ Scale: Proven at thousands
✅ Support: Dedicated teams
✅ Analytics: Built-in
```

**Our message:**
```
"Same power, 40x cheaper, better observability."
"For teams that don't need (or want to pay for) LaunchDarkly."
```

