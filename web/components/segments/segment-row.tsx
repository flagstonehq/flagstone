import Link from "next/link";
import type { Segment } from "@/lib/types";
import { ArchiveSegmentButton } from "./archive-segment-button";
import { formatRelative } from "@/lib/utils";

interface SegmentRowProps {
  segment: Segment;
  projectSlug: string;
}

export function SegmentRow({ segment, projectSlug }: SegmentRowProps) {
  return (
    <tr className="hover:bg-hover transition-colors">
      <td className="text-text px-4 py-3 font-mono text-xs">
        <Link
          href={`/projects/${projectSlug}/segments/${segment.key}`}
          className="text-primary hover:underline"
        >
          {segment.key}
        </Link>
      </td>
      <td className="text-text px-4 py-3">{segment.name}</td>
      <td className="text-text-secondary px-4 py-3 text-xs max-w-xs truncate">
        {segment.description ?? "—"}
      </td>
      <td className="text-text-secondary px-4 py-3 text-xs whitespace-nowrap">
        {formatRelative(segment.updatedAt)}
      </td>
      <td className="px-4 py-3 text-right">
        <ArchiveSegmentButton projectSlug={projectSlug} segmentKey={segment.key} segmentName={segment.name} />
      </td>
    </tr>
  );
}
