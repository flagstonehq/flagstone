package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/thomas-vilte/flagstone/internal/auth"
	"github.com/thomas-vilte/flagstone/internal/engine"
	"github.com/thomas-vilte/flagstone/internal/storage"
)

func seedEvaluateData(t *testing.T) (tenantID, projectID uuid.UUID, envSlug, apiKey string) {
	t.Helper()

	password := "securepass123"
	hash, err := auth.HashPassword(password, testServer.cfg.BcryptCost)
	require.NoError(t, err)

	tenant := &storage.Tenant{Slug: "eval-tenant-" + uuid.New().String()[:8], Name: "Eval Tenant", Plan: "free"}
	require.NoError(t, testServer.stores.Tenants.Create(context.Background(), tenant))

	user := &storage.User{Email: "eval@example.com", PasswordHash: &hash}
	require.NoError(t, testServer.stores.Users.Create(context.Background(), user))

	member := &storage.TenantMember{TenantID: tenant.ID, UserID: user.ID, Role: "admin"}
	require.NoError(t, testServer.stores.Members.Add(context.Background(), member))

	project := &storage.Project{TenantID: tenant.ID, Slug: "eval-proj", Name: "Eval Project"}
	require.NoError(t, testServer.stores.Projects.Create(context.Background(), project))

	env := &storage.Environment{ProjectID: project.ID, Slug: "prod", Name: "Production"}
	require.NoError(t, testServer.stores.Environments.Create(context.Background(), env))

	rawKey, keyHash, prefix, err := auth.GenerateAPIKey("test", 32)
	require.NoError(t, err)

	apikey := &storage.APIKey{
		EnvironmentID: env.ID,
		Name:          "eval-key",
		KeyHash:       keyHash,
		KeyPrefix:     prefix,
		CreatedBy:     &user.ID,
	}
	require.NoError(t, testServer.stores.APIKeys.Create(context.Background(), apikey))

	flag := &storage.Flag{
		ProjectID:    project.ID,
		Key:          "test-flag",
		Name:         "Test Flag",
		Type:         "boolean",
		DefaultValue: json.RawMessage("false"),
	}
	require.NoError(t, testServer.stores.Flags.Create(context.Background(), flag))

	rules := []engine.Rule{
		{
			Conditions: engine.ConditionNode{
				Attribute: strPtr("plan"),
				Op:        strPtr("eq"),
				Value:     "premium",
			},
			Value: true,
		},
	}
	rulesJSON, err := json.Marshal(rules)
	require.NoError(t, err)

	flagEnv := &storage.FlagEnvironment{
		FlagID:        flag.ID,
		EnvironmentID: env.ID,
		Enabled:       true,
		Rules:         rulesJSON,
	}
	require.NoError(t, testServer.stores.FlagEnvironments.Upsert(context.Background(), flagEnv))

	return tenant.ID, project.ID, env.Slug, rawKey
}

func strPtr(s string) *string { return &s }

func TestEvaluateFlag_NotFound(t *testing.T) {
	skipIfNoDB(t)
	truncateTables(t, "audit_log", "sessions", "flag_environments", "environments", "flags", "segments", "projects", "apikeys", "tenant_members", "users", "tenants")

	_, _, _, apiKey := seedEvaluateData(t)

	body := `{"context":{"user_id":"u1","plan":"free"}}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/evaluate/flags/nonexistent", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)
	rec := httptest.NewRecorder()
	testServer.Routes().ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)

	var resp evaluateFlagResponse
	err := json.NewDecoder(rec.Body).Decode(&resp)
	require.NoError(t, err)
	assert.Equal(t, "nonexistent", resp.Key)
	assert.Equal(t, false, resp.Value)
	assert.Equal(t, engine.ReasonFlagNotFound, resp.Reason)
	assert.Equal(t, -1, resp.RuleIndex)
}

func TestEvaluateFlag_Disabled(t *testing.T) {
	skipIfNoDB(t)
	truncateTables(t, "audit_log", "sessions", "flag_environments", "environments", "flags", "segments", "projects", "apikeys", "tenant_members", "users", "tenants")

	tenantID, projectID, _, apiKey := seedEvaluateData(t)

	flag := &storage.Flag{
		ProjectID:    projectID,
		Key:          "disabled-flag",
		Name:         "Disabled Flag",
		Type:         "boolean",
		DefaultValue: json.RawMessage("true"),
	}
	require.NoError(t, testServer.stores.Flags.Create(context.Background(), flag))

	envs, err := testServer.stores.Environments.ListByProject(context.Background(), projectID)
	require.NoError(t, err)
	require.Len(t, envs, 1)

	flagEnv := &storage.FlagEnvironment{
		FlagID:        flag.ID,
		EnvironmentID: envs[0].ID,
		Enabled:       false,
		Rules:         json.RawMessage("[]"),
	}
	require.NoError(t, testServer.stores.FlagEnvironments.Upsert(context.Background(), flagEnv))

	body := `{"context":{"user_id":"u1"}}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/evaluate/flags/disabled-flag", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)
	_ = tenantID
	rec := httptest.NewRecorder()
	testServer.Routes().ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)

	var resp evaluateFlagResponse
	err = json.NewDecoder(rec.Body).Decode(&resp)
	require.NoError(t, err)
	assert.Equal(t, false, resp.Value)
	assert.Equal(t, engine.ReasonDisabled, resp.Reason)
}

