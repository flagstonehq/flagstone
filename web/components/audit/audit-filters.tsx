"use client";

import { useRouter, useSearchParams, usePathname } from "next/navigation";
import { useTransition } from "react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  Select, SelectTrigger, SelectValue, SelectContent, SelectItem,
} from "@/components/ui/select";
import { RotateCcw } from "lucide-react";

interface AuditFiltersProps {
  actorType?: string;
  action?: string;
  resourceType?: string;
  since?: string;
  until?: string;
}

const ACTIONS = [
  "project.created", "project.updated", "project.deleted",
  "flag.created", "flag.updated", "flag.archived", "flag.toggled",
  "segment.created", "segment.updated", "segment.archived",
  "environment.created", "environment.deleted",
  "api_key.created", "api_key.revoked",
];

const RESOURCE_TYPES = ["project", "flag", "segment", "environment", "api_key"];

export function AuditFilters({
  actorType, action, resourceType, since, until,
}: AuditFiltersProps) {
  const router = useRouter();
  const pathname = usePathname();
  const search = useSearchParams();
  const [pending, startTransition] = useTransition();

  function update(key: string, value: string) {
    const next = new URLSearchParams(search.toString());
    if (value) next.set(key, value);
    else next.delete(key);
    next.delete("offset");
    startTransition(() => {
      router.push(`${pathname}?${next.toString()}`);
    });
  }

  function reset() {
    startTransition(() => router.push(pathname));
  }

  const hasFilters = !!(actorType || action || resourceType || since || until);

  return (
    <div className="grid grid-cols-1 gap-3 rounded-lg border border-border bg-surface p-4 md:grid-cols-5">
      <div className="space-y-1">
        <Label htmlFor="actor-type">Actor</Label>
        <Select
          value={actorType ?? ""}
          onValueChange={(v: string | null) => update("actor_type", v ?? "")}
        >
          <SelectTrigger id="actor-type" aria-label="Actor type">
            <SelectValue placeholder="Any" />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="">Any</SelectItem>
            <SelectItem value="user">User</SelectItem>
            <SelectItem value="api_key">API key</SelectItem>
            <SelectItem value="system">System</SelectItem>
          </SelectContent>
        </Select>
      </div>

      <div className="space-y-1">
        <Label htmlFor="action">Action</Label>
        <Select
          value={action ?? ""}
          onValueChange={(v: string | null) => update("action", v ?? "")}
        >
          <SelectTrigger id="action" aria-label="Action">
            <SelectValue placeholder="Any" />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="">Any</SelectItem>
            {ACTIONS.map((a) => (
              <SelectItem key={a} value={a}>
                {a}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      </div>

      <div className="space-y-1">
        <Label htmlFor="resource-type">Resource</Label>
        <Select
          value={resourceType ?? ""}
          onValueChange={(v: string | null) => update("resource_type", v ?? "")}
        >
          <SelectTrigger id="resource-type" aria-label="Resource type">
            <SelectValue placeholder="Any" />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="">Any</SelectItem>
            {RESOURCE_TYPES.map((r) => (
              <SelectItem key={r} value={r}>
                {r}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      </div>

      <div className="space-y-1">
        <Label htmlFor="since">From</Label>
        <Input
          id="since"
          type="date"
          value={since ?? ""}
          onChange={(e) => update("since", e.target.value)}
        />
      </div>

      <div className="space-y-1">
        <Label htmlFor="until">To</Label>
        <Input
          id="until"
          type="date"
          value={until ?? ""}
          onChange={(e) => update("until", e.target.value)}
        />
      </div>

      {hasFilters && (
        <div className="md:col-span-5 flex justify-end">
          <Button
            type="button"
            variant="ghost"
            size="sm"
            onClick={reset}
            disabled={pending}
          >
            <RotateCcw className="h-3.5 w-3.5" />
            Reset filters
          </Button>
        </div>
      )}
    </div>
  );
}
