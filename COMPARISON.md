# Flagstone — Panorama competitivo

> Comparación honesta de Flagstone contra el mercado de feature flags. Este documento es de uso **interno/estratégico**: las cifras de competidores son aproximadas y cambian; verificá precios y licencias antes de citarlos en público, y **nunca publiques quotes o case studies que no sean reales.**
>
> Estado del conocimiento: principios de 2026. Las licencias y precios de los competidores cambian seguido — tratá esto como un mapa, no como una fuente citable.

---

## 0. La corrección de altitud

El error del análisis viejo era comparar **solo contra LaunchDarkly**. LD es el gigante caro: fácil de ganarle en precio, pero **casi nadie que busca algo barato está eligiendo entre Flagstone y LD.** La competencia real de Flagstone son las alternativas OSS y freemium que ocupan exactamente el mismo nicho.

Hay dos frentes distintos, con argumentos distintos:

- **Frente "precio" (vs LaunchDarkly, Split):** ganás por costo y simplicidad. Argumento fácil.
- **Frente "OSS / gratis" (vs Unleash, Flagsmith, GrowthBook, PostHog, Statsig, ConfigCat):** acá "más barato" **no aplica** — ellos también son gratis o casi. La cuña es otra: **OTel nativo + verdaderamente sin gating de features (incluido SSO) + Go liviano + simplicidad.**

---

## 1. El mapa del mercado

| Producto | Licencia | Modelo | Foco | Amenaza para Flagstone |
|---|---|---|---|---|
| **LaunchDarkly** | Propietario | SaaS (enterprise) | Flags + experimentación, maduro, caro | 🟢 Baja — distinto segmento (enterprise con presupuesto) |
| **Unleash** | Open-core (Apache 2.0 core; features pagas cerradas) | OSS + Cloud + self-host | Flags, muy maduro, self-host serio | 🔴 **Alta** — es "el Flagstone que ya existe hace 10 años" |
| **Flagsmith** | Open-core (BSD-3 core; enterprise cerrado) | OSS + Cloud + self-host | Flags + remote config, tu mismo playbook | 🔴 **Alta** — modelo idéntico al tuyo, ya ejecutado |
| **GrowthBook** | Open-core (MIT core; enterprise cerrado) | OSS + Cloud + self-host | **Experimentación** + flags, motor stats fuerte | 🟡 Media — te gana en A/B testing |
| **PostHog** | MIT + carpeta `ee/` propietaria | Cloud + self-host | Analytics + flags **incluidos gratis** | 🔴 **Alta** — "ya lo tengo gratis con mi analytics" |
| **Statsig** | Propietario | SaaS (free tier enorme) | Experimentación + flags, data-heavy | 🟡 Media — free tier brutal capta indies (ver nota) |
| **ConfigCat** | Propietario | SaaS (free tier generoso) | Flags simples, barato | 🟡 Media — simple y barato, sin OTel ni self-host |
| **Split (Harness)** | Propietario | SaaS (enterprise) | Experimentación, parte de Harness | 🟢 Baja — enterprise |
| **OpenFeature** | CNCF (estándar, no producto) | Spec + SDKs | Estandariza la capa SDK | 🟢 Positivo — soportarlo te beneficia |

> **Nota Statsig:** fue adquirida (OpenAI, 2025). Eso puede reorientar su foco hacia uso interno y enfriar su empuje en el mercado general de flags — oportunidad para vos, pero verificá el estado actual antes de apoyarte en este punto.

---

## 2. El insight central: tu enemigo no es el precio de LD, es el *open-core gating*

Unleash, Flagsmith y GrowthBook son OSS — pero **open-core**: el núcleo es abierto y **gatean detrás de pago las cosas que más duelen**: SSO/SAML, RBAC avanzado, project-level permissions, audit retention, change requests. El self-hoster gratis choca con un paywall justo cuando su equipo crece.

**Ahí está tu cuña diferencial, y es la misma jugada de confianza que hizo Plausible:**

