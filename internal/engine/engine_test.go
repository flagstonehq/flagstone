package engine

import (
	"testing"

	"go.uber.org/zap"
)

func TestEngine_Disabled(t *testing.T) {
	e := New(zap.NewNop())
	fc := FlagConfig{Key: "test", Enabled: false}
	req := EvaluateRequest{FlagConfig: fc, Context: map[string]any{}}
	res := e.Evaluate(req)
	if res.Reason != ReasonDisabled {
		t.Fatalf("want DISABLED, got %s", res.Reason)
	}
}

func TestEngine_NoRulesDefault(t *testing.T) {
	e := New(zap.NewNop())
	fc := FlagConfig{Key: "test", Enabled: true, DefaultValue: false}
	req := EvaluateRequest{FlagConfig: fc, Context: map[string]any{}}
	res := e.Evaluate(req)
	if res.Reason != ReasonDefault || res.Value != false {
		t.Fatalf("want DEFAULT/false, got %s/%v", res.Reason, res.Value)
	}
}

func TestEngine_EnvDefaultPreferred(t *testing.T) {
	e := New(zap.NewNop())
	fc := FlagConfig{
		Key: "test", Enabled: true,
		DefaultValue:            false,
		EnvironmentDefaultValue: true,
		HasEnvironmentDefault:   true,
	}
	req := EvaluateRequest{FlagConfig: fc, Context: map[string]any{}}
	res := e.Evaluate(req)
	if res.Value != true || res.Reason != ReasonDefault {
		t.Fatalf("want env default true, got %v/%s", res.Value, res.Reason)
	}
}

func TestEngine_FirstMatchWins(t *testing.T) {
	e := New(zap.NewNop())
	attr := "a"
	op := "eq"
	fc := FlagConfig{
		Key: "test", Enabled: true, DefaultValue: false,
		Rules: []Rule{
			{Conditions: ConditionNode{Attribute: &attr, Op: &op, Value: "x"}, Value: "rule0"},
			{Conditions: ConditionNode{Attribute: &attr, Op: &op, Value: "x"}, Value: "rule1"},
		},
	}
	req := EvaluateRequest{FlagConfig: fc, Context: map[string]any{"a": "x"}}
	res := e.Evaluate(req)
	if res.RuleIndex != 0 || res.Value != "rule0" {
		t.Fatalf("want rule0, got index %d value %v", res.RuleIndex, res.Value)
	}
}

func TestEngine_RolloutFallthrough(t *testing.T) {
	e := New(zap.NewNop())
	attr := "a"
	op := "eq"
	fc := FlagConfig{
		Key: "test", Enabled: true, DefaultValue: false,
		Rules: []Rule{
			{
				Conditions: ConditionNode{Attribute: &attr, Op: &op, Value: "x"},
				Rollout:    &RolloutConfig{Percentage: 0},
				Value:      "rollout",
			},
			{
				Conditions: ConditionNode{Attribute: &attr, Op: &op, Value: "x"},
				Value:      "fallback",
			},
		},
	}
	req := EvaluateRequest{FlagConfig: fc, Context: map[string]any{"a": "x", "user_id": "user-1"}}
	res := e.Evaluate(req)
	if res.Value != "fallback" || res.RuleIndex != 1 {
		t.Fatalf("want fallback rule1, got %v index %d", res.Value, res.RuleIndex)
	}
}

func TestEngine_EvaluateAll(t *testing.T) {
	e := New(zap.NewNop())
	attr := "a"
	op := "eq"
	flags := []FlagConfig{
		{Key: "f1", Enabled: true, DefaultValue: false,
			Rules: []Rule{{Conditions: ConditionNode{Attribute: &attr, Op: &op, Value: "x"}, Value: true}}},
		{Key: "f2", Enabled: false},
	}
	results := e.EvaluateAll(flags, nil, map[string]any{"a": "x"})
	if len(results) != 2 {
		t.Fatalf("want 2 results, got %d", len(results))
	}
	if results["f1"].Value != true {
		t.Fatal("f1 should match")
	}
	if results["f2"].Reason != ReasonDisabled {
		t.Fatal("f2 should be disabled")
	}
}

func TestEngine_ResilientEvaluateAllEmpty(t *testing.T) {
	e := New(zap.NewNop())
	results := e.EvaluateAll(nil, nil, nil)
	if results == nil {
		t.Fatal("EvaluateAll should return empty map, not nil")
	}
	if len(results) != 0 {
		t.Fatalf("want 0 results, got %d", len(results))
	}
}

func strPtr(s string) *string { return &s }
