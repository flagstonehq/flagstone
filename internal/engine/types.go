package engine

// Reason describes why an evaluation produced its result.
type Reason string

const (
	// ReasonRuleMatch means a targeting rule matched (and rollout passed, if applicable).
	ReasonRuleMatch Reason = "RULE_MATCH"
	// ReasonDefault means no rule matched; the flag's default value was returned.
	ReasonDefault Reason = "DEFAULT"
	// ReasonDisabled means the flag's master kill-switch is off.
	ReasonDisabled Reason = "DISABLED"
	// ReasonFlagNotFound means the flag key does not exist in this environment.
	ReasonFlagNotFound Reason = "FLAG_NOT_FOUND"
	// ReasonFlagArchived means the flag has been soft-deleted.
	ReasonFlagArchived Reason = "FLAG_ARCHIVED"
	// ReasonInternalErr means the engine panicked and recovered.
	ReasonInternalErr Reason = "INTERNAL_ERROR"
)

// EvaluateResult holds the outcome of a single flag evaluation.
type EvaluateResult struct {
	Value     any
	Reason    Reason
	RuleIndex int // -1 when no rule matched
}

// EvaluateRequest bundles everything the engine needs to evaluate one flag.
// Segments must be preloaded by the caller — the engine does no I/O.
type EvaluateRequest struct {
	FlagConfig FlagConfig
	Segments   map[string]Segment
	Context    map[string]any
}

// FlagConfig holds all data needed to evaluate a single feature flag in a
// given environment. It is populated from storage by the evaluate handler.
type FlagConfig struct {
	Key                     string
	Enabled                 bool
	FlagType                string
	DefaultValue            any
	EnvironmentDefaultValue any
	HasEnvironmentDefault   bool
	Version                 int64
	Rules                   []Rule
}

// Rule is a single targeting rule. Rules are evaluated in order; the first
// rule whose conditions match (and whose rollout bucket applies) wins.
type Rule struct {
	Conditions ConditionNode
	Rollout    *RolloutConfig // nil = 100% of matched users
	Value      any            // nil defaults to true for boolean flags
}

// RolloutConfig controls gradual rollout via consistent hashing on user_id.
type RolloutConfig struct {
	Percentage int    // 0–100
	Seed       string // defaults to the flag key when empty
}

// Segment is a reusable named condition set that flag rules can reference
// via the "segment" operator. The engine evaluates segment conditions
// recursively and detects cycles via a visited set.
type Segment struct {
	Key        string
	Conditions ConditionNode
}

// ConditionNode is a node in a boolean expression tree (all/any/not/leaf).
// Exactly one of All, Any, Not, or the leaf fields (Attribute + Op + Value)
// should be set; the engine short-circuits on the first matching branch.
type ConditionNode struct {
	// Leaf fields — used when the node is a single condition.
	Attribute *string
	Op        *string
	Value     any

	// Composite fields — used when the node groups other conditions.
	All []ConditionNode // AND — true when all children are true
	Any []ConditionNode // OR  — true when any child is true
	Not *ConditionNode  // NOT — inverts the single child
}
