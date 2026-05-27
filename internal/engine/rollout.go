package engine

import "hash/fnv"

func inRollout(seed, userID string, percentage int) bool {
	if percentage <= 0 {
		return false
	}

	if percentage >= 100 {
		return true
	}

	h := fnv.New32a()
	_, _ = h.Write([]byte(seed + ":" + userID))
	bucket := h.Sum32() % 100

	return bucket < uint32(percentage)
}
