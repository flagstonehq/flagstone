package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/flagstonehq/flagstone/internal/auth"
	"github.com/flagstonehq/flagstone/internal/storage"
	"github.com/flagstonehq/flagstone/internal/telemetry"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
)

func seedUsageEnv(t *testing.T) (apiKey string) {
	t.Helper()
	tenant := &storage.Tenant{
		Slug: "usage-tenant-" + uuid.New().String()[:8],
		Name: "Usage Tenant",
		Plan: "free",
	}
	require.NoError(t, testServer.stores.Tenants.Create(context.Background(), tenant))

	project := &storage.Project{TenantID: tenant.ID, Slug: "usage-proj", Name: "Usage Project"}
	require.NoError(t, testServer.stores.Projects.Create(context.Background(), project))

	env := &storage.Environment{ProjectID: project.ID, Slug: "prod", Name: "Production"}
	require.NoError(t, testServer.stores.Environments.Create(context.Background(), env))

	rawKey, keyHash, prefix, err := auth.GenerateAPIKey("test", 32)
	require.NoError(t, err)
	require.NoError(t, testServer.stores.APIKeys.Create(context.Background(), &storage.APIKey{
		EnvironmentID: env.ID,
		Name:          "usage-key",
		KeyHash:       keyHash,
		KeyPrefix:     prefix,
	}))
	return rawKey
}

func TestHandleSDKUsage_NoAuth(t *testing.T) {
	skipIfNoDB(t)

	body := `{"entries":[{"flag_key":"f","count":1,"last_reason":"DEFAULT","last_evaluated_at":"2026-06-08T00:00:00Z"}]}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/sdk/usage", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	testServer.Routes().ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestHandleSDKUsage_InvalidJSON(t *testing.T) {
	skipIfNoDB(t)

	apiKey := seedUsageEnv(t)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/sdk/usage", strings.NewReader("not-json"))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)
	rec := httptest.NewRecorder()
	testServer.Routes().ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestHandleSDKUsage_EmptyEntries(t *testing.T) {
	skipIfNoDB(t)

	apiKey := seedUsageEnv(t)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/sdk/usage", strings.NewReader(`{"entries":[]}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)
	rec := httptest.NewRecorder()
	testServer.Routes().ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNoContent, rec.Code)
}

func TestHandleSDKUsage_HappyPath(t *testing.T) {
	skipIfNoDB(t)

	apiKey := seedUsageEnv(t)
	body := `{"entries":[
		{"flag_key":"checkout","count":5,"last_reason":"RULE_MATCH","last_evaluated_at":"2026-06-08T00:00:00Z"},
		{"flag_key":"dark-mode","count":3,"last_reason":"DEFAULT","last_evaluated_at":"2026-06-08T00:00:00Z"}
	]}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/sdk/usage", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)
	rec := httptest.NewRecorder()
	testServer.Routes().ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNoContent, rec.Code)
}

func TestHandleSDKUsage_CountCapIsEnforced(t *testing.T) {
	skipIfNoDB(t)

	// Create a server with real metrics so we can assert the recorded count.
	reader := metric.NewManualReader()
	mp := metric.NewMeterProvider(metric.WithReader(reader))
	defer func() { _ = mp.Shutdown(context.Background()) }()

	m, err := telemetry.NewMetrics(mp)
	require.NoError(t, err)

	srv := NewServer(testServer.stores, testPool, testServer.cfg, testServer.logger, nil, m)
	srv.loginLimiter = testServer.loginLimiter
	srv.refreshLimiter = testServer.refreshLimiter

	apiKey := seedUsageEnv(t)

	// Send count = 200_001, which is above the 100_000 cap.
	body := `{"entries":[{"flag_key":"f","count":200001,"last_reason":"DEFAULT","last_evaluated_at":"2026-06-08T00:00:00Z"}]}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/sdk/usage", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)
	rec := httptest.NewRecorder()
	srv.Routes().ServeHTTP(rec, req)

	require.Equal(t, http.StatusNoContent, rec.Code)

	var rm metricdata.ResourceMetrics
	require.NoError(t, reader.Collect(context.Background(), &rm))

	var total int64
	for _, sm := range rm.ScopeMetrics {
		for _, md := range sm.Metrics {
			if md.Name != "flagstone.evaluations.total" {
				continue
			}
			sum, ok := md.Data.(metricdata.Sum[int64])
			require.True(t, ok)
			for _, dp := range sum.DataPoints {
				total += dp.Value
			}
		}
	}
	assert.Equal(t, int64(100_000), total, "count should be capped at 100_000")
}
