package sdk

import "context"

// Context carries the subject of a flag evaluation: who the user (or
// service) is, their attributes, and which attributes are private.
//
// It is a small value type — copy it freely. Use NewContext plus the
// With* builders to construct one; all builders return a new Context and
// never mutate the receiver, so a Context is safe to share.
type Context struct {
	// TargetingKey uniquely identifies the subject. It is the key used
	// for sticky/consistent rollout bucketing.
	TargetingKey string

	// Attributes are arbitrary key-value pairs used in rule conditions
	// (e.g. "plan", "country").
	Attributes map[string]any

	// PrivateAttributes lists attribute keys that must never be sent to
	// any external system (analytics, exposure events, logs). Redaction
	// is wired in Phase 4 (exposure events); until then this is recorded
	// but has no observable effect.
	PrivateAttributes []string
}

// NewContext builds a Context from a targeting key and attribute map.
func NewContext(targetingKey string, attrs map[string]any) Context {
	return Context{TargetingKey: targetingKey, Attributes: attrs}
}

// With returns a copy of the Context with one attribute added or
// overwritten. The original is not modified.
func (c Context) With(attr string, value any) Context {
	next := make(map[string]any, len(c.Attributes)+1)
	for k, v := range c.Attributes {
		next[k] = v
	}
	next[attr] = value
	c.Attributes = next
	return c
}

// WithPrivate returns a copy of the Context with the given attribute
// keys marked private. The original is not modified.
func (c Context) WithPrivate(attrs ...string) Context {
	next := make([]string, 0, len(c.PrivateAttributes)+len(attrs))
	next = append(next, c.PrivateAttributes...)
	next = append(next, attrs...)
	c.PrivateAttributes = next
	return c
}

// asMap flattens the Context into the map[string]any the engine expects.
// TargetingKey is exposed as "targeting_key"; for compatibility with the
// engine's current rollout (which buckets on "user_id") it is also
// mirrored to "user_id" when the caller has not set one explicitly.
func (c Context) asMap() map[string]any {
	m := make(map[string]any, len(c.Attributes)+2)
	for k, v := range c.Attributes {
		m[k] = v
	}
	if c.TargetingKey != "" {
		m["targeting_key"] = c.TargetingKey
		if _, ok := m["user_id"]; !ok {
			m["user_id"] = c.TargetingKey
		}
	}
	return m
}

// ScopedClient is a thin wrapper that binds a Context to all evaluations,
// so callers don't repeat the attributes on every flag check. Cheap to
// create and safe to share across goroutines.
type ScopedClient struct {
	client *Client
	ctx    Context
}

// Scoped returns a ScopedClient bound to ctx. Define the Context once
// (e.g. at request start) and reuse it for every flag check in scope.
func (c *Client) Scoped(ctx Context) *ScopedClient {
	return &ScopedClient{client: c, ctx: ctx}
}

// Bool evaluates a boolean flag using the bound Context, returning def on
// any error. Like Client.Bool it never errors and never blocks.
func (s *ScopedClient) Bool(key string, def bool) bool {
	return s.client.Bool(context.Background(), key, def, s.ctx.asMap())
}

// String evaluates a string flag using the bound Context.
func (s *ScopedClient) String(key, def string) string {
	return s.client.String(context.Background(), key, def, s.ctx.asMap())
}

// Number evaluates a numeric flag using the bound Context.
func (s *ScopedClient) Number(key string, def float64) float64 {
	return s.client.Number(context.Background(), key, def, s.ctx.asMap())
}

// JSON evaluates a flag of any JSON type using the bound Context.
func (s *ScopedClient) JSON(key string, def any) any {
	return s.client.JSON(context.Background(), key, def, s.ctx.asMap())
}

// BoolDetail returns the full evaluation detail using the bound Context.
func (s *ScopedClient) BoolDetail(key string, def bool) EvaluationDetail {
	return s.client.BoolDetail(context.Background(), key, def, s.ctx.asMap())
}

// All evaluates every flag in the snapshot using the bound Context.
func (s *ScopedClient) All() (map[string]Value, error) {
	return s.client.All(context.Background(), s.ctx.asMap())
}
