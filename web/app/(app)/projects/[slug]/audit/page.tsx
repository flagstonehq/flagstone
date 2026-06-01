import { cookies } from "next/headers";
import { redirect } from "next/navigation";
import Link from "next/link";
import { Suspense } from "react";
import { serverFetch } from "@/lib/api";
import type { AuditLogParams, AuditLogPage } from "@/lib/api";
import { AuditTable } from "@/components/audit/audit-table";
import { AuditFilters } from "@/components/audit/audit-filters";
import { Topbar } from "@/components/layout/topbar";

interface AuditPageProps {
  params: Promise<{ slug: string }>;
  searchParams: Promise<Record<string, string | string[] | undefined>>;
}

const PAGE_SIZE = 20;

function pickString(v: string | string[] | undefined): string | undefined {
  return Array.isArray(v) ? v[0] : v;
}

function dateToRfc3339(dateStr: string | undefined): string | undefined {
  if (!dateStr) return undefined;
  if (/^\d{4}-\d{2}-\d{2}$/.test(dateStr)) {
    return `${dateStr}T00:00:00Z`;
  }
  return dateStr;
}

export default async function AuditPage({ params, searchParams }: AuditPageProps) {
  const { slug } = await params;
  const sp = await searchParams;

  const cookieStore = await cookies();
  const token = cookieStore.get("access_token")?.value;
  if (!token) redirect("/login");

  const reqParams = {
    actor_type: pickString(sp.actor_type) as AuditLogParams["actor_type"],
    action: pickString(sp.action),
    resource_type: pickString(sp.resource_type),
    since: dateToRfc3339(pickString(sp.since)),
    until: dateToRfc3339(pickString(sp.until)),
    limit: PAGE_SIZE,
    offset: pickString(sp.offset) ? Number(pickString(sp.offset)) : 0,
  } satisfies AuditLogParams;

  const search = new URLSearchParams();
  for (const [key, value] of Object.entries(reqParams)) {
    if (value !== undefined && value !== "") {
      search.set(key, String(value));
    }
  }
  const qs = search.toString();
  const { entries, total, limit, offset } = await serverFetch<AuditLogPage>(
    `/api/v1/audit${qs ? "?" + qs : ""}`,
    token,
  );

  const currentOffset = offset ?? 0;
  const nextOffset = currentOffset + limit;
  const hasMore = nextOffset < total;

  return (
    <div className="flex h-full flex-col">
      <Topbar title="Audit Log" />
      <main className="flex-1 overflow-y-auto space-y-4 p-6">
        <Suspense fallback={<div className="h-24 animate-pulse rounded-lg bg-surface" />}>
          <AuditFilters
            actorType={pickString(sp.actor_type)}
            action={pickString(sp.action)}
            resourceType={pickString(sp.resource_type)}
            since={pickString(sp.since)}
            until={pickString(sp.until)}
          />
        </Suspense>
        <AuditTable entries={entries} />
        <div className="flex items-center justify-between text-sm text-text-secondary">
          <span>
            Showing {entries.length === 0 ? 0 : currentOffset + 1}
            {"\u2013"}
            {currentOffset + entries.length} of {total}
          </span>
          {hasMore && (
            <Link
              href={`/projects/${slug}/audit?offset=${nextOffset}${pickString(sp.actor_type) ? `&actor_type=${pickString(sp.actor_type)}` : ""}${pickString(sp.action) ? `&action=${pickString(sp.action)}` : ""}${pickString(sp.resource_type) ? `&resource_type=${pickString(sp.resource_type)}` : ""}${pickString(sp.since) ? `&since=${pickString(sp.since)}` : ""}${pickString(sp.until) ? `&until=${pickString(sp.until)}` : ""}`}
              className="text-primary hover:underline"
            >
              Load more &rarr;
            </Link>
          )}
        </div>
      </main>
    </div>
  );
}
