import { cookies } from "next/headers";
import { redirect, notFound } from "next/navigation";
import { serverFetch } from "@/lib/api";
import type { Segment } from "@/lib/types";
import { SegmentRuleEditor } from "@/components/segments/segment-rule-editor";
import { Topbar } from "@/components/layout/topbar";

interface SegmentPageProps {
  params: Promise<{ slug: string; key: string }>;
}

export default async function SegmentPage({ params }: SegmentPageProps) {
  const { slug, key } = await params;

  const cookieStore = await cookies();
  const token = cookieStore.get("access_token")?.value;
  if (!token) redirect("/login");

  const segment = await serverFetch<Segment>(
    `/api/v1/projects/${slug}/segments/${key}`,
    token,
  ).catch(() => null);

  if (!segment || segment.archivedAt) notFound();

  return (
    <div className="flex h-full flex-col">
      <Topbar title={`Segment: ${segment.key}`} />
      <main className="flex-1 overflow-y-auto p-4 sm:p-6">
        <div className="mx-auto max-w-2xl space-y-6">
          <div className="rounded-lg border border-border bg-surface p-4 space-y-2">
            <div className="flex items-center gap-2">
              <span className="text-xs font-medium text-text-secondary uppercase tracking-wider">
                {segment.name}
              </span>
            </div>
            {segment.description && (
              <p className="text-sm text-text-secondary">{segment.description}</p>
            )}
            <div className="flex gap-4 text-xs text-text-secondary">
              <span>Key: <span className="font-mono text-text">{segment.key}</span></span>
            </div>
          </div>

          <div>
            <h2 className="mb-3 text-sm font-semibold text-text">Matching rules</h2>
            <SegmentRuleEditor
              projectSlug={slug}
              segmentKey={key}
              initialRules={segment.rules}
              segmentName={segment.name}
            />
          </div>
        </div>
      </main>
    </div>
  );
}