> "Todo es OSS de verdad — incluido SSO, MFA, RBAC y change approvals. No gateamos seguridad. Cloud cobra por no operarlo, no por desbloquear features."

Esto es defendible, verificable y emocionalmente potente para la comunidad. Es un argumento que **no podés usar contra LD** (cerrado) pero que es demoledor contra Unleash/Flagsmith/GrowthBook, que son tu competencia real. Combinado con **OTel nativo** (que ninguno de ellos trae de fábrica), tenés dos ejes claros de diferenciación.

---

## 3. Matriz de features

Leyenda: ✅ sí · 🟡 parcial / via integración · 🔒 detrás de paywall (open-core) · ❌ no · ⏳ en roadmap

| Feature | Flagstone | Unleash | Flagsmith | GrowthBook | PostHog | LaunchDarkly |
|---|---|---|---|---|---|---|
| Boolean flags | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| Targeting / segments | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| % rollout | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| Reglas AND/OR/NOT | ✅ | ✅ | ✅ | ✅ | 🟡 | ✅ |
| Real-time push (SSE/streaming) | ✅ | 🟡 (polling) | 🟡 | 🟡 | 🟡 | ✅ |
| **OTel nativo (trace por eval)** | ✅ | ❌ | ❌ | ❌ | 🟡 | 🟡 |
| Self-hosted | ✅ | ✅ | ✅ | ✅ | ✅ | ❌ |
| SSO/SAML/OIDC | ✅ open | 🔒 pago | 🔒 pago | 🔒 pago | 🔒 `ee/` | ✅ pago |
| RBAC | ✅ open | 🟡/🔒 | 🟡/🔒 | 🔒 | 🔒 | ✅ pago |
| MFA (TOTP/WebAuthn) | ⏳ M4 open | 🔒 | 🟡 | ❌ | 🟡 | ✅ |
| Change approvals | ⏳ M4 open | 🔒 | 🔒 | ❌ | ❌ | ✅ pago |
| Audit log | ✅ | 🟡/🔒 | ✅ | 🟡 | ✅ | ✅ |
| Webhooks | ⏳ M4 | ✅ | ✅ | ✅ | ✅ | ✅ |
| Scheduled rollouts | ⏳ M3 | 🟡 | 🟡 | 🟡 | ❌ | ✅ |
| Shadow mode | ⏳ M3 | ❌ | ❌ | 🟡 | ❌ | ❌ |
| Terraform provider | ⏳ M5 | ✅ | ✅ | 🟡 | ✅ | ✅ |
| Helm chart | ⏳ M5 | ✅ | ✅ | ✅ | ✅ | ❌ |
| OpenFeature provider | ⏳ | ✅ | ✅ | ✅ | 🟡 | ✅ |
| **Experimentación / A/B con stats** | ⏳ M5 / ❌ | 🟡 | 🟡 | ✅ **fuerte** | ✅ | ✅ **fuerte** |
| Analytics integrado | ❌ | ❌ | ❌ | 🟡 | ✅ **nativo** | 🟡 |
| SDKs maduros multi-lenguaje | ⏳ (Go primero) | ✅ muchos | ✅ muchos | ✅ | ✅ | ✅ muchos |
| Mobile SDKs | ⏳ | ✅ | ✅ | 🟡 | ✅ | ✅ |

**Lectura honesta:** hoy Flagstone está **por detrás en madurez y cobertura** (muchos ⏳). Donde ya ganás o vas a ganar: **OTel nativo, no-gating de seguridad, real-time de fábrica, simplicidad/Go.** Donde claramente perdés: **experimentación con stats, breadth de SDKs, madurez.** No lo escondas: posicionate donde sos fuerte, no donde sos débil.

---

## 4. Las brechas reales de Flagstone (lo que falta para competir)

Ordenadas por impacto en adopción:

