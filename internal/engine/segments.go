package engine

func (e *Engine) resolveSegment(segmentKey string, ctx map[string]any, segments map[string]Segment, visited map[string]struct{}) bool {
	if _, seen := visited[segmentKey]; seen {
		return false
	}

	segment, ok := segments[segmentKey]
	if !ok {
		return false
	}

	nextVisited := make(map[string]struct{}, len(visited)+1)
	for k := range visited {
		nextVisited[k] = struct{}{}
	}
	nextVisited[segmentKey] = struct{}{}

	return e.evaluateNode(segment.Conditions, ctx, segments, nextVisited)
}
