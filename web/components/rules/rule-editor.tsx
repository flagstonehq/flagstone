"use client";

import { useState, useCallback } from "react";
import { useRouter } from "next/navigation";
import { Plus, Loader2, Save, AlertTriangle, CheckCircle2, HelpCircle, X } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Switch } from "@/components/ui/switch";
import {
  Select, SelectTrigger, SelectValue, SelectContent, SelectItem,
} from "@/components/ui/select";
import { RuleCard } from "./rule-card";
import { saveFlagEnvironment, ApiError } from "@/lib/api";
import type { Rule, Environment } from "@/lib/types";

function RulesHelp({ open, onClose }: { open: boolean; onClose: () => void }) {
  if (!open) return null;
  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/40" onClick={onClose}>
      <div
        className="relative w-full max-w-lg rounded-xl border border-border bg-popover p-6 shadow-xl"
        onClick={(e) => e.stopPropagation()}
      >
        <button
          type="button"
          onClick={onClose}
          className="absolute right-3 top-3 flex h-7 w-7 items-center justify-center rounded-md text-text-secondary hover:bg-hover hover:text-text"
          aria-label="Close help"
        >
          <X className="h-4 w-4" />
        </button>

        <h2 className="text-base font-semibold mb-1">How rules work</h2>
        <p className="text-sm text-text-secondary mb-4">
          Rules are evaluated top-to-bottom. The first rule whose conditions match wins.
          If no rule matches, the flag returns its default value.
        </p>

        <h3 className="text-xs font-semibold uppercase tracking-wide text-text-secondary mb-2">Operators</h3>
        <div className="space-y-1 text-sm mb-4">
          {[
            ["eq / neq", "Exact match", "plan eq premium"],
            ["gt / gte / lt / lte", "Numeric comparison", "age gte 18"],
            ["contains", "String substring", "email contains @acme"],
            ["starts_with / ends_with", "String prefix or suffix", "domain ends_with .com"],
            ["matches", "Regex (RE2 syntax)", "userId matches ^usr_[0-9]+"],
            ["in / not_in", "Value in a list", "country in AR,BR,CL"],
            ["exists / not_exists", "Attribute is present or missing", "betaOptIn exists"],
            ["segment", "User matches a saved segment", "plan segment premium-users"],
          ].map(([op, desc, example]) => (
            <div key={op} className="grid grid-cols-[140px_1fr] gap-2 rounded-md px-2 py-1.5 hover:bg-hover">
              <code className="text-xs font-mono text-primary">{op}</code>
              <span className="text-text-secondary text-xs">
                {desc}
                <span className="ml-2 text-text-secondary/60 italic">{example}</span>
              </span>
            </div>
          ))}
        </div>

        <h3 className="text-xs font-semibold uppercase tracking-wide text-text-secondary mb-2">Example rule</h3>
        <pre className="rounded-lg bg-hover p-3 text-xs text-text leading-relaxed overflow-x-auto">{`If  plan  eq  "premium"
And country  in  "AR,BR,CL"
→ Return true  (Rollout 25%)`}</pre>
        <p className="mt-3 text-xs text-text-secondary">
          Add multiple conditions to the same rule to AND them together.
          Add multiple rules to create OR-style logic (first match wins).
        </p>
      </div>
    </div>
  );
}

interface RuleEditorProps {
  projectSlug: string;
  flagKey: string;
  flagType: string;
  initialRules: Rule[];
  initialEnabled: boolean;
  initialVersion: number;
  environments: Environment[];
  currentEnvSlug: string;
}

function defaultRule(): Rule {
  return { conditions: { attribute: "", op: "eq" as const, value: "" }, value: null };
}

