package sdk

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestContext_WithDoesNotMutate(t *testing.T) {
	base := NewContext("u-1", map[string]any{"plan": "free"})
	derived := base.With("plan", "premium").With("country", "AR")

	assert.Equal(t, "free", base.Attributes["plan"], "original must be unchanged")
	assert.NotContains(t, base.Attributes, "country")
	assert.Equal(t, "premium", derived.Attributes["plan"])
	assert.Equal(t, "AR", derived.Attributes["country"])
}

func TestContext_WithPrivateDoesNotMutate(t *testing.T) {
	base := NewContext("u-1", nil)
	derived := base.WithPrivate("email", "ip")
	assert.Empty(t, base.PrivateAttributes)
	assert.Equal(t, []string{"email", "ip"}, derived.PrivateAttributes)
}

func TestContext_AsMapBridgesTargetingKey(t *testing.T) {
	m := NewContext("u-1", map[string]any{"plan": "premium"}).asMap()
	assert.Equal(t, "u-1", m["targeting_key"])
	assert.Equal(t, "u-1", m["user_id"], "mirrored to user_id for the current rollout")
	assert.Equal(t, "premium", m["plan"])

	m2 := NewContext("u-1", map[string]any{"user_id": "real"}).asMap()
	assert.Equal(t, "real", m2["user_id"])
	assert.Equal(t, "u-1", m2["targeting_key"])
}

func TestScoped_BindsContext(t *testing.T) {
	c := startedClient(t)
	premium := c.Scoped(NewContext("u-1", map[string]any{"plan": "premium"}))
	free := c.Scoped(NewContext("u-2", map[string]any{"plan": "free"}))

	assert.True(t, premium.Bool("premium-only", false), "rule matches plan=premium")
	assert.False(t, free.Bool("premium-only", false), "rule does not match plan=free")
}

func TestScoped_Detail(t *testing.T) {
	c := startedClient(t)
	d := c.Scoped(NewContext("u-1", map[string]any{"plan": "premium"})).BoolDetail("premium-only", false)
	assert.Equal(t, ReasonRuleMatch, d.Reason)
	assert.Equal(t, true, d.Value)
}

func TestScoped_Concurrent(t *testing.T) {
	c := startedClient(t)
	sc := c.Scoped(NewContext("u-1", map[string]any{"plan": "premium"}))
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if !sc.Bool("premium-only", false) {
				t.Error("expected true under concurrent access")
			}
		}()
	}
	wg.Wait()
}
