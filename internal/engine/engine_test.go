package engine

import (
	"testing"
)

func TestEvaluate_FirstRuleWins(t *testing.T) {
	op, attr := "eq", "country"
	flag := FlagConfig{
		Key:          "checkout",
		Enabled:      true,
		FlagType:     "boolean",
		DefaultValue: false,
		Rules: []Rule{
			{
				Conditions: ConditionNode{Attribute: &attr, Op: &op, Value: "AR"},
				Value:      true,
			},
			{
				Conditions: ConditionNode{Attribute: &attr, Op: &op, Value: "AR"},
				Value:      false,
			},
		},
	}
	got := Evaluate(flag, map[string]any{"country": "AR"}, nil)
	if got.Reason != ReasonRuleMatch {
		t.Fatalf("expected RULE_MATCH, got %s", got.Reason)
	}
	if got.RuleIndex != 0 {
		t.Fatalf("expected first rule (0), got %d", got.RuleIndex)
	}
	if got.Value != true {
		t.Fatalf("expected true, got %#v", got.Value)
	}
}

func TestEvaluate_SecondRuleMatches(t *testing.T) {
	op, attr := "eq", "country"
	flag := FlagConfig{
		Key:          "checkout",
		Enabled:      true,
		FlagType:     "boolean",
		DefaultValue: false,
		Rules: []Rule{
			{
				Conditions: ConditionNode{Attribute: &attr, Op: &op, Value: "BR"},
				Value:      true,
			},
			{
				Conditions: ConditionNode{Attribute: &attr, Op: &op, Value: "AR"},
				Value:      true,
			},
		},
	}
	got := Evaluate(flag, map[string]any{"country": "AR"}, nil)
	if got.Reason != ReasonRuleMatch {
		t.Fatalf("expected RULE_MATCH, got %s", got.Reason)
	}
	if got.RuleIndex != 1 {
		t.Fatalf("expected second rule (1), got %d", got.RuleIndex)
	}
}

func TestEvaluate_DefaultValue(t *testing.T) {
	flag := FlagConfig{
		Key:          "checkout",
		Enabled:      true,
		FlagType:     "boolean",
		DefaultValue: false,
	}
	got := Evaluate(flag, map[string]any{}, nil)
	if got.Reason != ReasonDefault {
		t.Fatalf("expected DEFAULT, got %s", got.Reason)
	}
	if got.Value != false {
		t.Fatalf("expected false, got %#v", got.Value)
	}
}

func TestEvaluate_Disabled(t *testing.T) {
	flag := FlagConfig{
		Key:     "checkout",
		Enabled: false,
	}
	got := Evaluate(flag, nil, nil)
	if got.Reason != ReasonDisabled {
		t.Fatalf("expected DISABLED, got %s", got.Reason)
	}
	if got.Value != false {
		t.Fatalf("expected false, got %#v", got.Value)
	}
}

func TestEvaluate_EnvDefaultOverridesFlagDefault(t *testing.T) {
	flag := FlagConfig{
		Key:                     "checkout",
		Enabled:                 true,
		FlagType:                "boolean",
		DefaultValue:            false,
		EnvironmentDefaultValue: true,
		HasEnvironmentDefault:   true,
	}
	got := Evaluate(flag, map[string]any{}, nil)
	if got.Reason != ReasonDefault {
		t.Fatalf("expected DEFAULT, got %s", got.Reason)
	}
	if got.Value != true {
		t.Fatalf("expected true (env default), got %#v", got.Value)
	}
}

func TestEvaluate_RolloutHit(t *testing.T) {
	op, attr := "eq", "country"
	flag := FlagConfig{
		Key:      "dark-mode",
		Enabled:  true,
		FlagType: "boolean",
		Rules: []Rule{
			{
				Conditions: ConditionNode{Attribute: &attr, Op: &op, Value: "AR"},
				Rollout:    &RolloutConfig{Percentage: 100},
				Value:      true,
			},
		},
	}
	got := Evaluate(flag, map[string]any{"country": "AR", "user_id": "u123"}, nil)
	if got.Reason != ReasonRuleMatch {
		t.Fatalf("expected RULE_MATCH, got %s", got.Reason)
	}
}

func TestEvaluate_RolloutMissFallsThrough(t *testing.T) {
	op, attr := "eq", "country"
	attr2 := "is_admin"
	op2 := "eq"
	flag := FlagConfig{
		Key:          "dark-mode",
		Enabled:      true,
		FlagType:     "boolean",
		DefaultValue: false,
		Rules: []Rule{
			{
				Conditions: ConditionNode{Attribute: &attr, Op: &op, Value: "AR"},
				Rollout:    &RolloutConfig{Percentage: 0},
				Value:      true,
			},
			{
				Conditions: ConditionNode{Attribute: &attr2, Op: &op2, Value: true},
				Value:      true,
			},
		},
	}
	got := Evaluate(flag, map[string]any{
		"country":  "AR",
		"user_id":  "u123",
		"is_admin": true,
	}, nil)
	if got.Reason != ReasonRuleMatch {
		t.Fatalf("expected RULE_MATCH (fallthrough to admin rule), got %s", got.Reason)
	}
	if got.RuleIndex != 1 {
		t.Fatalf("expected second rule to match, got %d", got.RuleIndex)
	}
}

func TestEvaluate_RolloutWithoutUserIDSkipsRule(t *testing.T) {
	op, attr := "eq", "country"
	flag := FlagConfig{
		Key:          "dark-mode",
		Enabled:      true,
		FlagType:     "boolean",
		DefaultValue: false,
		Rules: []Rule{
			{
				Conditions: ConditionNode{Attribute: &attr, Op: &op, Value: "AR"},
				Rollout:    &RolloutConfig{Percentage: 50},
				Value:      true,
			},
		},
	}
	got := Evaluate(flag, map[string]any{"country": "AR"}, nil)
	if got.Reason != ReasonDefault {
		t.Fatalf("expected DEFAULT (no user_id, rule skipped), got %s", got.Reason)
	}
}

func TestEvaluate_RolloutWithSeed(t *testing.T) {
	op, attr := "eq", "country"
	flag := FlagConfig{
		Key:     "dark-mode",
		Enabled: true,
		Rules: []Rule{
			{
				Conditions: ConditionNode{Attribute: &attr, Op: &op, Value: "AR"},
				Rollout:    &RolloutConfig{Percentage: 100, Seed: "custom-seed"},
				Value:      true,
			},
		},
	}
	got := Evaluate(flag, map[string]any{"country": "AR", "user_id": "u123"}, nil)
	if got.Reason != ReasonRuleMatch {
		t.Fatalf("expected RULE_MATCH with custom seed, got %s", got.Reason)
	}
}

func TestEvaluate_RuleWithoutValueDefaultsToTrue(t *testing.T) {
	op, attr := "eq", "country"
	flag := FlagConfig{
		Key:     "flag",
		Enabled: true,
		Rules: []Rule{
			{
				Conditions: ConditionNode{Attribute: &attr, Op: &op, Value: "AR"},
			},
		},
	}
	got := Evaluate(flag, map[string]any{"country": "AR"}, nil)
	if got.Value != true {
		t.Fatalf("expected true (default value for match), got %#v", got.Value)
	}
}
