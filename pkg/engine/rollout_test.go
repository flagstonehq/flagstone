package engine

import (
	"fmt"
	"testing"
)

func TestInRollout_Deterministic(t *testing.T) {
	for range 1000 {
		a := inRollout("checkout", "user-123", 25)
		b := inRollout("checkout", "user-123", 25)
		if a != b {
			t.Fatal("rollout must be deterministic")
		}
	}
}

func TestInRollout_Monotonic(t *testing.T) {
	for i := range 5000 {
		userID := fmt.Sprintf("user-%d", i)
		in10 := inRollout("checkout", userID, 10)
		in25 := inRollout("checkout", userID, 25)
		if in10 && !in25 {
			t.Fatalf("user %s in 10%% must also be in 25%%", userID)
		}
	}
}

func TestInRollout_Edges(t *testing.T) {
	if inRollout("f", "u", 0) {
		t.Fatal("percentage 0 must be false")
	}
	if !inRollout("f", "u", 100) {
		t.Fatal("percentage 100 must be true")
	}
}

func TestInRollout_SeedAffectsBucket(t *testing.T) {
	// With two different seeds at 50%, a meaningful fraction of users must
	// land in one bucket but not the other — that's how we know the seed is
	// actually feeding into the hash. We don't pin a specific user (any single
	// user can coincidentally match), we check the distribution across many.
	differing := 0
	for i := range 1000 {
		userID := fmt.Sprintf("user-%d", i)
		if inRollout("seed-a", userID, 50) != inRollout("seed-b", userID, 50) {
			differing++
		}
	}
	if differing < 250 {
		t.Fatalf("expected >=250 users to flip between seed-a and seed-b at 50%%, got %d — seed isn't reaching the hash", differing)
	}
}

func TestInRollout_EmptyUserIDIsDeterministic(t *testing.T) {
	// The engine layer skips rollout rules when user_id is missing, so this
	// function isn't called with an empty userID in production. We only
	// guarantee that if it IS called, the result is stable — not that it
	// lands on any specific side of the bucket.
	first := inRollout("flag", "", 50)
	for range 100 {
		if inRollout("flag", "", 50) != first {
			t.Fatal("empty userID must still produce a deterministic bucket")
		}
	}
}
