package telemetry

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
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

	tracesOn := os.Getenv("OTEL_TRACES_EXPORTER") != "" &&
		os.Getenv("OTEL_TRACES_EXPORTER") != "none"
	metricsOn := os.Getenv("OTEL_METRICS_EXPORTER") != "" &&
		os.Getenv("OTEL_METRICS_EXPORTER") != "none"

	if !tracesOn && !metricsOn {
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

	if metricsOn {
		metricExp, err := otlpmetrichttp.New(ctx)
		if err != nil {
			return nil, fmt.Errorf("telemetry: create metric exporter: %w", err)
		}

		mp := metric.NewMeterProvider(
			metric.WithReader(
				metric.NewPeriodicReader(metricExp,
					metric.WithInterval(30*time.Second),
				),
			),
			metric.WithResource(res),
		)
		shutdownFuncs = append(shutdownFuncs, mp.Shutdown)

		otel.SetMeterProvider(mp)
	}

	return shutdown, nil
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
