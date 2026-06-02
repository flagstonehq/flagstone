import type { Segment } from "@/lib/types";
import { SegmentRow } from "./segment-row";
import { Users } from "lucide-react";

interface SegmentsTableProps {
  segments: Segment[];
  projectSlug: string;
}

export function SegmentsTable({ segments, projectSlug }: SegmentsTableProps) {
  if (segments.length === 0) {
    return (
      <div className="flex min-h-[400px] flex-col items-center justify-center gap-4 text-center">
        <div className="bg-primary-bg flex h-16 w-16 items-center justify-center rounded-full">
          <Users className="text-primary h-8 w-8" />
        </div>
        <div>
          <h2 className="text-text text-lg font-semibold">No segments yet</h2>
          <p className="text-text-secondary mt-1 text-sm">
            Create your first segment to define user groups for targeting rules.
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
            <th className="text-text-secondary px-4 py-3 text-left font-medium">Description</th>
            <th className="text-text-secondary px-4 py-3 text-left font-medium">Updated</th>
            <th className="px-4 py-3" />
          </tr>
        </thead>
        <tbody className="divide-border bg-surface divide-y">
          {segments.map((segment) => (
            <SegmentRow key={segment.id} segment={segment} projectSlug={projectSlug} />
          ))}
        </tbody>
      </table>
    </div>
  );
}
