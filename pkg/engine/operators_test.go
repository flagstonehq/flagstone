package engine

import (
	"testing"

	"go.uber.org/zap"
)

func e() *Engine { return New(zap.NewNop()) }

func TestOp_exists(t *testing.T) {
	node := ConditionNode{Op: strPtr("exists"), Attribute: strPtr("email")}
	if !e().evaluateNode(node, map[string]any{"email": "a@b.com"}, nil, nil) {
		t.Fatal("exists must be true")
	}
	if e().evaluateNode(node, map[string]any{}, nil, nil) {
		t.Fatal("exists must be false")
	}
}

func TestOp_not_exists(t *testing.T) {
	node := ConditionNode{Op: strPtr("not_exists"), Attribute: strPtr("missing")}
	if !e().evaluateNode(node, map[string]any{}, nil, nil) {
		t.Fatal("not_exists must be true when attr absent")
	}
	if e().evaluateNode(node, map[string]any{"missing": "x"}, nil, nil) {
		t.Fatal("not_exists must be false when attr present")
	}
}

func TestOp_eq(t *testing.T) {
	attr := "a"
	node := ConditionNode{Op: strPtr("eq"), Attribute: &attr, Value: "x"}
	if !e().evaluateNode(node, map[string]any{"a": "x"}, nil, nil) {
		t.Fatal("eq string match must be true")
	}
	if e().evaluateNode(node, map[string]any{"a": "y"}, nil, nil) {
		t.Fatal("eq no match must be false")
	}
	if e().evaluateNode(node, map[string]any{}, nil, nil) {
		t.Fatal("eq missing attr must be false")
	}
}

func TestOp_neq(t *testing.T) {
	attr := "a"
	node := ConditionNode{Op: strPtr("neq"), Attribute: &attr, Value: "x"}
	if !e().evaluateNode(node, map[string]any{"a": "y"}, nil, nil) {
		t.Fatal("neq different must be true")
	}
	if e().evaluateNode(node, map[string]any{"a": "x"}, nil, nil) {
		t.Fatal("neq same must be false")
	}
	if e().evaluateNode(node, map[string]any{}, nil, nil) {
		t.Fatal("neq missing attr must be false")
	}
}

func TestOp_gt(t *testing.T) {
	attr := "age"
	node := ConditionNode{Op: strPtr("gt"), Attribute: &attr, Value: float64(18)}
	if !e().evaluateNode(node, map[string]any{"age": float64(25)}, nil, nil) {
		t.Fatal("gt 25>18 must be true")
	}
	if e().evaluateNode(node, map[string]any{"age": float64(10)}, nil, nil) {
		t.Fatal("gt 10>18 must be false")
	}
	if e().evaluateNode(node, map[string]any{}, nil, nil) {
		t.Fatal("gt missing attr must be false")
	}
	if e().evaluateNode(node, map[string]any{"age": "abc"}, nil, nil) {
		t.Fatal("gt type mismatch must be false")
	}
}

func TestOp_gte(t *testing.T) {
	attr := "age"
	node := ConditionNode{Op: strPtr("gte"), Attribute: &attr, Value: float64(18)}
	if !e().evaluateNode(node, map[string]any{"age": float64(18)}, nil, nil) {
		t.Fatal("gte 18>=18 must be true")
	}
	if e().evaluateNode(node, map[string]any{"age": float64(10)}, nil, nil) {
		t.Fatal("gte 10>=18 must be false")
	}
}

func TestOp_lt(t *testing.T) {
	attr := "age"
	node := ConditionNode{Op: strPtr("lt"), Attribute: &attr, Value: float64(18)}
	if !e().evaluateNode(node, map[string]any{"age": float64(10)}, nil, nil) {
		t.Fatal("lt 10<18 must be true")
	}
	if e().evaluateNode(node, map[string]any{"age": float64(25)}, nil, nil) {
		t.Fatal("lt 25<18 must be false")
	}
}

func TestOp_lte(t *testing.T) {
	attr := "age"
	node := ConditionNode{Op: strPtr("lte"), Attribute: &attr, Value: float64(18)}
	if !e().evaluateNode(node, map[string]any{"age": float64(18)}, nil, nil) {
		t.Fatal("lte 18<=18 must be true")
	}
	if e().evaluateNode(node, map[string]any{"age": float64(25)}, nil, nil) {
		t.Fatal("lte 25<=18 must be false")
	}
}

