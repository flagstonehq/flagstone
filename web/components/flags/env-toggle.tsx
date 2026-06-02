"use client";
import { useOptimistic, useTransition, useState } from "react";
import { useRouter } from "next/navigation";
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
  const router = useRouter();
  const [isPending, startTransition] = useTransition();
  const [optimisticEnabled, setOptimisticEnabled] = useOptimistic(defaultEnabled);
  const [error, setError] = useState(false);

  function handleToggle(next: boolean) {
    setError(false);
    startTransition(async () => {
      setOptimisticEnabled(next);
      try {
        await toggleFlagEnv(projectSlug, flagKey, envSlug, next);
        router.refresh();
      } catch {
        setError(true);
        // useOptimistic reverts to defaultEnabled automatically when transition ends
      }
    });
  }

  return (
    <div className="flex flex-col items-center gap-1">
      <Switch
        checked={optimisticEnabled}
        onCheckedChange={handleToggle}
        disabled={isPending}
        aria-label={`Toggle ${flagKey} in ${envSlug}`}
      />
      {error && <span className="text-danger text-xs">Failed</span>}
    </div>
  );
}
