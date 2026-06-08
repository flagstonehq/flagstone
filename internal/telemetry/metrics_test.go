package telemetry_test

import (
	"context"
	"testing"

	"github.com/flagstonehq/flagstone/internal/telemetry"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
)

func TestMetricsNilSafe(t *testing.T) {
	var m *telemetry.Metrics

	ctx := context.Background()

	m.RecordAuthLogin(ctx, "success", "")
	m.RecordAuthLogin(ctx, "failure", "INVALID_CREDENTIALS")
	m.RecordAuthRefresh(ctx, "success")
	m.RecordAuthLogout(ctx)
	m.RecordAuthAPIKeyValidation(ctx, "expired")
	m.RecordAuthRateLimitHit(ctx, "login")

	m.RecordEvaluation(ctx)
	m.RecordSSEEvent(ctx, "test")
	_ = m.RegisterDBPoolStats(func() telemetry.DBPoolStats {
		return telemetry.DBPoolStats{}
	})
}

func TestNewMetricsAndRecord(t *testing.T) {
	mp := metric.NewMeterProvider()
	defer func() {
		_ = mp.Shutdown(context.Background())
	}()

	m, err := telemetry.NewMetrics(mp)
	if err != nil {
		t.Fatalf("failed to create metrics: %v", err)
	}

	ctx := context.Background()

	m.RecordAuthLogin(ctx, "success", "")
	m.RecordAuthLogin(ctx, "failure", "INVALID_CREDENTIALS")
	m.RecordAuthRefresh(ctx, "success")
	m.RecordAuthLogout(ctx)
	m.RecordAuthAPIKeyValidation(ctx, "expired")
	m.RecordAuthRateLimitHit(ctx, "login")
}

func TestAuthMetricsAreEmitted(t *testing.T) {
	reader := metric.NewManualReader()
	mp := metric.NewMeterProvider(metric.WithReader(reader))
	defer func() { _ = mp.Shutdown(context.Background()) }()

	m, err := telemetry.NewMetrics(mp)
	if err != nil {
		t.Fatalf("failed to create metrics: %v", err)
	}

	ctx := context.Background()
	m.RecordAuthLogin(ctx, "failure", "INVALID_CREDENTIALS")
	m.RecordAuthLogin(ctx, "failure", "INVALID_CREDENTIALS")
	m.RecordAuthLogin(ctx, "success", "")
	m.RecordAuthRateLimitHit(ctx, "login")

	var rm metricdata.ResourceMetrics
	if err := reader.Collect(ctx, &rm); err != nil {
		t.Fatalf("collect: %v", err)
	}

	if got := sumForAttrs(t, rm, "flagstone.auth.login.total", map[string]string{
		"flagstone.status":          "failure",
		"flagstone.auth.error_code": "INVALID_CREDENTIALS",
	}); got != 2 {
		t.Fatalf("auth.login.total{failure,INVALID_CREDENTIALS} = %d, want 2", got)
	}

	if got := sumForAttrs(t, rm, "flagstone.auth.login.total", map[string]string{
		"flagstone.status": "success",
	}); got != 1 {
		t.Fatalf("auth.login.total{success} = %d, want 1", got)
	}

	if got := sumForAttrs(t, rm, "flagstone.auth.ratelimit.hits.total", map[string]string{
		"flagstone.auth.endpoint": "login",
	}); got != 1 {
		t.Fatalf("auth.ratelimit.hits.total{login} = %d, want 1", got)
	}
}

func sumForAttrs(t *testing.T, rm metricdata.ResourceMetrics, name string, want map[string]string) int64 {
	t.Helper()
	for _, sm := range rm.ScopeMetrics {
		for _, md := range sm.Metrics {
			if md.Name != name {
				continue
			}
			sum, ok := md.Data.(metricdata.Sum[int64])
			if !ok {
				t.Fatalf("metric %s is not Sum[int64]", name)
			}
			for _, dp := range sum.DataPoints {
				if attrsMatch(dp.Attributes, want) {
					return dp.Value
				}
			}
		}
	}
	t.Fatalf("metric %s with attributes %v not found", name, want)
	return 0
}

func attrsMatch(set attribute.Set, want map[string]string) bool {
	for k, v := range want {
		val, ok := set.Value(attribute.Key(k))
		if !ok || val.AsString() != v {
			return false
		}
	}
	return true
}
