package telemetry

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	promexporter "go.opentelemetry.io/otel/exporters/prometheus"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
)

// SetupOTel initializes the OTel tracer and meter providers. It reads
// standard OTEL_* environment variables and returns a shutdown function
// that flushes and tears down the providers.
//
// If OTEL_TRACES_EXPORTER and OTEL_METRICS_EXPORTER are both unset
// (or "none"), the providers are no-op and shutdown is a no-op.
// This gives zero-overhead when observability is not configured.
//
// OTEL_METRICS_EXPORTER accepts a comma-separated list: "otlp", "prometheus",
// or "otlp,prometheus". The prometheus reader exposes a pull endpoint at
// :9464/metrics (override host/port via OTEL_EXPORTER_PROMETHEUS_HOST/PORT).
func SetupOTel(ctx context.Context, serviceName string) (shutdown func(context.Context) error, err error) {
	var shutdownFuncs []func(context.Context) error

	shutdown = func(ctx context.Context) error {
		var errs []error
		for _, fn := range shutdownFuncs {
			if err := fn(ctx); err != nil {
				errs = append(errs, err)
			}
		}
		return errors.Join(errs...)
	}

	tracesOn := exporterEnabled("OTEL_TRACES_EXPORTER")
	metricExporters := metricExporterSet()

	if !tracesOn && len(metricExporters) == 0 {
		// OTel defaults to noop providers already. Nothing to set up.
		return shutdown, nil
	}

	res, err := resource.New(ctx,
		resource.WithFromEnv(),
		resource.WithAttributes(
			semconv.ServiceName(serviceName),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("telemetry: build resource: %w", err)
	}

	if tracesOn {
		traceExp, err := otlptracehttp.New(ctx)
		if err != nil {
			return nil, fmt.Errorf("telemetry: create trace exporter: %w", err)
		}

		sampler := sdktrace.ParentBased(
			sdktrace.TraceIDRatioBased(sampleRatio()),
		)

		tp := sdktrace.NewTracerProvider(
			sdktrace.WithBatcher(traceExp,
				sdktrace.WithBatchTimeout(5*time.Second),
			),
			sdktrace.WithResource(res),
			sdktrace.WithSampler(sampler),
		)
		shutdownFuncs = append(shutdownFuncs, tp.Shutdown)

		otel.SetTracerProvider(tp)
	}

	if len(metricExporters) > 0 {
		opts := []metric.Option{metric.WithResource(res)}

		if metricExporters["otlp"] {
			metricExp, err := otlpmetrichttp.New(ctx)
			if err != nil {
				return nil, fmt.Errorf("telemetry: create otlp metric exporter: %w", err)
			}
			opts = append(opts, metric.WithReader(
				metric.NewPeriodicReader(metricExp,
					metric.WithInterval(30*time.Second),
				),
			))
		}

		if metricExporters["prometheus"] {
			registry := prometheus.NewRegistry()
			promReader, err := promexporter.New(promexporter.WithRegisterer(registry))
			if err != nil {
				return nil, fmt.Errorf("telemetry: create prometheus exporter: %w", err)
			}
			opts = append(opts, metric.WithReader(promReader))

			promSrv := startPrometheusServer(registry)
			shutdownFuncs = append(shutdownFuncs, promSrv.Shutdown)
		}

		mp := metric.NewMeterProvider(opts...)
		shutdownFuncs = append(shutdownFuncs, mp.Shutdown)

		otel.SetMeterProvider(mp)
	}

	return shutdown, nil
}

// startPrometheusServer launches an HTTP server exposing /metrics for the
// given registry. The address defaults to :9464 and honors the standard
// OTEL_EXPORTER_PROMETHEUS_HOST / OTEL_EXPORTER_PROMETHEUS_PORT env vars.
func startPrometheusServer(registry *prometheus.Registry) *http.Server {
	host := os.Getenv("OTEL_EXPORTER_PROMETHEUS_HOST")
	port := os.Getenv("OTEL_EXPORTER_PROMETHEUS_PORT")
	if port == "" {
		port = "9464"
	}

	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.HandlerFor(registry, promhttp.HandlerOpts{}))

	srv := &http.Server{
		Addr:              host + ":" + port,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}
	go func() {
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			otel.Handle(err)
		}
	}()
	return srv
}

// exporterEnabled reports whether the given OTEL_*_EXPORTER env var is set
// to a non-empty value other than "none".
func exporterEnabled(envVar string) bool {
	v := strings.TrimSpace(os.Getenv(envVar))
	return v != "" && v != "none"
}

// metricExporterSet parses OTEL_METRICS_EXPORTER (comma-separated) into a set
// of recognized exporters. "none" or empty yields an empty set.
func metricExporterSet() map[string]bool {
	raw := strings.TrimSpace(os.Getenv("OTEL_METRICS_EXPORTER"))
	if raw == "" || raw == "none" {
		return nil
	}
	set := make(map[string]bool)
	for part := range strings.SplitSeq(raw, ",") {
		switch p := strings.TrimSpace(part); p {
		case "otlp", "prometheus":
			set[p] = true
		}
	}
	return set
}

// sampleRatio returns the sampling ratio from OTEL_TRACES_SAMPLER_ARG,
// defaulting to 0.1 (10%).
func sampleRatio() float64 {
	const defaultRatio = 0.1
	raw := os.Getenv("OTEL_TRACES_SAMPLER_ARG")
	if raw == "" {
		return defaultRatio
	}
	var ratio float64
	if _, err := fmt.Sscanf(raw, "%f", &ratio); err != nil || ratio <= 0 {
		return defaultRatio
	}
	if ratio > 1.0 {
		return 1.0
	}
	return ratio
}
