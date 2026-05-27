package engine

func evaluateNode(node ConditionNode, context map[string]any, segments map[string]Segment, visited map[string]struct{}) bool {
	if len(node.All) > 0 {
		for _, child := range node.All {
			if !evaluateNode(child, context, segments, visited) {
				return false
			}
		}
		return true
	}

	if len(node.Any) > 0 {
		for _, child := range node.Any {
			if evaluateNode(child, context, segments, visited) {
				return true
			}
		}
		return false
	}

	if node.Not != nil {
		return !evaluateNode(*node.Not, context, segments, visited)
	}

	return evaluateLeaf(node, context, segments, visited)
}
