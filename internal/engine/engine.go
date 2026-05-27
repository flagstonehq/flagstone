package engine

// Evaluate evaluates a flag configuration against the provided context and
// returns the result (value, reason, matched rule index).
func Evaluate(flag FlagConfig, context map[string]any, segments map[string]Segment) EvaluateResult {
	if !flag.Enabled {
		return EvaluateResult{
			Value:     false,
			Reason:    ReasonDisabled,
			RuleIndex: -1,
		}
	}
	userID, _ := context["user_id"].(string)
	for i, rule := range flag.Rules {
		if !evaluateNode(rule.Conditions, context, segments, map[string]struct{}{}) {
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
			continue
		}
		seed := rule.Rollout.Seed
		if seed == "" {
			seed = flag.Key
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
	if flag.HasEnvironmentDefault {
		return EvaluateResult{
			Value:     flag.EnvironmentDefaultValue,
			Reason:    ReasonDefault,
			RuleIndex: -1,
		}
	}
	return EvaluateResult{
		Value:     flag.DefaultValue,
		Reason:    ReasonDefault,
		RuleIndex: -1,
	}
}

func ruleValueOrDefault(rule Rule) any {
	if rule.Value == nil {
		return true
	}
	return rule.Value
}
