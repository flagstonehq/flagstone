package engine

import (
	"runtime/debug"

	"go.uber.org/zap"
)

// Engine evaluates feature flag rules against a user context. It is
// intentionally free of I/O — callers preload flags and segments from storage
// and pass them in. This keeps the engine unit-testable with no mocks.
type Engine struct {
	logger *zap.Logger
}

// New creates an Engine backed by the given logger. One instance is
// typically shared for the lifetime of the process.
func New(logger *zap.Logger) *Engine {
	return &Engine{logger: logger}
}

// Evaluate evaluates a single flag against the provided context and segments.
// It always returns a result — it never errors. If the evaluation panics
// internally, it recovers and returns ReasonInternalErr with value false.
func (e *Engine) Evaluate(req EvaluateRequest) (result EvaluateResult) {
	defer func() {
		if r := recover(); r != nil {
			e.logger.Error("engine panic",
				zap.String("flag_key", req.FlagConfig.Key),
				zap.Any("panic", r),
				zap.ByteString("stack", debug.Stack()),
			)
			result = EvaluateResult{
				Value:     false,
				Reason:    ReasonInternalErr,
				RuleIndex: -1,
			}
		}
	}()

	fc := req.FlagConfig
	if !fc.Enabled {
		return EvaluateResult{
			Value:     false,
			Reason:    ReasonDisabled,
			RuleIndex: -1,
		}
	}

	userID, _ := req.Context["user_id"].(string)
	for i, rule := range fc.Rules {
		if !e.evaluateNode(rule.Conditions, req.Context, req.Segments, map[string]struct{}{}) {
			continue
		}
		if rule.Rollout == nil {
			return EvaluateResult{
				Value:     ruleValueOrDefault(rule),
				Reason:    ReasonRuleMatch,
				RuleIndex: i,
			}
		}
		if userID == "" {
			e.logger.Debug("rollout rule skipped: no user_id",
				zap.String("flag_key", fc.Key),
				zap.Int("rule_index", i),
			)
			continue
		}
		seed := rule.Rollout.Seed
		if seed == "" {
			seed = fc.Key
		}
		if !inRollout(seed, userID, rule.Rollout.Percentage) {
			continue
		}
		return EvaluateResult{
			Value:     ruleValueOrDefault(rule),
			Reason:    ReasonRuleMatch,
			RuleIndex: i,
		}
	}

	if fc.HasEnvironmentDefault {
		return EvaluateResult{
			Value:     fc.EnvironmentDefaultValue,
			Reason:    ReasonDefault,
			RuleIndex: -1,
		}
	}
	return EvaluateResult{
		Value:     fc.DefaultValue,
		Reason:    ReasonDefault,
		RuleIndex: -1,
	}
}

// EvaluateAll evaluates every flag in one pass and returns a map keyed by
// flag key. Used by the bulk evaluate endpoint (SDK bootstrap). Each flag is
// evaluated independently; a panic in one flag does not affect the others.
func (e *Engine) EvaluateAll(flags []FlagConfig, segments map[string]Segment, ctx map[string]any) map[string]EvaluateResult {
	results := make(map[string]EvaluateResult, len(flags))
	for _, fc := range flags {
		results[fc.Key] = e.Evaluate(EvaluateRequest{
			FlagConfig: fc,
			Segments:   segments,
			Context:    ctx,
		})
	}
	return results
}

func ruleValueOrDefault(rule Rule) any {
	if rule.Value == nil {
		return true
	}
	return rule.Value
}
