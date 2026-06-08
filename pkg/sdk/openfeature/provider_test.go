package openfeature_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/flagstonehq/flagstone/pkg/sdk"
	fsopenfeature "github.com/flagstonehq/flagstone/pkg/sdk/openfeature"
	of "github.com/open-feature/go-sdk/openfeature"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testSnapshot = `{
  "environment": "test",
  "flags": {
    "bool-flag": {
      "key": "bool-flag",
      "enabled": true,
      "flag_type": "boolean",
      "default_value": false,
      "version": 1,
      "rules": [
        {
          "conditions": {"attribute": "plan", "op": "eq", "value": "premium"},
          "value": true
        }
      ]
    },
    "str-flag": {
      "key": "str-flag",
      "enabled": true,
      "flag_type": "string",
      "default_value": "hello",
      "version": 1,
      "rules": []
    },
    "num-flag": {
      "key": "num-flag",
      "enabled": true,
      "flag_type": "number",
      "default_value": 3.14,
      "version": 1,
      "rules": []
    }
  },
  "segments": {},
  "fetched_at": "2026-06-08T00:00:00Z"
}`

func newProvider(t *testing.T) *fsopenfeature.Provider {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/sdk/snapshot", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(testSnapshot))
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	c, err := sdk.New(sdk.WithEndpoint(srv.URL), sdk.WithAPIKey("k"))
	require.NoError(t, err)
	require.NoError(t, c.Start(context.Background()))
	return fsopenfeature.NewProvider(c)
}

func TestProviderMetadata(t *testing.T) {
	p := newProvider(t)
	assert.Equal(t, "flagstone", p.Metadata().Name)
}

func TestBooleanEvaluation_Default(t *testing.T) {
	p := newProvider(t)
	d := p.BooleanEvaluation(context.Background(), "bool-flag", true, nil)
	assert.Equal(t, false, d.Value)
	assert.Equal(t, of.DefaultReason, d.Reason)
}

func TestBooleanEvaluation_RuleMatch(t *testing.T) {
	p := newProvider(t)
	d := p.BooleanEvaluation(context.Background(), "bool-flag", false, of.FlattenedContext{"plan": "premium"})
	assert.Equal(t, true, d.Value)
	assert.Equal(t, of.TargetingMatchReason, d.Reason)
}

func TestBooleanEvaluation_FlagNotFound(t *testing.T) {
	p := newProvider(t)
	d := p.BooleanEvaluation(context.Background(), "ghost", true, nil)
	assert.Equal(t, true, d.Value, "default returned on not-found")
	assert.Equal(t, of.ErrorReason, d.Reason)
	assert.NotEmpty(t, d.ResolutionError.Error())
}

func TestStringEvaluation_Default(t *testing.T) {
	p := newProvider(t)
	d := p.StringEvaluation(context.Background(), "str-flag", "fallback", nil)
	assert.Equal(t, "hello", d.Value)
	assert.Equal(t, of.DefaultReason, d.Reason)
}

func TestFloatEvaluation_Default(t *testing.T) {
	p := newProvider(t)
	d := p.FloatEvaluation(context.Background(), "num-flag", 0.0, nil)
	assert.InDelta(t, 3.14, d.Value, 0.001)
	assert.Equal(t, of.DefaultReason, d.Reason)
}

func TestIntEvaluation_Default(t *testing.T) {
	p := newProvider(t)
	d := p.IntEvaluation(context.Background(), "num-flag", 0, nil)
	assert.Equal(t, int64(3), d.Value)
	assert.Equal(t, of.DefaultReason, d.Reason)
}

func TestObjectEvaluation_FlagNotFound(t *testing.T) {
	p := newProvider(t)
	d := p.ObjectEvaluation(context.Background(), "ghost", map[string]any{"x": 1}, nil)
	assert.Equal(t, map[string]any{"x": 1}, d.Value)
	assert.Equal(t, of.ErrorReason, d.Reason)
}

func TestBooleanEvaluation_TypeMismatch(t *testing.T) {
	p := newProvider(t)
	// str-flag is a string flag; asking for bool should return default + error
	d := p.BooleanEvaluation(context.Background(), "str-flag", true, nil)
	assert.Equal(t, true, d.Value, "default returned on type mismatch")
	assert.Equal(t, of.ErrorReason, d.Reason)
	assert.NotEmpty(t, d.ResolutionError.Error())
}
