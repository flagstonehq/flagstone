package engine

import (
	"strings"
)

func evaluateLeaf(node ConditionNode, context map[string]any, segments map[string]Segment, visited map[string]struct{}) bool {
	if node.Op == nil {
		return false
	}
	op := strings.TrimSpace(*node.Op)
	attr := ""
	if node.Attribute != nil {
		attr = strings.TrimSpace(*node.Attribute)
	}
	value, exists := context[attr]
	switch op {
	case "exists":
		return exists
	case "eq":
		if !exists {
			return false
		}
		return equalStrict(value, node.Value)
	case "neq":
		if !exists {
			return false
		}
		return !equalStrict(value, node.Value)
	case "gt":
		if !exists {
			return false
		}
		left, ok1 := asFloat64(value)
		right, ok2 := asFloat64(node.Value)
		if !ok1 || !ok2 {
			return false
		}
		return left > right
	case "contains":
		if !exists {
			return false
		}
		left, ok1 := value.(string)
		right, ok2 := node.Value.(string)
		if !ok1 || !ok2 {
			return false
		}
		return strings.Contains(left, right)
	case "in":
		if !exists {
			return false
		}
		items, ok := node.Value.([]any)
		if !ok {
			return false
		}
		for _, item := range items {
			if equalStrict(value, item) {
				return true
			}
		}
		return false
	case "segment":
		segmentKey, ok := node.Value.(string)
		if !ok {
			return false
		}
		return segmentResolver{
			segments: segments,
			context:  context,
		}.matchesVisited(segmentKey, visited)
	default:
		return false
	}
}

func equalStrict(left, right any) bool {
	switch l := left.(type) {
	case string:
		r, ok := right.(string)
		return ok && l == r
	case bool:
		r, ok := right.(bool)
		return ok && l == r
	case float64:
		r, ok := asFloat64(right)
		return ok && l == r
	case float32:
		r, ok := asFloat64(right)
		return ok && float64(l) == r
	case int:
		r, ok := asFloat64(right)
		return ok && float64(l) == r
	case int8:
		r, ok := asFloat64(right)
		return ok && float64(l) == r
	case int16:
		r, ok := asFloat64(right)
		return ok && float64(l) == r
	case int32:
		r, ok := asFloat64(right)
		return ok && float64(l) == r
	case int64:
		r, ok := asFloat64(right)
		return ok && float64(l) == r
	case uint:
		r, ok := asFloat64(right)
		return ok && float64(l) == r
	case uint8:
		r, ok := asFloat64(right)
		return ok && float64(l) == r
	case uint16:
		r, ok := asFloat64(right)
		return ok && float64(l) == r
	case uint32:
		r, ok := asFloat64(right)
		return ok && float64(l) == r
	case uint64:
		r, ok := asFloat64(right)
		return ok && float64(l) == r
	default:
		return false
	}
}

func asFloat64(v any) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case float32:
		return float64(n), true
	case int:
		return float64(n), true
	case int8:
		return float64(n), true
	case int16:
		return float64(n), true
	case int32:
		return float64(n), true
	case int64:
		return float64(n), true
	case uint:
		return float64(n), true
	case uint8:
		return float64(n), true
	case uint16:
		return float64(n), true
	case uint32:
		return float64(n), true
	case uint64:
		return float64(n), true
	default:
		return 0, false
	}
}