export function RuleEditor({
  projectSlug, flagKey, flagType: _flagType,
  initialRules, initialEnabled, initialVersion,
  environments, currentEnvSlug,
}: RuleEditorProps) {
  const router = useRouter();
  const [rules, setRules] = useState<Rule[]>(initialRules.length > 0 ? initialRules : [defaultRule()]);
  const [enabled, setEnabled] = useState(initialEnabled);
  const [version, setVersion] = useState(initialVersion);
  const [isDirty, setIsDirty] = useState(false);
  const [saving, setSaving] = useState(false);
  const [saveError, setSaveError] = useState<string | null>(null);
  const [saveSuccess, setSaveSuccess] = useState(false);
  const [pendingEnv, setPendingEnv] = useState<string | null>(null);
  const [validationErrors, setValidationErrors] = useState<Record<string, string>>({});
  const [showHelp, setShowHelp] = useState(false);

  const markDirty = useCallback(() => {
    setIsDirty(true);
    setSaveError(null);
    setSaveSuccess(false);
  }, []);

  function handleRulesChange(index: number, rule: Rule) {
    const next = [...rules];
    next[index] = rule;
    setRules(next);
    markDirty();
  }

  function deleteRule(index: number) {
    const next = rules.filter((_, i) => i !== index);
    if (next.length === 0) next.push(defaultRule());
    setRules(next);
    markDirty();
  }

  function addRule() {
    setRules([...rules, defaultRule()]);
    markDirty();
  }

  function handleEnabledChange(next: boolean) {
    setEnabled(next);
    markDirty();
  }

  function handleEnvSelect(envSlug: string | null) {
    if (!envSlug || envSlug === currentEnvSlug) return;
    if (isDirty) {
      setPendingEnv(envSlug);
    } else {
      router.push(`/projects/${projectSlug}/flags/${flagKey}?env=${envSlug}`);
    }
  }

  function confirmEnvChange() {
    if (pendingEnv) {
      router.push(`/projects/${projectSlug}/flags/${flagKey}?env=${pendingEnv}`);
    }
  }

  function cancelEnvChange() {
    setPendingEnv(null);
  }

  function validate(): boolean {
    const errs: Record<string, string> = {};
    rules.forEach((r, i) => {
      if (r.rollout && (r.rollout.percentage < 0 || r.rollout.percentage > 100)) {
        errs[`rules.${i}.rollout`] = "Must be 0-100";
      }
    });
    setValidationErrors(errs);
    return Object.keys(errs).length === 0;
  }

  async function handleSave() {
    if (!validate()) return;
    setSaving(true);
    setSaveError(null);
    setSaveSuccess(false);
    try {
      const resp = await saveFlagEnvironment(projectSlug, flagKey, currentEnvSlug, {
        enabled,
        rules,
        version,
      });
      setVersion(resp.version);
      setIsDirty(false);
      setSaveSuccess(true);
    } catch (err: unknown) {
      const msg =
        err instanceof ApiError && (err.code === "VERSION_CONFLICT" || err.status === 409)
          ? ""
          : err instanceof Error
            ? err.message
            : "Failed to save";
      if (err instanceof ApiError && (err.code === "VERSION_CONFLICT" || err.status === 409)) {
        setSaveError("This flag was modified by someone else. Reload to see the latest version.");
      } else {
        setSaveError(msg);
      }
    } finally {
      setSaving(false);
    }
  }

  return (
    <div className="space-y-4">
      <RulesHelp open={showHelp} onClose={() => setShowHelp(false)} />

      {/* Controls bar */}
      <div className="flex flex-wrap items-center gap-3 rounded-lg border border-border bg-surface p-3 sm:gap-4 sm:p-4">
        <div className="flex items-center gap-2">
          <span className="text-sm text-text-secondary">Enabled</span>
          <Switch checked={enabled} onCheckedChange={handleEnabledChange} />
        </div>
        <div className="h-5 w-px bg-border" />
        <div className="flex items-center gap-2">
          <span className="text-sm text-text-secondary">Environment</span>
          <Select value={currentEnvSlug} onValueChange={handleEnvSelect}>
            <SelectTrigger className="w-36" aria-label="Select environment">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              {environments.map((env) => (
                <SelectItem key={env.id} value={env.slug}>
                  {env.name}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>
        <div className="ml-auto flex items-center gap-2">
          <button
            type="button"
            onClick={() => setShowHelp(true)}
            className="flex h-7 w-7 items-center justify-center rounded-md text-text-secondary hover:bg-hover hover:text-text"
            aria-label="Help: how rules work"
          >
            <HelpCircle className="h-4 w-4" />
          </button>
          <div className="h-4 w-px bg-border" />
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

      {/* Rules */}
      <div className="space-y-3">
        {rules.map((rule, i) => (
          <RuleCard
            key={i}
            rule={rule}
            index={i}
            onChange={(r) => handleRulesChange(i, r)}
            onDelete={() => deleteRule(i)}
            errors={validationErrors}
          />
        ))}
        <Button type="button" variant="outline" size="sm" onClick={addRule}>
          <Plus className="h-3.5 w-3.5" />
          Add rule
        </Button>
      </div>

      {/* Env change confirm dialog */}
      {pendingEnv !== null && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50">
          <div className="w-full max-w-sm rounded-lg bg-popover p-6 shadow-lg">
            <h2 className="text-base font-medium">Unsaved changes</h2>
            <p className="mt-2 text-sm text-text-secondary">
              You have unsaved changes. Discard them and switch environment?
            </p>
            <div className="mt-4 flex justify-end gap-2">
              <Button type="button" variant="outline" size="sm" onClick={cancelEnvChange}>
                Cancel
              </Button>
              <Button type="button" variant="default" size="sm" onClick={confirmEnvChange}>
                Discard
              </Button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