func TestOp_in(t *testing.T) {
	attr := "country"
	node := ConditionNode{Op: strPtr("in"), Attribute: &attr, Value: []any{"ar", "br", "cl"}}
	if !e().evaluateNode(node, map[string]any{"country": "ar"}, nil, nil) {
		t.Fatal("in must match")
	}
	if e().evaluateNode(node, map[string]any{"country": "mx"}, nil, nil) {
		t.Fatal("in must not match")
	}
	if e().evaluateNode(node, map[string]any{}, nil, nil) {
		t.Fatal("in missing attr must be false")
	}
}

func TestOp_not_in(t *testing.T) {
	attr := "country"
	node := ConditionNode{Op: strPtr("not_in"), Attribute: &attr, Value: []any{"ar", "br"}}
	if !e().evaluateNode(node, map[string]any{"country": "mx"}, nil, nil) {
		t.Fatal("not_in not in list must be true")
	}
	if e().evaluateNode(node, map[string]any{"country": "ar"}, nil, nil) {
		t.Fatal("not_in in list must be false")
	}
}

func TestOp_contains(t *testing.T) {
	attr := "email"
	node := ConditionNode{Op: strPtr("contains"), Attribute: &attr, Value: "@beta.com"}
	if !e().evaluateNode(node, map[string]any{"email": "user@beta.com"}, nil, nil) {
		t.Fatal("contains substring must be true")
	}
	if e().evaluateNode(node, map[string]any{"email": "user@alpha.com"}, nil, nil) {
		t.Fatal("contains no match must be false")
	}
	if e().evaluateNode(node, map[string]any{}, nil, nil) {
		t.Fatal("contains missing attr must be false")
	}
}

func TestOp_starts_with(t *testing.T) {
	attr := "name"
	node := ConditionNode{Op: strPtr("starts_with"), Attribute: &attr, Value: "admin"}
	if !e().evaluateNode(node, map[string]any{"name": "admin_user"}, nil, nil) {
		t.Fatal("starts_with prefix must be true")
	}
	if e().evaluateNode(node, map[string]any{"name": "user_admin"}, nil, nil) {
		t.Fatal("starts_with no prefix must be false")
	}
}

func TestOp_ends_with(t *testing.T) {
	attr := "email"
	node := ConditionNode{Op: strPtr("ends_with"), Attribute: &attr, Value: "@company.com"}
	if !e().evaluateNode(node, map[string]any{"email": "user@company.com"}, nil, nil) {
		t.Fatal("ends_with suffix must be true")
	}
	if e().evaluateNode(node, map[string]any{"email": "user@other.com"}, nil, nil) {
		t.Fatal("ends_with no suffix must be false")
	}
}

func TestOp_matches(t *testing.T) {
	attr := "email"
	node := ConditionNode{Op: strPtr("matches"), Attribute: &attr, Value: `^[a-z]+@company\.com$`}
	if !e().evaluateNode(node, map[string]any{"email": "user@company.com"}, nil, nil) {
		t.Fatal("matches regex must be true")
	}
	if e().evaluateNode(node, map[string]any{"email": "USER@company.com"}, nil, nil) {
		t.Fatal("matches case sensitive must be false")
	}
}

func TestOp_matches_invalid_regex(t *testing.T) {
	attr := "email"
	node := ConditionNode{Op: strPtr("matches"), Attribute: &attr, Value: `[invalid`}
	if e().evaluateNode(node, map[string]any{"email": "test"}, nil, nil) {
		t.Fatal("matches invalid regex must be false")
	}
}

func TestOp_unknown(t *testing.T) {
	attr := "a"
	node := ConditionNode{Op: strPtr("unknown_op"), Attribute: &attr, Value: "x"}
	if e().evaluateNode(node, map[string]any{"a": "x"}, nil, nil) {
		t.Fatal("unknown operator must be false")
	}
}

func TestOp_segment_not_found(t *testing.T) {
	node := ConditionNode{Op: strPtr("segment"), Value: "nonexistent"}
	if e().evaluateNode(node, map[string]any{}, map[string]Segment{}, nil) {
		t.Fatal("segment not found must be false")
	}
}
