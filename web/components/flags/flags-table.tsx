import type { Flag, Environment } from "@/lib/types";
import { FlagRow } from "./flag-row";
import { Flag as FlagIcon } from "lucide-react";

interface FlagsTableProps {
  flags: Flag[];
  environments: Environment[];
  projectSlug: string;
  enabledMap: Record<string, Record<string, boolean>>;
}

export function FlagsTable({ flags, environments, projectSlug, enabledMap }: FlagsTableProps) {
  if (flags.length === 0) {
    return (
      <div className="flex min-h-[400px] flex-col items-center justify-center gap-4 text-center">
        <div className="bg-primary-bg flex h-16 w-16 items-center justify-center rounded-full">
          <FlagIcon className="text-primary h-8 w-8" />
        </div>
        <div>
          <h2 className="text-text text-lg font-semibold">No flags yet</h2>
          <p className="text-text-secondary mt-1 text-sm">
            Create your first flag to start controlling feature rollouts.
          </p>
        </div>
      </div>
    );
  }
  return (
    <div className="overflow-x-auto rounded-xl border border-border">
      <table className="w-full text-sm">
        <thead>
          <tr className="border-border bg-surface border-b">
            <th className="text-text-secondary px-4 py-3 text-left font-medium">Key</th>
            <th className="text-text-secondary px-4 py-3 text-left font-medium">Name</th>
            <th className="text-text-secondary px-4 py-3 text-left font-medium">Type</th>
            {environments.map((env) => (
              <th key={env.id} className="text-text-secondary px-4 py-3 text-center font-medium">
                {env.name}
              </th>
            ))}
            <th className="text-text-secondary px-4 py-3 text-left font-medium">Updated</th>
            <th className="px-4 py-3" />
          </tr>
        </thead>
        <tbody className="divide-border bg-surface divide-y">
          {flags.map((flag) => (
            <FlagRow
              key={flag.id}
              flag={flag}
              environments={environments}
              projectSlug={projectSlug}
              enabledMap={enabledMap}
            />
          ))}
        </tbody>
      </table>
    </div>
  );
}
