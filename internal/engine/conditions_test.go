package engine

import (
	"testing"

	"go.uber.org/zap"
)

func TestConditions_All(t *testing.T) {
	e := New(zap.NewNop())
	attrA, attrB := "a", "b"
	op := "eq"
	node := ConditionNode{
		All: []ConditionNode{
			{Attribute: &attrA, Op: &op, Value: "x"},
			{Attribute: &attrB, Op: &op, Value: "y"},
		},
	}
	ctx := map[string]any{"a": "x", "b": "y"}
	if !e.evaluateNode(node, ctx, nil, nil) {
		t.Fatal("all matched must return true")
	}
}

func TestConditions_AllOneFails(t *testing.T) {
	e := New(zap.NewNop())
	attrA, attrB := "a", "b"
	op := "eq"
	node := ConditionNode{
		All: []ConditionNode{
			{Attribute: &attrA, Op: &op, Value: "x"},
			{Attribute: &attrB, Op: &op, Value: "z"},
		},
	}
	ctx := map[string]any{"a": "x", "b": "y"}
	if e.evaluateNode(node, ctx, nil, nil) {
		t.Fatal("all with one failing must return false")
	}
}

func TestConditions_AllShortCircuit(t *testing.T) {
	e := New(zap.NewNop())
	attrA, attrB := "a", "b"
	opEq, opContains := "eq", "contains"
	node := ConditionNode{
		All: []ConditionNode{
			{Attribute: &attrA, Op: &opEq, Value: "wrong"},
			{Attribute: &attrB, Op: &opContains, Value: "should not eval"},
		},
	}
	ctx := map[string]any{"a": "x", "b": "y"}
	if e.evaluateNode(node, ctx, nil, nil) {
		t.Fatal("all must short-circuit on first false")
	}
}

func TestConditions_Any(t *testing.T) {
	e := New(zap.NewNop())
	attrA, attrB := "a", "b"
	op := "eq"
	node := ConditionNode{
		Any: []ConditionNode{
			{Attribute: &attrA, Op: &op, Value: "x"},
			{Attribute: &attrB, Op: &op, Value: "y"},
		},
	}
	ctx := map[string]any{"a": "x", "b": "z"}
	if !e.evaluateNode(node, ctx, nil, nil) {
		t.Fatal("any with at least one match must return true")
	}
}

func TestConditions_AnyAllFail(t *testing.T) {
	e := New(zap.NewNop())
	attrA, attrB := "a", "b"
	op := "eq"
	node := ConditionNode{
		Any: []ConditionNode{
			{Attribute: &attrA, Op: &op, Value: "w"},
			{Attribute: &attrB, Op: &op, Value: "z"},
		},
	}
	ctx := map[string]any{"a": "x", "b": "y"}
	if e.evaluateNode(node, ctx, nil, nil) {
		t.Fatal("any with no matches must return false")
	}
}

func TestConditions_Not(t *testing.T) {
	e := New(zap.NewNop())
	attr := "a"
	op := "eq"
	node := ConditionNode{
		Not: &ConditionNode{Attribute: &attr, Op: &op, Value: "x"},
	}

	ctxMatch := map[string]any{"a": "y"}
	if !e.evaluateNode(node, ctxMatch, nil, nil) {
		t.Fatal("not eq must invert false to true")
	}

	ctxNoMatch := map[string]any{"a": "x"}
	if e.evaluateNode(node, ctxNoMatch, nil, nil) {
		t.Fatal("not eq must invert true to false")
	}
}

func TestConditions_NestedAllAny(t *testing.T) {
	e := New(zap.NewNop())
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
	if !e.evaluateNode(node, ctx, nil, nil) {
		t.Fatal("nested all/any must evaluate correctly")
	}
}
