package engine

import (
	"testing"

	"go.uber.org/zap"
)

func TestSegment_Match(t *testing.T) {
	e := New(zap.NewNop())
	attr := "country"
	op := "in"
	seg := Segment{
		Key: "latin_america",
		Conditions: ConditionNode{
			Attribute: &attr,
			Op:        &op,
			Value:     []any{"ar", "br", "cl"},
		},
	}
	cond := ConditionNode{
		Op:    strPtr("segment"),
		Value: "latin_america",
	}
	ctx := map[string]any{"country": "ar"}
	if !e.evaluateNode(cond, ctx, map[string]Segment{"latin_america": seg}, nil) {
		t.Fatal("segment with matching condition must be true")
	}
}

func TestSegment_NoMatch(t *testing.T) {
	e := New(zap.NewNop())
	attr := "country"
	op := "in"
	seg := Segment{
		Key: "latin_america",
		Conditions: ConditionNode{
			Attribute: &attr,
			Op:        &op,
			Value:     []any{"ar", "br", "cl"},
		},
	}
	cond := ConditionNode{
		Op:    strPtr("segment"),
		Value: "latin_america",
	}
	ctx := map[string]any{"country": "mx"}
	if e.evaluateNode(cond, ctx, map[string]Segment{"latin_america": seg}, nil) {
		t.Fatal("segment with non-matching condition must be false")
	}
}

func TestSegment_Missing(t *testing.T) {
	e := New(zap.NewNop())
	cond := ConditionNode{
		Op:    strPtr("segment"),
		Value: "nonexistent",
	}
	if e.evaluateNode(cond, map[string]any{}, map[string]Segment{}, nil) {
		t.Fatal("missing segment must be false")
	}
}

func TestSegment_Circular(t *testing.T) {
	e := New(zap.NewNop())
	seg1 := Segment{
		Key: "seg_a",
		Conditions: ConditionNode{
			Op:    strPtr("segment"),
			Value: "seg_b",
		},
	}
	seg2 := Segment{
		Key: "seg_b",
		Conditions: ConditionNode{
			Op:    strPtr("segment"),
			Value: "seg_a",
		},
	}
	cond := ConditionNode{
		Op:    strPtr("segment"),
		Value: "seg_a",
	}
	segs := map[string]Segment{"seg_a": seg1, "seg_b": seg2}
	if e.evaluateNode(cond, map[string]any{}, segs, nil) {
		t.Fatal("circular segment must be false")
	}
}

func TestSegment_Nested(t *testing.T) {
	e := New(zap.NewNop())
	attr := "role"
	op := "eq"

	admin := Segment{
		Key: "admin",
		Conditions: ConditionNode{
			Attribute: &attr,
			Op:        &op,
			Value:     "admin",
		},
	}
	internal := Segment{
		Key: "internal",
		Conditions: ConditionNode{
			All: []ConditionNode{
				{Op: strPtr("segment"), Value: "admin"},
				{Attribute: strPtr("email"), Op: strPtr("ends_with"), Value: "@company.com"},
			},
		},
	}
	cond := ConditionNode{Op: strPtr("segment"), Value: "internal"}
	segs := map[string]Segment{"admin": admin, "internal": internal}
	ctx := map[string]any{"role": "admin", "email": "admin@company.com"}
	if !e.evaluateNode(cond, ctx, segs, nil) {
		t.Fatal("nested segment must be true")
	}
}
