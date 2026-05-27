package engine

import "testing"

func TestConditions_All(t *testing.T) {
	attrA, attrB := "a", "b"
	op := "eq"
	node := ConditionNode{
		All: []ConditionNode{
			{Attribute: &attrA, Op: &op, Value: "x"},
			{Attribute: &attrB, Op: &op, Value: "y"},
		},
	}
	ctx := map[string]any{"a": "x", "b": "y"}
	ok := evaluateNode(node, ctx, nil, nil)
	if !ok {
		t.Fatal("all matched must return true")
	}
}

func TestConditions_AllOneFails(t *testing.T) {
	attrA, attrB := "a", "b"
	op := "eq"
	node := ConditionNode{
		All: []ConditionNode{
			{Attribute: &attrA, Op: &op, Value: "x"},
			{Attribute: &attrB, Op: &op, Value: "z"},
		},
	}
	ctx := map[string]any{"a": "x", "b": "y"}
	ok := evaluateNode(node, ctx, nil, nil)
	if ok {
		t.Fatal("all with one failing must return false")
	}
}

func TestConditions_Any(t *testing.T) {
	attrA, attrB := "a", "b"
	op := "eq"
	node := ConditionNode{
		Any: []ConditionNode{
			{Attribute: &attrA, Op: &op, Value: "x"},
			{Attribute: &attrB, Op: &op, Value: "y"},
		},
	}
	ctx := map[string]any{"a": "x", "b": "z"}
	ok := evaluateNode(node, ctx, nil, nil)
	if !ok {
		t.Fatal("any with at least one match must return true")
	}
}

func TestConditions_AnyAllFail(t *testing.T) {
	attrA, attrB := "a", "b"
	op := "eq"
	node := ConditionNode{
		Any: []ConditionNode{
			{Attribute: &attrA, Op: &op, Value: "w"},
			{Attribute: &attrB, Op: &op, Value: "z"},
		},
	}
	ctx := map[string]any{"a": "x", "b": "y"}
	ok := evaluateNode(node, ctx, nil, nil)
	if ok {
		t.Fatal("any with no matches must return false")
	}
}

func TestConditions_Not(t *testing.T) {
	attr := "a"
	op := "eq"
	node := ConditionNode{
		Not: &ConditionNode{Attribute: &attr, Op: &op, Value: "x"},
	}

	ctxMatch := map[string]any{"a": "y"}
	ok := evaluateNode(node, ctxMatch, nil, nil)
	if !ok {
		t.Fatal("not eq must invert false to true")
	}

	ctxNoMatch := map[string]any{"a": "x"}
	notOk := evaluateNode(node, ctxNoMatch, nil, nil)
	if notOk {
		t.Fatal("not eq must invert true to false")
	}
}

func TestConditions_NestedAllAny(t *testing.T) {
	attrA, attrB, attrC := "a", "b", "c"
	op := "eq"
	node := ConditionNode{
		All: []ConditionNode{
			{Attribute: &attrA, Op: &op, Value: "x"},
			{
				Any: []ConditionNode{
					{Attribute: &attrB, Op: &op, Value: "y"},
					{Attribute: &attrC, Op: &op, Value: "z"},
				},
			},
		},
	}
	ctx := map[string]any{"a": "x", "b": "nope", "c": "z"}
	ok := evaluateNode(node, ctx, nil, nil)
	if !ok {
		t.Fatal("nested all/any must evaluate correctly")
	}
}
