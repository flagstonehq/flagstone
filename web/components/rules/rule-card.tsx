"use client";

import { X, Plus } from "lucide-react";
import { Button } from "@/components/ui/button";
import { ConditionRow } from "./condition-row";
import { RolloutInput } from "./rollout-input";
import {
  Select, SelectTrigger, SelectValue, SelectContent, SelectItem,
} from "@/components/ui/select";
import type { Rule, LeafCondition } from "@/lib/types";

interface RuleCardProps {
  rule: Rule;
  index: number;
  onChange: (rule: Rule) => void;
  onDelete: () => void;
  errors?: Record<string, string>;
}

function isLeaf(c: unknown): c is LeafCondition {
  return typeof c === "object" && c !== null && "attribute" in c && "op" in c;
}

function extractConditions(rule: Rule): LeafCondition[] {
  if (rule.conditions && typeof rule.conditions === "object" && !Array.isArray(rule.conditions)) {
    const conds = rule.conditions as Record<string, unknown>;
    if ("all" in conds && Array.isArray(conds.all)) {
      return conds.all.filter(isLeaf);
    }
    if ("any" in conds && Array.isArray(conds.any)) {
      return conds.any.filter(isLeaf);
    }
    if (isLeaf(conds)) return [conds];
  }
  return [];
}

export function RuleCard({ rule, index, onChange, onDelete, errors }: RuleCardProps) {
  const conditions = extractConditions(rule);

  function setConditions(leaves: LeafCondition[]) {
    if (leaves.length === 0) {
      onChange({
        ...rule,
        conditions: { attribute: "", op: "eq" as const, value: "" },
      });
    } else if (leaves.length === 1) {
      onChange({ ...rule, conditions: leaves[0] });
    } else {
      onChange({ ...rule, conditions: { all: leaves } });
    }
  }

  function updateCondition(i: number, c: LeafCondition) {
    const next = [...conditions];
    next[i] = c;
    setConditions(next);
  }

  function addCondition() {
    setConditions([...conditions, { attribute: "", op: "eq" as const, value: "" }]);
  }

  function deleteCondition(i: number) {
    setConditions(conditions.filter((_, idx) => idx !== i));
  }

  function setReturnValue(v: string | null) {
    const parsed: unknown = v === "true" ? true : v === "false" ? false : v === "" ? null : v;
    onChange({ ...rule, value: parsed });
  }

  function setRollout(pct: number | undefined) {
    if (pct === undefined || pct >= 100) {
      const next = { ...rule };
      delete next.rollout;
      onChange(next);
    } else {
      onChange({ ...rule, rollout: { percentage: pct } });
    }
  }

  const returnStr = rule.value === true ? "true" : rule.value === false ? "false" : rule.value === null ? "" : String(rule.value ?? "");

  return (
    <div className="rounded-lg border border-border bg-surface p-4 space-y-3">
      <div className="flex items-center justify-between">
        <span className="text-sm font-medium">Rule {index + 1}</span>
        <button
          type="button"
          onClick={onDelete}
          className="flex h-6 w-6 items-center justify-center rounded text-text-secondary hover:bg-hover hover:text-text"
          aria-label={`Remove rule ${index + 1}`}
        >
          <X className="h-4 w-4" />
        </button>
      </div>

      <div className="space-y-2">
        {conditions.length > 1 && (
          <span className="text-xs font-medium text-text-secondary">ALL of:</span>
        )}
        {conditions.map((c, i) => (
          <ConditionRow
            key={i}
            condition={c}
            onChange={(nc) => updateCondition(i, nc)}
            onDelete={() => deleteCondition(i)}
          />
        ))}
        <Button type="button" variant="ghost" size="sm" onClick={addCondition}>
          <Plus className="h-3.5 w-3.5" />
          Add condition
        </Button>
      </div>

      <div className="flex flex-wrap items-center gap-3 sm:gap-4">
        <div className="space-y-1">
          <span className="text-xs text-text-secondary">Return</span>
          <Select value={returnStr} onValueChange={setReturnValue}>
            <SelectTrigger className="w-24" aria-label="Return value">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="">Default</SelectItem>
              <SelectItem value="true">true</SelectItem>
              <SelectItem value="false">false</SelectItem>
            </SelectContent>
          </Select>
        </div>
        <RolloutInput
          value={rule.rollout?.percentage}
          onChange={setRollout}
          error={errors?.[`rules.${index}.rollout`]}
        />
      </div>
    </div>
  );
}
