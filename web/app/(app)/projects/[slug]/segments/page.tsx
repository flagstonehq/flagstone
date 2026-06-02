import { cookies } from "next/headers";
import { redirect } from "next/navigation";
import { serverFetch } from "@/lib/api";
import type { Segment } from "@/lib/types";
import { SegmentsTable } from "@/components/segments/segments-table";
import { CreateSegmentDialog } from "@/components/segments/create-segment-dialog";
import { Topbar } from "@/components/layout/topbar";

interface SegmentsPageProps {
  params: Promise<{ slug: string }>;
}

export default async function SegmentsPage({ params }: SegmentsPageProps) {
  const { slug } = await params;

  const cookieStore = await cookies();
  const token = cookieStore.get("access_token")?.value;
  if (!token) redirect("/login");

  const segments = await serverFetch<Segment[]>(
    `/api/v1/projects/${slug}/segments`,
    token,
  );

  const activeSegments = segments.filter((s) => !s.archivedAt);

  return (
    <div className="flex h-full flex-col">
      <Topbar
        title="Segments"
        action={<CreateSegmentDialog projectSlug={slug} />}
      />
      <main className="flex-1 overflow-y-auto p-4 sm:p-6">
        <SegmentsTable segments={activeSegments} projectSlug={slug} />
      </main>
    </div>
  );
}