1. **SDKs multi-lenguaje (TS/JS y Python).** Bloqueante absoluto. Con solo Go, el ~80% de los equipos no te puede adoptar aunque quiera. Es la prioridad #1 después del Go SDK.
2. **OpenFeature provider.** Tu canal de distribución pasivo más barato y bajás el riesgo de adopción ("sos intercambiable"). Subilo en la prioridad — hoy está en Phase 4, debería estar antes.
3. **Experimentación / métricas por variante.** GrowthBook y Statsig lo regalan. "Flags sin experimentos" ya se siente incompleto en 2026. No necesitás un motor bayesiano completo el día 1, pero sí métricas de conversión por variante.
4. **Webhooks + Terraform + Helm.** Tu persona DevOps (Carlos) los espera de fábrica. Están en M3-M5; ok, pero son tabla de apuesta para enterprise.
5. **Madurez / pruebas en producción.** No se compra, se gana con tiempo. Por eso el dogfooding (tu bot) y la beta privada de 90 días son críticos.

---

## 5. Precio (honesto y conservador)

> Los precios de la competencia cambian seguido y dependen de evals/seats/features. Estas son **bandas aproximadas**, no cifras citables. El argumento de precio es genuino, pero no lo exageres: un número inflado que alguien refuta te hace perder toda la discusión.

| Escenario | Flagstone Cloud | Unleash/Flagsmith Cloud | LaunchDarkly | Statsig/ConfigCat |
|---|---|---|---|---|
| Indie / side-project | $0 self-host · $9 Hobby | free tier / self-host | sin tier real para indies | free tier generoso |
| SMB (~5M evals, 10 seats) | ~$39/mes | ~$80–150/mes (con paywalls de SSO) | varios cientos a ~$1k+/mes | $0–cientos |
| Growth (~50M evals) | ~$149/mes | varios cientos/mes | miles/mes | depende |
| Enterprise | custom (self-host = $0 + ops) | custom | el más caro del mercado | custom |

**El argumento de precio honesto:**
- Contra **LaunchDarkly**: sos dramáticamente más barato. Verdad, defendible.
- Contra **Unleash/Flagsmith**: el precio es parecido, pero **vos no cobras extra por SSO/RBAC** (ellos sí, vía open-core). Ese es el ahorro real y honesto.
- Contra **PostHog/Statsig/ConfigCat**: el precio **no es** tu argumento — es OTel, self-host con datos en tu VPC, y no estar atado a su plataforma de analytics.

---

## 6. Cuándo elegir cada uno (battle card)

**Elegí Flagstone si:** querés observabilidad real de cada evaluación (OTel nativo), querés todo open de verdad incluido SSO sin paywall, valorás simplicidad y un binario Go liviano, querés self-host con datos en tu VPC, y no necesitás un motor de experimentación pesado.

**Elegí Unleash/Flagsmith si:** necesitás un OSS ultra-maduro hoy, con SDKs en todos los lenguajes y años de producción — y no te molesta pagar por SSO/RBAC cuando crezcas.

**Elegí GrowthBook si:** experimentación con significancia estadística es tu caso de uso central, no los flags.

**Elegí PostHog si:** ya querés product analytics y los flags "gratis incluidos" te alcanzan.

**Elegí LaunchDarkly/Split si:** sos enterprise con presupuesto, necesitás "proven at scale" y soporte dedicado 24/7, y el costo no es restricción.

---

## 7. Conclusión estratégica

- **Reposicioná el discurso:** dejá de pelear solo contra LD (segmento que no es tuyo) y enfrentá a **Unleash/Flagsmith/GrowthBook/PostHog**, que ocupan tu nicho.
- **Tus dos cuñas defendibles:** (1) **OTel nativo** — nadie más lo trae de fábrica; (2) **cero gating de seguridad** — todo open incluido SSO, que es justo donde los open-core decepcionan.
- **Cerrá las brechas en este orden:** SDK TS/Python → OpenFeature → experimentación básica.
- **No infles números ni inventes testimonios.** Tu único moat de largo plazo es la confianza; un dato falso descubierto la destruye de una.
- **El tiempo juega a tu favor solo si dogfooded.** Usá Flagstone en tu propio bot desde ya: es tu primer case study honesto.
