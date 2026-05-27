package engine

import "testing"

func TestSegmentSimpleMatch(t *testing.T) {
	op, attr := "eq", "plan"
	segments := map[string]Segment{
		"premium-users": {
			Key: "premium-users",
			Conditions: ConditionNode{
				Attribute: &attr,
				Op:        &op,
				Value:     "premium",
			},
		},
	}
	resolver := segmentResolver{
		segments: segments,
		context:  map[string]any{"plan": "premium"},
	}
	if !resolver.matches("premium-users") {
		t.Fatal("segment must match when its condition is met")
	}
}

func TestSegmentNoMatch(t *testing.T) {
	op, attr := "eq", "plan"
	segments := map[string]Segment{
		"premium-users": {
			Key: "premium-users",
			Conditions: ConditionNode{
				Attribute: &attr,
				Op:        &op,
				Value:     "premium",
			},
		},
	}
	resolver := segmentResolver{
		segments: segments,
		context:  map[string]any{"plan": "free"},
	}
	if resolver.matches("premium-users") {
		t.Fatal("segment must not match when its condition is not met")
	}
}

func TestSegmentNotFound(t *testing.T) {
	resolver := segmentResolver{
		segments: map[string]Segment{},
		context:  map[string]any{},
	}
	if resolver.matches("nobody") {
		t.Fatal("missing segment must return false")
	}
}

func TestSegmentCycleDetected(t *testing.T) {
	op, attr := "segment", "ignored"
	segments := map[string]Segment{
		"a": {
			Key: "a",
			Conditions: ConditionNode{
				Attribute: &attr,
				Op:        &op,
				Value:     "b",
			},
		},
		"b": {
			Key: "b",
			Conditions: ConditionNode{
				Attribute: &attr,
				Op:        &op,
				Value:     "a",
			},
		},
	}
	resolver := segmentResolver{
		segments: segments,
		context:  map[string]any{},
	}
	if resolver.matches("a") {
		t.Fatal("cycle must evaluate to false")
	}
}

func TestSegmentIndirectCycle(t *testing.T) {
	op, attr := "segment", "ignored"
	segments := map[string]Segment{
		"a": {
			Key: "a",
			Conditions: ConditionNode{
				Attribute: &attr,
				Op:        &op,
				Value:     "b",
			},
		},
		"b": {
			Key: "b",
			Conditions: ConditionNode{
				Attribute: &attr,
				Op:        &op,
				Value:     "c",
			},
		},
		"c": {
			Key: "c",
			Conditions: ConditionNode{
				Attribute: &attr,
				Op:        &op,
				Value:     "a",
			},
		},
	}
	resolver := segmentResolver{
		segments: segments,
		context:  map[string]any{},
	}
	if resolver.matches("a") {
		t.Fatal("indirect cycle a→b→c→a must evaluate to false")
	}
}
