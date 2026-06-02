package api

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func mustJSON(t *testing.T, v any) json.RawMessage {
	t.Helper()
	b, err := json.Marshal(v)
	require.NoError(t, err)
	return b
}

func TestValidateFlagRules_Empty(t *testing.T) {
	assert.NoError(t, ValidateFlagRules(nil))
	assert.NoError(t, ValidateFlagRules(json.RawMessage{}))
}

func TestValidateFlagRules_ValidLeaf(t *testing.T) {
	rules := mustJSON(t, []any{
		map[string]any{
			"conditions": map[string]any{"attribute": "plan", "op": "eq", "value": "premium"},
			"value":      true,
		},
	})
	assert.NoError(t, ValidateFlagRules(rules))
}

func TestValidateFlagRules_ValidComposite(t *testing.T) {
	rules := mustJSON(t, []any{
		map[string]any{
			"conditions": map[string]any{
				"all": []any{
					map[string]any{"attribute": "country", "op": "in", "value": []string{"AR", "BR"}},
					map[string]any{"attribute": "plan", "op": "neq", "value": "free"},
				},
			},
			"value": true,
		},
	})
	assert.NoError(t, ValidateFlagRules(rules))
}

func TestValidateFlagRules_UnknownOperator(t *testing.T) {
	rules := mustJSON(t, []any{
		map[string]any{
			"conditions": map[string]any{"attribute": "plan", "op": "like", "value": "%pro%"},
			"value":      true,
		},
	})
	err := ValidateFlagRules(rules)
	require.Error(t, err)
	assert.Contains(t, err.Error(), `operator "like" is not allowed`)
}

func TestValidateFlagRules_InvalidAttributeName(t *testing.T) {
	rules := mustJSON(t, []any{
		map[string]any{
			"conditions": map[string]any{"attribute": "a b c", "op": "eq", "value": "x"},
			"value":      true,
		},
	})
	err := ValidateFlagRules(rules)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "attribute")
}

func TestValidateFlagRules_ExceedsMaxRules(t *testing.T) {
	rules := make([]any, maxRulesPerFlag+1)
	for i := range rules {
		rules[i] = map[string]any{
			"conditions": map[string]any{"attribute": "plan", "op": "eq", "value": "x"},
			"value":      true,
		}
	}
	err := ValidateFlagRules(mustJSON(t, rules))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "maximum")
}

func TestValidateFlagRules_ExceedsMaxSize(t *testing.T) {
	big := strings.Repeat("x", maxRuleJSONBytes+1)
	err := ValidateFlagRules(json.RawMessage(`"` + big + `"`))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "size")
}

func TestValidateFlagRules_ExceedsDepth(t *testing.T) {
	// Build a deeply nested "all" tree beyond maxConditionDepth.
	var buildNested func(depth int) map[string]any
	buildNested = func(depth int) map[string]any {
		if depth == 0 {
			return map[string]any{"attribute": "x", "op": "exists"}
		}
		return map[string]any{"all": []any{buildNested(depth - 1)}}
	}

	rules := mustJSON(t, []any{
		map[string]any{
			"conditions": buildNested(maxConditionDepth + 2),
			"value":      true,
		},
	})
	err := ValidateFlagRules(rules)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "depth")
}

func TestValidateFlagRules_AllOperatorsAccepted(t *testing.T) {
	ops := []string{
		"eq", "neq", "gt", "gte", "lt", "lte",
		"contains", "starts_with", "ends_with",
		"in", "not_in", "matches",
		"exists", "not_exists", "segment",
	}
	for _, op := range ops {
		rules := mustJSON(t, []any{
			map[string]any{
				"conditions": map[string]any{"attribute": "attr", "op": op},
				"value":      true,
			},
		})
		assert.NoError(t, ValidateFlagRules(rules), "op %q should be valid", op)
	}
}

func TestValidateSegmentRules_Valid(t *testing.T) {
	raw := mustJSON(t, map[string]any{
		"all": []any{
			map[string]any{"attribute": "plan", "op": "eq", "value": "premium"},
			map[string]any{"attribute": "country", "op": "in", "value": []string{"AR"}},
		},
	})
	assert.NoError(t, ValidateSegmentRules(raw))
}

func TestValidateSegmentRules_UnknownOperator(t *testing.T) {
	raw := mustJSON(t, map[string]any{"attribute": "x", "op": "INVALID"})
	err := ValidateSegmentRules(raw)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not allowed")
}
