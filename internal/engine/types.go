package engine

// Reason describes why an evaluation produced its result.
type Reason string

const (
	// ReasonRuleMatch means a rule's conditions matched (and rollout passed if applicable).
	ReasonRuleMatch Reason = "RULE_MATCH"
	// ReasonDefault means no rule matched; the default value was returned.
	ReasonDefault Reason = "DEFAULT"
	// ReasonDisabled means the flag's master kill switch is off.
	ReasonDisabled Reason = "DISABLED"
	// ReasonFlagNotFound means the flag key does not exist in this project.
	ReasonFlagNotFound Reason = "FLAG_NOT_FOUND"
	// ReasonFlagArchived means the flag was soft-deleted.
	ReasonFlagArchived Reason = "FLAG_ARCHIVED"
	// ReasonInternalErr means the engine panicked and recovered.
	ReasonInternalErr Reason = "INTERNAL_ERROR"
)

// EvaluateResult holds the outcome of a single flag evaluation.
type EvaluateResult struct {
	Value     any
	Reason    Reason
	RuleIndex int
}

// FlagConfig holds all data needed to evaluate a single feature flag.
type FlagConfig struct {
	Key                     string
	Enabled                 bool
	FlagType                string
	DefaultValue            any
	EnvironmentDefaultValue any
	HasEnvironmentDefault   bool
	Rules                   []Rule
}

// Rule is a single targeting rule evaluated in order (first match wins).
type Rule struct {
	Conditions ConditionNode
	Rollout    *RolloutConfig
	Value      any
}

// RolloutConfig controls gradual rollout via consistent hashing.
type RolloutConfig struct {
	Percentage int
	Seed       string
}

// Segment is a reusable named condition evaluated by reference.
type Segment struct {
	Key        string
	Conditions ConditionNode
}

// ConditionNode is a boolean expression tree (all/any/not/leaf).
type ConditionNode struct {
	Attribute *string
	Op        *string
	Value     any

	All []ConditionNode
	Any []ConditionNode
	Not *ConditionNode
}
