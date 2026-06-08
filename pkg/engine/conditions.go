package engine

const maxConditionDepth = 10

func (e *Engine) evaluateNode(node ConditionNode, ctx map[string]any, segments map[string]Segment, visited map[string]struct{}) bool {
	return e.evaluateNodeDepth(node, ctx, segments, visited, 0)
}

func (e *Engine) evaluateNodeDepth(node ConditionNode, ctx map[string]any, segments map[string]Segment, visited map[string]struct{}, depth int) bool {
	if depth > maxConditionDepth {
		if e.OnWarn != nil {
			e.OnWarn(WarnKindMaxConditionDepth)
		}
		return false
	}
	nextDepth := depth + 1

	if len(node.All) > 0 {
		for _, child := range node.All {
			if !e.evaluateNodeDepth(child, ctx, segments, visited, nextDepth) {
				return false
			}
		}
		return true
	}

	if len(node.Any) > 0 {
		for _, child := range node.Any {
			if e.evaluateNodeDepth(child, ctx, segments, visited, nextDepth) {
				return true
			}
		}
		return false
	}

	if node.Not != nil {
		return !e.evaluateNodeDepth(*node.Not, ctx, segments, visited, nextDepth)
	}

	return e.evaluateLeaf(node, ctx, segments, visited)
}
