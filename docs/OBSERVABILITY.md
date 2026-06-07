# Observability

Flagstone emits **traces** and **metrics** via the [OpenTelemetry](https://opentelemetry.io/) protocol (OTLP/HTTP). All configuration is through standard `OTEL_*` environment variables — no custom config surface.

This is zero-overhead by default: none of the `OTEL_*` vars are set, so no instrumentation runs.

## Quick start

```bash
# Point at any OTLP-capable backend (Tempo, Jaeger, OTel Collector, etc.)
export OTEL_TRACES_EXPORTER=otlp
export OTEL_METRICS_EXPORTER=otlp
export OTEL_EXPORTER_OTLP_ENDPOINT=http://localhost:4318

# 10% trace sampling (default is 100% when exporter is on)
export OTEL_TRACES_SAMPLER=parentbased_traceidratio
export OTEL_TRACES_SAMPLER_ARG=0.1

# Identity
export OTEL_SERVICE_NAME=flagstone
export OTEL_RESOURCE_ATTRIBUTES=deployment.environment=production,service.version=v1.0.0

flagstone
```

## Configuration reference

| Env var | Default | Description |
|---|---|---|
| `OTEL_TRACES_EXPORTER` | _(unset → no-op)_ | `otlp` or `none` |
| `OTEL_METRICS_EXPORTER` | _(unset → no-op)_ | `otlp`, `prometheus`, `otlp,prometheus`, or `none` |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | `http://localhost:4318` | Target backend or OTel Collector |
| `OTEL_EXPORTER_PROMETHEUS_HOST` | _(all interfaces)_ | Host for the Prometheus pull endpoint |
| `OTEL_EXPORTER_PROMETHEUS_PORT` | `9464` | Port for the Prometheus pull endpoint (`/metrics`) |
| `OTEL_TRACES_SAMPLER` | `parentbased_traceidratio` | Sampling strategy |
| `OTEL_TRACES_SAMPLER_ARG` | `0.1` (10%) | Sampling ratio (0.0–1.0) |
| `OTEL_SERVICE_NAME` | `flagstone` | Service identity |
| `OTEL_RESOURCE_ATTRIBUTES` | — | [Resource attributes](https://opentelemetry.io/docs/concepts/resources/) |

### Metrics: push (OTLP) vs pull (Prometheus)

`OTEL_METRICS_EXPORTER` accepts a comma-separated list, so the two modes can coexist:

- `otlp` — push metrics to `OTEL_EXPORTER_OTLP_ENDPOINT` on a 30s interval.
- `prometheus` — expose a pull endpoint at `:9464/metrics` for teams that already scrape with Prometheus. No push pipeline needed.
- `otlp,prometheus` — run both.

```bash
# Prometheus pull only — scrape http://<host>:9464/metrics
export OTEL_METRICS_EXPORTER=prometheus
flagstone
```

## Traces

Flagstone creates spans for:

**HTTP requests** — via `otelhttp` automatic instrumentation wrapping the router. Skips `/healthz` and `/readyz`. Includes `http.method`, `http.route`, and `http.response.status_code`.

**Flag evaluations** — single-flag (`POST /api/v1/evaluate/flags/{key}`) and bulk (`POST /api/v1/evaluate/flags`). Uses the [OTel feature-flag semantic conventions](https://opentelemetry.io/docs/specs/semconv/attributes-registry/feature-flag/):

| Attribute | Example | Source |
|---|---|---|
| `feature_flag.key` | `new-checkout` | The evaluated flag key |
| `feature_flag.provider_name` | `flagstone` | Always "flagstone" |
| `feature_flag.result.reason` | `rule_match` | Why the value was chosen |
| `feature_flag.result.variant` | `true` | The evaluated value |
| `flagstone.environment` | `production` | The environment slug |

**Snapshot fetch** — (`GET /api/v1/sdk/snapshot`) includes `flagstone.environment`.

**Database queries** — automatically via `otelpgx` tracer on the pgx pool.

**SSE publish** — each event broadcast includes `flagstone.event.type` and `flagstone.environment`.

## Metrics

| Metric | Type | Labels | Description |
|---|---|---|---|
| `flagstone.evaluations.total` | Counter | `feature_flag.key`, `feature_flag.result.reason`, `flagstone.environment` | Total flag evaluations |
| `flagstone.snapshot.fetch.duration` | Histogram | `flagstone.environment`, `flagstone.status` | Snapshot fetch latency (seconds) |
| `flagstone.snapshot.fetch.total` | Counter | `flagstone.environment`, `flagstone.status` | Snapshot fetch count |
| `flagstone.sse.connections.active` | Gauge | — | Concurrent SSE connections |
| `flagstone.sse.events.published.total` | Counter | `flagstone.event.type` | Published SSE events |
| `flagstone.db.pool.connections.idle` | Gauge | — | Idle DB connections (from `pgxpool.Stat()`) |
| `flagstone.db.pool.connections.acquired.total` | Counter | — | Cumulative connections acquired from pool |
| `flagstone.db.pool.acquire.wait.duration` | Counter | — | Cumulative pool acquire wait (seconds) |

Additional standard metrics from `otelhttp` and `otelpgx`.

## Log correlation

Structured logs (zap) include `trace_id` and `span_id` when a trace is active, so you can pivot from a log line to the corresponding trace in your backend.

```json
{"level":"info","ts":"...","msg":"request completed",
 "trace_id":"a1b2c3...","span_id":"d4e5f6...",
 "method":"POST","path":"/api/v1/evaluate/flags","status":200}
```

## Backend examples

### Grafana Tempo + Loki

```bash
export OTEL_TRACES_EXPORTER=otlp
export OTEL_METRICS_EXPORTER=otlp
export OTEL_EXPORTER_OTLP_ENDPOINT=http://tempo:4318
```

The trace_id in logs correlates with Tempo traces. Otelhttp and otelpgx metrics appear in Grafana's metric browser.

### OTel Collector → Datadog

```yaml
# collector.yaml
receivers:
  otlp:
    protocols:
      http:
exporters:
  datadog:
    api:
      key: "${DD_API_KEY}"
service:
  pipelines:
    traces:
      receivers: [otlp]
      exporters: [datadog]
    metrics:
      receivers: [otlp]
      exporters: [datadog]
```

```bash
export OTEL_EXPORTER_OTLP_ENDPOINT=http://collector:4318
```

### Jaeger

```bash
export OTEL_TRACES_EXPORTER=otlp
export OTEL_EXPORTER_OTLP_ENDPOINT=http://jaeger:4318
```

Jaeger ≥ 1.35 speaks OTLP natively.

## Important notes

- **Cardinality discipline**: never use `user_id`, `targeting_key`, or `request_id` as metric labels. These appear in traces but never in metrics.
- **The OTel Collector** is recommended for production deployments (buffering, retries, tail sampling, multi-destination fan-out) but not required.
