package sdk

import (
	"context"
	"fmt"

	"github.com/flagstonehq/flagstone/pkg/engine"
)

// Reason explains why an evaluation produced its value. It is a type
// alias for engine.Reason so callers can compare against the re-exported
// constants below without importing pkg/engine.
type Reason = engine.Reason

// Re-exported evaluation reasons, so callers compare against sdk.Reason*
// without importing pkg/engine.
const (
	ReasonRuleMatch    Reason = engine.ReasonRuleMatch
	ReasonDefault      Reason = engine.ReasonDefault
	ReasonDisabled     Reason = engine.ReasonDisabled
	ReasonFlagNotFound Reason = engine.ReasonFlagNotFound
	ReasonFlagArchived Reason = engine.ReasonFlagArchived
	// ReasonError is SDK-level: the snapshot was not loaded yet, or the
	// flag's type did not match the requested type. Distinct from the
	// engine's internal-panic reason.
	ReasonError Reason = "ERROR"
)

// EvaluationDetail is the full result of one flag evaluation: the
// resolved value plus why it was chosen. The happy-path methods
// (Bool/String/Number/JSON) return just the value; the *Detail methods
// return this struct when the caller wants the reason, the matched rule,
// or the error.
type EvaluationDetail struct {
	// Value is the resolved value, or the supplied default on any error.
	Value any
	// Reason explains the outcome.
	Reason Reason
	// RuleIndex is the index of the matching rule, or -1 if none matched.
	RuleIndex int
	// Variant is the assigned variant key. Always empty until
	// multi-variant flags land (Phase 4); reserved for forward-compat.
	Variant string
	// Error is non-nil when the value could not be resolved (flag not
	// found, type mismatch, or snapshot not loaded). In those cases
	// Value holds the supplied default.
	Error error
}

// evalDetail is the shared, lock-free core. It loads the current
// snapshot atomically and runs the engine in memory; it never blocks on
// the network. Type checking and default substitution happen in the
// typed *Detail wrappers.
func (c *Client) evalDetail(key string, evalCtx map[string]any) EvaluationDetail {
	snap := c.cache.get()
	// A zero fetchedAt means no snapshot has ever been loaded (the cache
	// is seeded with an empty one). Point the caller at Start().
	if snap.fetchedAt.IsZero() {
		c.signalRefresh()
		return EvaluationDetail{
			Reason:    ReasonError,
			RuleIndex: -1,
			Error:     fmt.Errorf("sdk: snapshot not loaded, call Start() first"),
		}
	}
	fc, ok := snap.flags[key]
	if !ok {
		return EvaluationDetail{
			Reason:    ReasonFlagNotFound,
			RuleIndex: -1,
			Error:     fmt.Errorf("sdk: flag %q not found", key),
		}
	}
	res := c.engine.Evaluate(engine.EvaluateRequest{
		FlagConfig: fc,
		Segments:   snap.segments,
		Context:    evalCtx,
	})
	return EvaluationDetail{
		Value:     res.Value,
		Reason:    res.Reason,
		RuleIndex: res.RuleIndex,
	}
}

// BoolDetail evaluates a boolean flag and returns the full detail. On any
// error (not found, type mismatch, not loaded) Value is def and Error is
// set; the call never returns a Go error and never blocks on the network.
func (c *Client) BoolDetail(_ context.Context, key string, def bool, evalCtx map[string]any) EvaluationDetail {
	d := c.evalDetail(key, evalCtx)
	if d.Error != nil {
		d.Value = def
		return d
	}
	b, ok := d.Value.(bool)
	if !ok {
		return EvaluationDetail{
			Value:     def,
			Reason:    ReasonError,
			RuleIndex: -1,
			Error:     fmt.Errorf("sdk: flag %q is not boolean (got %T)", key, d.Value),
		}
	}
	d.Value = b
	return d
}

// StringDetail evaluates a string flag and returns the full detail.
func (c *Client) StringDetail(_ context.Context, key, def string, evalCtx map[string]any) EvaluationDetail {
	d := c.evalDetail(key, evalCtx)
	if d.Error != nil {
		d.Value = def
		return d
	}
	s, ok := d.Value.(string)
	if !ok {
		return EvaluationDetail{
			Value:     def,
			Reason:    ReasonError,
			RuleIndex: -1,
			Error:     fmt.Errorf("sdk: flag %q is not string (got %T)", key, d.Value),
		}
	}
	d.Value = s
	return d
}

// NumberDetail evaluates a numeric flag and returns the full detail.
func (c *Client) NumberDetail(_ context.Context, key string, def float64, evalCtx map[string]any) EvaluationDetail {
	d := c.evalDetail(key, evalCtx)
	if d.Error != nil {
		d.Value = def
		return d
	}
	n, ok := d.Value.(float64)
	if !ok {
		return EvaluationDetail{
			Value:     def,
			Reason:    ReasonError,
			RuleIndex: -1,
			Error:     fmt.Errorf("sdk: flag %q is not number (got %T)", key, d.Value),
		}
	}
	d.Value = n
	return d
}

// JSONDetail evaluates a flag of any JSON type and returns the full
// detail. There is no type check; def is used only on not-found /
// not-loaded errors.
func (c *Client) JSONDetail(_ context.Context, key string, def any, evalCtx map[string]any) EvaluationDetail {
	d := c.evalDetail(key, evalCtx)
	if d.Error != nil {
		d.Value = def
	}
	return d
}
