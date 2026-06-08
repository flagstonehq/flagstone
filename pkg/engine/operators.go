package engine

import (
	"regexp"
	"strings"

	"go.uber.org/zap"
)

func (e *Engine) evaluateLeaf(node ConditionNode, ctx map[string]any, segments map[string]Segment, visited map[string]struct{}) bool {
	if node.Op == nil {
		return false
	}
	op := strings.TrimSpace(*node.Op)
	attr := ""
	if node.Attribute != nil {
		attr = strings.TrimSpace(*node.Attribute)
	}
	val, exists := ctx[attr]

	switch op {
	case "exists":
		return exists
	case "not_exists":
		return !exists
	case "eq":
		if !exists {
			return false
		}
		return equalStrict(val, node.Value)
	case "neq":
		if !exists {
			return false
		}
		return !equalStrict(val, node.Value)
	case "gt":
		return e.compareNumber(val, node.Value, exists, func(l, r float64) bool { return l > r })
	case "gte":
		return e.compareNumber(val, node.Value, exists, func(l, r float64) bool { return l >= r })
	case "lt":
		return e.compareNumber(val, node.Value, exists, func(l, r float64) bool { return l < r })
	case "lte":
		return e.compareNumber(val, node.Value, exists, func(l, r float64) bool { return l <= r })
	case "in":
		if !exists {
			return false
		}
		items, ok := node.Value.([]any)
		if !ok {
			return false
		}
		for _, item := range items {
			if equalStrict(val, item) {
				return true
			}
		}
		return false
	case "not_in":
		if !exists {
			return false
		}
		items, ok := node.Value.([]any)
		if !ok {
			return false
		}
		for _, item := range items {
			if equalStrict(val, item) {
				return false
			}
		}
		return true
	case "contains":
		if !exists {
			return false
		}
		left, ok1 := val.(string)
		right, ok2 := node.Value.(string)
		if !ok1 || !ok2 {
			return false
		}
		return strings.Contains(left, right)
	case "starts_with":
		if !exists {
			return false
		}
		left, ok1 := val.(string)
		right, ok2 := node.Value.(string)
		if !ok1 || !ok2 {
			return false
		}
		return strings.HasPrefix(left, right)
	case "ends_with":
		if !exists {
			return false
		}
		left, ok1 := val.(string)
		right, ok2 := node.Value.(string)
		if !ok1 || !ok2 {
			return false
		}
		return strings.HasSuffix(left, right)
	case "matches":
		if !exists {
			return false
		}
		left, ok1 := val.(string)
		pattern, ok2 := node.Value.(string)
		if !ok1 || !ok2 {
			return false
		}
		matched, err := regexp.MatchString(pattern, left)
		if err != nil {
			e.logger.Error("matches operator: invalid regex",
				zap.String("pattern", pattern),
				zap.Error(err),
			)
			if e.OnError != nil {
				e.OnError(ErrorKindInvalidRegex)
			}
			return false
		}
		return matched
	case "segment":
		segmentKey, ok := node.Value.(string)
		if !ok {
			return false
		}
		return e.resolveSegment(segmentKey, ctx, segments, visited)
	default:
		e.logger.Warn("unknown operator",
			zap.String("operator", op),
			zap.String("attribute", attr),
		)
		if e.OnWarn != nil {
			e.OnWarn(WarnKindUnknownOp)
		}
		return false
	}
}

func (e *Engine) compareNumber(val, ruleVal any, exists bool, cmp func(float64, float64) bool) bool {
	if !exists {
		return false
	}
	left, ok1 := asFloat64(val)
	right, ok2 := asFloat64(ruleVal)
	if !ok1 || !ok2 {
		return false
	}
	return cmp(left, right)
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