func TestEvaluateFlag_RuleMatch(t *testing.T) {
	skipIfNoDB(t)
	truncateTables(t, "audit_log", "sessions", "flag_environments", "environments", "flags", "segments", "projects", "apikeys", "tenant_members", "users", "tenants")

	_, _, _, apiKey := seedEvaluateData(t)

	body := `{"context":{"user_id":"u1","plan":"premium"}}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/evaluate/flags/test-flag", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)
	rec := httptest.NewRecorder()
	testServer.Routes().ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)

	var resp evaluateFlagResponse
	err := json.NewDecoder(rec.Body).Decode(&resp)
	require.NoError(t, err)
	assert.Equal(t, "test-flag", resp.Key)
	assert.Equal(t, true, resp.Value)
	assert.Equal(t, engine.ReasonRuleMatch, resp.Reason)
	assert.Equal(t, 0, resp.RuleIndex)
}

func TestEvaluateFlag_DefaultValue(t *testing.T) {
	skipIfNoDB(t)
	truncateTables(t, "audit_log", "sessions", "flag_environments", "environments", "flags", "segments", "projects", "apikeys", "tenant_members", "users", "tenants")

	_, _, _, apiKey := seedEvaluateData(t)

	body := `{"context":{"user_id":"u1","plan":"free"}}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/evaluate/flags/test-flag", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)
	rec := httptest.NewRecorder()
	testServer.Routes().ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)

	var resp evaluateFlagResponse
	err := json.NewDecoder(rec.Body).Decode(&resp)
	require.NoError(t, err)
	assert.Equal(t, false, resp.Value)
	assert.Equal(t, engine.ReasonDefault, resp.Reason)
}

