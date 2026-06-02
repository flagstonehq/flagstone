"use client";

import { useState, useCallback } from "react";
import { useRouter } from "next/navigation";
import { Plus, Loader2, Save, AlertTriangle, CheckCircle2 } from "lucide-react";
import { Button } from "@/components/ui/button";
import { ConditionRow } from "@/components/rules/condition-row";
import { updateSegment, ApiError } from "@/lib/api";
import type { RuleConditionNode, LeafCondition } from "@/lib/types";

interface SegmentRuleEditorProps {
  projectSlug: string;
  segmentKey: string;
  initialRules: RuleConditionNode;
  segmentName: string;
}

function isLeaf(c: unknown): c is LeafCondition {
  return typeof c === "object" && c !== null && "attribute" in c && "op" in c;
}

function extractConditions(rules: RuleConditionNode): LeafCondition[] {
  if (typeof rules === "object" && !Array.isArray(rules)) {
    const r = rules as Record<string, unknown>;
    if ("all" in r && Array.isArray(r.all)) return r.all.filter(isLeaf);
    if ("any" in r && Array.isArray(r.any)) return r.any.filter(isLeaf);
    if (isLeaf(r)) return [r];
  }
  return [];
}

export function SegmentRuleEditor({ projectSlug, segmentKey, initialRules, segmentName }: SegmentRuleEditorProps) {
  const router = useRouter();
  const [conditions, setConditions] = useState<LeafCondition[]>(() => extractConditions(initialRules));
  const [isDirty, setIsDirty] = useState(false);
  const [saving, setSaving] = useState(false);
  const [saveError, setSaveError] = useState<string | null>(null);
  const [saveSuccess, setSaveSuccess] = useState(false);

  const markDirty = useCallback(() => {
    setIsDirty(true);
    setSaveError(null);
    setSaveSuccess(false);
  }, []);

  function updateCondition(i: number, c: LeafCondition) {
    const next = [...conditions];
    next[i] = c;
    setConditions(next);
    markDirty();
  }

  function addCondition() {
    setConditions([...conditions, { attribute: "", op: "eq" as const, value: "" }]);
    markDirty();
  }

  function deleteCondition(i: number) {
    setConditions(conditions.filter((_, idx) => idx !== i));
    markDirty();
  }

  function buildRulesNode(): RuleConditionNode {
    if (conditions.length === 0) return { attribute: "", op: "eq" as const, value: "" };
    if (conditions.length === 1) return conditions[0];
    return { all: conditions };
  }

  async function handleSave() {
    setSaving(true);
    setSaveError(null);
    setSaveSuccess(false);
    try {
      await updateSegment(projectSlug, segmentKey, { rules: buildRulesNode() });
      setIsDirty(false);
      setSaveSuccess(true);
      router.refresh();
    } catch (err: unknown) {
      const msg =
        err instanceof ApiError ? err.message : "Failed to save";
      setSaveError(msg);
    } finally {
      setSaving(false);
    }
  }

  return (
    <div className="space-y-4">
      <div className="flex flex-wrap items-center gap-3 rounded-lg border border-border bg-surface p-3 sm:gap-4 sm:p-4">
        <span className="text-sm text-text-secondary">
          Conditions for <span className="font-medium text-text">{segmentName}</span>
        </span>
        <div className="ml-auto flex items-center gap-2">
          {saveSuccess && (
            <span className="flex items-center gap-1 text-xs text-success">
              <CheckCircle2 className="h-3.5 w-3.5" />
              Saved
            </span>
          )}
          {saveError && (
            <span className="flex items-center gap-1 text-xs text-danger">
              <AlertTriangle className="h-3.5 w-3.5" />
              {saveError}
            </span>
          )}
          <Button type="button" size="sm" onClick={handleSave} disabled={saving || !isDirty}>
            {saving ? <Loader2 className="h-3.5 w-3.5 animate-spin" /> : <Save className="h-3.5 w-3.5" />}
            Save
          </Button>
        </div>
      </div>

      <div className="rounded-lg border border-border bg-surface p-4 space-y-3">
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
    </div>
  );
}
