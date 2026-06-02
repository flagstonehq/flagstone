package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
)

const (
	maxRulesPerFlag      = 100
	maxRuleJSONBytes     = 64 * 1024 // 64 KB
	maxConditionDepth    = 10
	maxConditionsPerRule = 50
)

var (
	validOperators = map[string]bool{
		"eq": true, "neq": true,
		"gt": true, "gte": true, "lt": true, "lte": true,
		"contains": true, "starts_with": true, "ends_with": true,
		"in": true, "not_in": true,
		"matches":    true,
		"exists":     true,
		"not_exists": true,
		"segment":    true,
	}
	attributeNameRe = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_.]{0,63}$`)
)

// ValidateFlagRules validates the JSONB array stored in flag_environments.rules.
// It enforces the limits described in SECURITY.md: max rules, max size, max
// nesting depth, max conditions per rule, operator whitelist, and attribute
// name format.
func ValidateFlagRules(raw json.RawMessage) error {
	if len(raw) == 0 {
		return nil
	}
	if len(raw) > maxRuleJSONBytes {
		return fmt.Errorf("rules JSON exceeds maximum size of %d KB", maxRuleJSONBytes/1024)
	}

	var rules []map[string]any
	if err := json.Unmarshal(raw, &rules); err != nil {
		return errors.New("rules must be a valid JSON array")
	}
	if len(rules) > maxRulesPerFlag {
		return fmt.Errorf("rules exceeds maximum of %d entries", maxRulesPerFlag)
	}

	for i, rule := range rules {
		cond, ok := rule["conditions"]
		if !ok {
			return fmt.Errorf("rule[%d]: missing conditions", i)
		}
		condMap, ok := cond.(map[string]any)
		if !ok {
			return fmt.Errorf("rule[%d].conditions: must be an object", i)
		}
		nodeCount := 0
		if err := validateConditionNode(condMap, 0, &nodeCount); err != nil {
			return fmt.Errorf("rule[%d].conditions: %w", i, err)
		}
		if nodeCount > maxConditionsPerRule {
			return fmt.Errorf("rule[%d] has %d conditions (max %d)", i, nodeCount, maxConditionsPerRule)
		}
	}
	return nil
}

// ValidateSegmentRules validates the JSONB object stored in segments.rules.
// A segment's rules is a single ConditionNode, not an array.
func ValidateSegmentRules(raw json.RawMessage) error {
	if len(raw) == 0 {
		return nil
	}
	if len(raw) > maxRuleJSONBytes {
		return fmt.Errorf("rules JSON exceeds maximum size of %d KB", maxRuleJSONBytes/1024)
	}

	var node map[string]any
	if err := json.Unmarshal(raw, &node); err != nil {
		return errors.New("rules must be a valid JSON object")
	}

	nodeCount := 0
	return validateConditionNode(node, 0, &nodeCount)
}

// validateConditionNode recursively validates a ConditionNode. It checks:
//   - nesting depth (max maxConditionDepth)
//   - operator whitelist on leaf nodes
//   - attribute name format on leaf nodes
func validateConditionNode(node map[string]any, depth int, count *int) error {
	if depth > maxConditionDepth {
		return fmt.Errorf("nesting depth exceeds maximum of %d", maxConditionDepth)
	}
	*count++

	// Composite: "all"
	if all, ok := node["all"]; ok {
		children, err := asSlice(all, "all")
		if err != nil {
			return err
		}
		for j, child := range children {
			childMap, ok := child.(map[string]any)
			if !ok {
				return fmt.Errorf("all[%d]: must be an object", j)
			}
			if err := validateConditionNode(childMap, depth+1, count); err != nil {
				return fmt.Errorf("all[%d]: %w", j, err)
			}
		}
		return nil
	}

	// Composite: "any"
	if anyVal, ok := node["any"]; ok {
		children, err := asSlice(anyVal, "any")
		if err != nil {
			return err
		}
		for j, child := range children {
			childMap, ok := child.(map[string]any)
			if !ok {
				return fmt.Errorf("any[%d]: must be an object", j)
			}
			if err := validateConditionNode(childMap, depth+1, count); err != nil {
				return fmt.Errorf("any[%d]: %w", j, err)
			}
		}
		return nil
	}

	// Composite: "not"
	if not, ok := node["not"]; ok {
		childMap, ok := not.(map[string]any)
		if !ok {
			return errors.New("not: must be an object")
		}
		return validateConditionNode(childMap, depth+1, count)
	}

	// Leaf node — must have attribute and op.
	attr, hasAttr := node["attribute"]
	op, hasOp := node["op"]

	if !hasAttr || !hasOp {
		return errors.New("leaf condition must have 'attribute' and 'op' fields")
	}

	attrStr, ok := attr.(string)
	if !ok || attrStr == "" {
		return errors.New("'attribute' must be a non-empty string")
	}
	if !attributeNameRe.MatchString(attrStr) {
		return fmt.Errorf("attribute %q does not match pattern ^[a-zA-Z_][a-zA-Z0-9_.]{0,63}$", attrStr)
	}

	opStr, ok := op.(string)
	if !ok || opStr == "" {
		return errors.New("'op' must be a non-empty string")
	}
	if !validOperators[opStr] {
		return fmt.Errorf("operator %q is not allowed", opStr)
	}

	return nil
}

func asSlice(v any, field string) ([]any, error) {
	s, ok := v.([]any)
	if !ok {
		return nil, fmt.Errorf("%q must be an array", field)
	}
	return s, nil
}