func TestEvaluateBulk_Success(t *testing.T) {
	skipIfNoDB(t)
	truncateTables(t, "audit_log", "sessions", "flag_environments", "environments", "flags", "segments", "projects", "apikeys", "tenant_members", "users", "tenants")

	_, _, _, apiKey := seedEvaluateData(t)

	body := `{"context":{"user_id":"u1","plan":"premium"}}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/evaluate/flags", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)
	rec := httptest.NewRecorder()
	testServer.Routes().ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)

	var resp evaluateFlagsResponse
	err := json.NewDecoder(rec.Body).Decode(&resp)
	require.NoError(t, err)
	assert.NotEmpty(t, resp.Environment)
	assert.NotEmpty(t, resp.EvaluatedAt)
	assert.Contains(t, resp.Flags, "test-flag")
	assert.Equal(t, true, resp.Flags["test-flag"].Value)
}

func TestEvaluateFlag_NoAuthHeader(t *testing.T) {
	skipIfNoDB(t)
	body := `{"context":{"user_id":"u1"}}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/evaluate/flags/test-flag", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	testServer.Routes().ServeHTTP(rec, req)

	require.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestEvaluateFlag_InvalidAPIKey(t *testing.T) {
	skipIfNoDB(t)
	body := `{"context":{"user_id":"u1","plan":"free"}}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/evaluate/flags/test-flag", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer invalid-key-here")
	rec := httptest.NewRecorder()
	testServer.Routes().ServeHTTP(rec, req)

	require.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestEvaluateFlag_RevokedAPIKey(t *testing.T) {
	skipIfNoDB(t)
	truncateTables(t, "audit_log", "sessions", "flag_environments", "environments", "flags", "segments", "projects", "api_keys", "tenant_members", "users", "tenants")

	_, _, _, apiKey := seedEvaluateData(t)

	// Revoke the key by hash lookup then soft-delete.
	keyHash := auth.HashAPIKey(apiKey)
	key, err := testServer.stores.APIKeys.GetByHash(context.Background(), keyHash)
	require.NoError(t, err)
	require.NoError(t, testServer.stores.APIKeys.Revoke(context.Background(), key.ID, time.Now().UTC()))

	body := `{"context":{"user_id":"u1","plan":"premium"}}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/evaluate/flags/test-flag", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)
	rec := httptest.NewRecorder()
	testServer.Routes().ServeHTTP(rec, req)

	// GetByHash filters revoked_at IS NULL, so the key is not found → 401.
	require.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestEvaluateFlag_RolloutDeterministic(t *testing.T) {
	skipIfNoDB(t)
	truncateTables(t, "audit_log", "sessions", "flag_environments", "environments", "flags", "segments", "projects", "api_keys", "tenant_members", "users", "tenants")

	_, projectID, _, apiKey := seedEvaluateData(t)

	// Create a flag with a 50% rollout rule.
	flag := &storage.Flag{
		ProjectID:    projectID,
		Key:          "rollout-flag",
		Name:         "Rollout Flag",
		Type:         "boolean",
		DefaultValue: json.RawMessage("false"),
	}
	require.NoError(t, testServer.stores.Flags.Create(context.Background(), flag))

	envs, err := testServer.stores.Environments.ListByProject(context.Background(), projectID)
	require.NoError(t, err)
	require.Len(t, envs, 1)

	rules := []engine.Rule{
		{
			Conditions: engine.ConditionNode{
				Attribute: strPtr("country"),
				Op:        strPtr("eq"),
				Value:     "AR",
			},
			Rollout: &engine.RolloutConfig{Percentage: 50, Seed: "rollout-flag"},
			Value:   true,
		},
	}
	rulesJSON, err := json.Marshal(rules)
	require.NoError(t, err)

	flagEnv := &storage.FlagEnvironment{
		FlagID:        flag.ID,
		EnvironmentID: envs[0].ID,
		Enabled:       true,
		Rules:         rulesJSON,
	}
	require.NoError(t, testServer.stores.FlagEnvironments.Upsert(context.Background(), flagEnv))

	// Call the endpoint 10 times for the same user — result must be identical every time.
	body := `{"context":{"user_id":"stable-user-123","country":"AR"}}`
	var firstValue any
	for range 10 {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/evaluate/flags/rollout-flag", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+apiKey)
		rec := httptest.NewRecorder()
		testServer.Routes().ServeHTTP(rec, req)
		require.Equal(t, http.StatusOK, rec.Code)

		var resp evaluateFlagResponse
		require.NoError(t, json.NewDecoder(rec.Body).Decode(&resp))

		if firstValue == nil {
			firstValue = resp.Value
		} else {
			assert.Equal(t, firstValue, resp.Value, "rollout result must be deterministic for the same user_id")
		}
	}
}

func TestEvaluateBulk_ContextTooManyKeys(t *testing.T) {
	skipIfNoDB(t)
	truncateTables(t, "audit_log", "sessions", "flag_environments", "environments", "flags", "segments", "projects", "apikeys", "tenant_members", "users", "tenants")

	_, _, _, apiKey := seedEvaluateData(t)

	ctxMap := make(map[string]any)
	for i := 0; i < 101; i++ {
		ctxMap[fmt.Sprintf("key_%d", i)] = "val"
	}
	ctxJSON, _ := json.Marshal(map[string]any{"context": ctxMap})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/evaluate/flags", strings.NewReader(string(ctxJSON)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)
	rec := httptest.NewRecorder()
	testServer.Routes().ServeHTTP(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestEvaluateBulk_ContextValueTooLong(t *testing.T) {
	skipIfNoDB(t)
	truncateTables(t, "audit_log", "sessions", "flag_environments", "environments", "flags", "segments", "projects", "apikeys", "tenant_members", "users", "tenants")

	_, _, _, apiKey := seedEvaluateData(t)

	longVal := strings.Repeat("a", 1025)
	body := fmt.Sprintf(`{"context":{"user_id":%q}}`, longVal)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/evaluate/flags", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)
	rec := httptest.NewRecorder()
	testServer.Routes().ServeHTTP(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestEvaluateBulk_ContextInvalidType(t *testing.T) {
	skipIfNoDB(t)
	truncateTables(t, "audit_log", "sessions", "flag_environments", "environments", "flags", "segments", "projects", "apikeys", "tenant_members", "users", "tenants")

	_, _, _, apiKey := seedEvaluateData(t)

	body := `{"context":{"user_id":{"nested":"object"}}}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/evaluate/flags", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)
	rec := httptest.NewRecorder()
	testServer.Routes().ServeHTTP(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
}
