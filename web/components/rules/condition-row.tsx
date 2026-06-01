"use client";

import { X } from "lucide-react";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  Select, SelectTrigger, SelectValue, SelectContent, SelectItem,
} from "@/components/ui/select";
import type { LeafCondition } from "@/lib/types";

interface ConditionRowProps {
  condition: LeafCondition;
  onChange: (condition: LeafCondition) => void;
  onDelete: () => void;
}

const OPERATORS = [
  "eq", "neq", "gt", "gte", "lt", "lte",
  "in", "not_in", "contains", "starts_with", "ends_with", "matches",
  "exists", "not_exists", "segment",
] as const;

export function ConditionRow({ condition, onChange, onDelete }: ConditionRowProps) {
  return (
    <div className="flex items-end gap-2">
      <div className="flex-1 space-y-1">
        <Label className="text-xs text-text-secondary">Attribute</Label>
        <Input
          value={condition.attribute}
          onChange={(e) => onChange({ ...condition, attribute: e.target.value })}
          placeholder="e.g. country"
          aria-label="Attribute"
        />
      </div>
      <div className="w-28 space-y-1">
        <Label className="text-xs text-text-secondary">Op</Label>
        <Select
          value={condition.operator}
          onValueChange={(v: string | null) =>
            onChange({ ...condition, operator: (v ?? "eq") as LeafCondition["operator"] })
          }
        >
          <SelectTrigger aria-label="Operator">
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            {OPERATORS.map((op) => (
              <SelectItem key={op} value={op}>
                {op}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      </div>
      <div className="flex-1 space-y-1">
        <Label className="text-xs text-text-secondary">Value</Label>
        <Input
          value={typeof condition.value === "string" ? condition.value : JSON.stringify(condition.value ?? "")}
          onChange={(e) => onChange({ ...condition, value: e.target.value })}
          placeholder="e.g. AR"
          aria-label="Value"
        />
      </div>
      <button
        type="button"
        onClick={onDelete}
        className="mb-0.5 flex h-8 w-8 items-center justify-center rounded-md text-text-secondary hover:bg-hover hover:text-text"
        aria-label="Remove condition"
      >
        <X className="h-4 w-4" />
      </button>
    </div>
  );
}
