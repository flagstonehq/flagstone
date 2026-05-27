package engine

type segmentResolver struct {
	segments map[string]Segment
	context  map[string]any
}

func (r segmentResolver) matches(segmentKey string) bool {
	return r.matchesVisited(segmentKey, map[string]struct{}{})
}

func (r segmentResolver) matchesVisited(segmentKey string, visited map[string]struct{}) bool {
	if _, seen := visited[segmentKey]; seen {
		return false
	}

	segment, ok := r.segments[segmentKey]
	if !ok {
		return false
	}

	nextVisited := make(map[string]struct{}, len(visited)+1)
	for k := range visited {
		nextVisited[k] = struct{}{}
	}
	nextVisited[segmentKey] = struct{}{}

	return evaluateNode(segment.Conditions, r.context, r.segments, nextVisited)
}
