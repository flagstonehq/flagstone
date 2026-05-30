import type { Flag, Environment } from "@/lib/types";
import { EnvToggle } from "./env-toggle";
import { ArchiveFlagButton } from "./archive-flag-button";
import { formatRelative } from "@/lib/utils";

const TYPE_COLORS: Record<string, string> = {
  boolean: "bg-green-100 text-green-800",
  string: "bg-blue-100 text-blue-800",
  number: "bg-orange-100 text-orange-800",
  json: "bg-purple-100 text-purple-800",
};

interface FlagRowProps {
  flag: Flag;
  environments: Environment[];
  projectSlug: string;
}

export function FlagRow({ flag, environments, projectSlug }: FlagRowProps) {
  return (
    <tr className="hover:bg-hover transition-colors">
      <td className="text-text px-4 py-3 font-mono text-xs">{flag.key}</td>
      <td className="text-text px-4 py-3">{flag.name}</td>
      <td className="px-4 py-3">
        <span
          className={`inline-flex items-center rounded px-2 py-0.5 text-xs font-medium ${TYPE_COLORS[flag.type] ?? ""}`}
        >
          {flag.type}
        </span>
      </td>
      {environments.map((env) => (
        <td key={env.id} className="px-4 py-3 text-center">
          <EnvToggle
            projectSlug={projectSlug}
            flagKey={flag.key}
            envSlug={env.slug}
          />
        </td>
      ))}
      <td className="text-text-secondary px-4 py-3 text-xs whitespace-nowrap">
        {formatRelative(flag.updatedAt)}
      </td>
      <td className="px-4 py-3 text-right">
        <ArchiveFlagButton projectSlug={projectSlug} flagKey={flag.key} flagName={flag.name} />
      </td>
    </tr>
  );
}
