import { AuditRow } from "./audit-row";
import type { AuditEntry } from "@/lib/types";

interface AuditTableProps {
  entries: AuditEntry[];
}

export function AuditTable({ entries }: AuditTableProps) {
  if (entries.length === 0) {
    return (
      <div className="rounded-lg border border-border bg-surface p-8 text-center text-sm text-text-secondary">
        No audit entries match the current filters.
      </div>
    );
  }
  return (
    <div className="overflow-hidden rounded-lg border border-border bg-surface">
      <table className="w-full text-sm">
        <thead className="border-b border-border bg-surface-hover text-left text-xs uppercase tracking-wide text-text-secondary">
          <tr>
            <th className="px-4 py-2 font-medium">Action</th>
            <th className="px-4 py-2 font-medium">Actor</th>
            <th className="px-4 py-2 font-medium">Resource</th>
            <th className="px-4 py-2 font-medium">When</th>
          </tr>
        </thead>
        <tbody>
          {entries.map((e) => (
            <AuditRow key={e.id} entry={e} />
          ))}
        </tbody>
      </table>
    </div>
  );
}
