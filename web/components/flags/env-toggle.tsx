"use client";
import { useState } from "react";
import { Switch } from "@/components/ui/switch";
import { toggleFlagEnv } from "@/lib/api";

interface EnvToggleProps {
  projectSlug: string;
  flagKey: string;
  envSlug: string;
  defaultEnabled?: boolean;
}

export function EnvToggle({
  projectSlug,
  flagKey,
  envSlug,
  defaultEnabled = false,
}: EnvToggleProps) {
  const [enabled, setEnabled] = useState(defaultEnabled);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState(false);

  async function handleToggle(next: boolean) {
    setLoading(true);
    setError(false);
    setEnabled(next); // optimistic — no router.refresh() so no revert
    try {
      await toggleFlagEnv(projectSlug, flagKey, envSlug, next);
    } catch {
      setEnabled(!next); // revert only on failure
      setError(true);
    } finally {
      setLoading(false);
    }
  }

  return (
    <div className="flex flex-col items-center gap-1">
      <Switch
        checked={enabled}
        onCheckedChange={handleToggle}
        disabled={loading}
        aria-label={`Toggle ${flagKey} in ${envSlug}`}
      />
      {error && <span className="text-danger text-xs">Failed</span>}
    </div>
  );
}
