package sdk

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func startedClient(t *testing.T) *Client {
	t.Helper()
	srv, _ := newTestServer(t, nil)
	c, err := New(WithEndpoint(srv.URL), WithAPIKey("k"))
	require.NoError(t, err)
	require.NoError(t, c.Start(context.Background()))
	return c
}

func TestBoolDetail_RuleMatch(t *testing.T) {
	c := startedClient(t)
	d := c.BoolDetail(context.Background(), "premium-only", false, map[string]any{"plan": "premium"})
	require.NoError(t, d.Error)
	assert.Equal(t, true, d.Value)
	assert.Equal(t, ReasonRuleMatch, d.Reason)
	assert.Equal(t, 0, d.RuleIndex)
}

func TestBoolDetail_Default(t *testing.T) {
	c := startedClient(t)
	d := c.BoolDetail(context.Background(), "new-checkout", true, nil)
	require.NoError(t, d.Error)
	assert.Equal(t, false, d.Value)
	assert.Equal(t, ReasonDefault, d.Reason)
	assert.Equal(t, -1, d.RuleIndex)
}

func TestBoolDetail_NotFound(t *testing.T) {
	c := startedClient(t)
	d := c.BoolDetail(context.Background(), "ghost", true, nil)
	require.Error(t, d.Error)
	assert.Equal(t, ReasonFlagNotFound, d.Reason)
	assert.Equal(t, true, d.Value, "the supplied default is returned on not-found")
}

func TestNumberDetail_TypeMismatch(t *testing.T) {
	c := startedClient(t)
	d := c.NumberDetail(context.Background(), "new-checkout", 9.0, nil) // a boolean flag
	require.Error(t, d.Error)
	assert.Equal(t, ReasonError, d.Reason)
	assert.Equal(t, 9.0, d.Value, "type mismatch returns the supplied default")
}

func TestHappyPath_ReturnsDefaultOnMissing(t *testing.T) {
	c := startedClient(t)
	assert.True(t, c.Bool(context.Background(), "ghost", true, nil))
	assert.False(t, c.Bool(context.Background(), "ghost", false, nil))
	assert.Equal(t, "fallback", c.String(context.Background(), "ghost", "fallback", nil))
	assert.Equal(t, 7.0, c.Number(context.Background(), "ghost", 7.0, nil))
}
