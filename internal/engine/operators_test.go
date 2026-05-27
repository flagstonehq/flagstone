package engine

import (
	"testing"
)

func TestEqualStrict_DifferentLogicalTypes(t *testing.T) {
	if equalStrict("5", 5) {
		t.Fatal(`"5" and 5 must not match`)
	}
}

func TestEqualStrict_NumericFamily(t *testing.T) {
	if !equalStrict(int(5), float64(5)) {
		t.Fatal("int(5) and float64(5) must match (numeric normalization)")
	}
	if !equalStrict(float64(5), int(5)) {
		t.Fatal("float64(5) and int(5) must match (numeric normalization)")
	}
}

func TestEqualStrict_String(t *testing.T) {
	if !equalStrict("hello", "hello") {
		t.Fatal("identical strings must match")
	}
	if equalStrict("hello", "world") {
		t.Fatal("different strings must not match")
	}
}

func TestEqualStrict_Bool(t *testing.T) {
	if !equalStrict(true, true) {
		t.Fatal("true must match true")
	}
	if equalStrict(true, false) {
		t.Fatal("true must not match false")
	}
}

func TestEqualStrict_Nil(t *testing.T) {
	if equalStrict(nil, nil) {
		t.Fatal("nil should not match nil (not a supported type)")
	}
}

func TestEvaluateLeaf_Exists(t *testing.T) {
	op, attr := "exists", "country"
	ok := evaluateLeaf(ConditionNode{
		Attribute: &attr,
		Op:        &op,
	}, map[string]any{"country": "AR"}, nil, nil)
	if !ok {
		t.Fatal("exists must be true when attribute is present")
	}
}

func TestEvaluateLeaf_NotExists(t *testing.T) {
	op, attr := "exists", "missing"
	ok := evaluateLeaf(ConditionNode{
		Attribute: &attr,
		Op:        &op,
	}, map[string]any{"country": "AR"}, nil, nil)
	if ok {
		t.Fatal("exists must be false when attribute is missing")
	}
}

func TestEvaluateLeaf_Eq(t *testing.T) {
	op, attr := "eq", "country"
	ok := evaluateLeaf(ConditionNode{
		Attribute: &attr,
		Op:        &op,
		Value:     "AR",
	}, map[string]any{"country": "AR"}, nil, nil)
	if !ok {
		t.Fatal(`eq "AR" must match context["country"]="AR"`)
	}
}

func TestEvaluateLeaf_EqStrictCrossType(t *testing.T) {
	op, attr := "eq", "age"
	ok := evaluateLeaf(ConditionNode{
		Attribute: &attr,
		Op:        &op,
		Value:     "5",
	}, map[string]any{"age": 5}, nil, nil)
	if ok {
		t.Fatal(`eq "5" must NOT match int(5)`)
	}
}

func TestEvaluateLeaf_Gt(t *testing.T) {
	op, attr := "gt", "age"
	ok := evaluateLeaf(ConditionNode{
		Attribute: &attr,
		Op:        &op,
		Value:     18,
	}, map[string]any{"age": 25}, nil, nil)
	if !ok {
		t.Fatal("gt 18 must match age=25")
	}
}

func TestEvaluateLeaf_GtString(t *testing.T) {
	op, attr := "gt", "age"
	ok := evaluateLeaf(ConditionNode{
		Attribute: &attr,
		Op:        &op,
		Value:     18,
	}, map[string]any{"age": "25"}, nil, nil)
	if ok {
		t.Fatal("gt against string value must return false (no coercion)")
	}
}

func TestEvaluateLeaf_Contains(t *testing.T) {
	op, attr := "contains", "email"
	ok := evaluateLeaf(ConditionNode{
		Attribute: &attr,
		Op:        &op,
		Value:     "@company.com",
	}, map[string]any{"email": "user@company.com"}, nil, nil)
	if !ok {
		t.Fatal(`contains "@company.com" must match "user@company.com"`)
	}
}

func TestEvaluateLeaf_In(t *testing.T) {
	op, attr := "in", "country"
	ok := evaluateLeaf(ConditionNode{
		Attribute: &attr,
		Op:        &op,
		Value:     []any{"AR", "BR", "CL"},
	}, map[string]any{"country": "AR"}, nil, nil)
	if !ok {
		t.Fatal(`in ["AR","BR","CL"] must match "AR"`)
	}
}

func TestEvaluateLeaf_InUnknownOperator(t *testing.T) {
	op, attr := "unknown", "x"
	ok := evaluateLeaf(ConditionNode{
		Attribute: &attr,
		Op:        &op,
		Value:     "y",
	}, map[string]any{"x": "y"}, nil, nil)
	if ok {
		t.Fatal("unknown operator must return false")
	}
}

func TestEvaluateLeaf_NilOp(t *testing.T) {
	attr := "x"
	ok := evaluateLeaf(ConditionNode{
		Attribute: &attr,
	}, map[string]any{"x": "y"}, nil, nil)
	if ok {
		t.Fatal("nil op must return false")
	}
}
